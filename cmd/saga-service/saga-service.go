// Grüezi!
// The saga-service orchestrates work of other services using saga pattern.
// This pattern is used for maintaining data consistency across services using
// a sequence of local transactions that are coordinated using asynchronous messaging.
// If you have any questions, email the author Ilya Scheblanov <ilya.scheblanov@gmail.com>.
package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/zap"

	"github.com/illyasch/saga-service/cmd/saga-service/handlers"
	"github.com/illyasch/saga-service/pkg/business/saga"
	"github.com/illyasch/saga-service/pkg/data/database"
	"github.com/illyasch/saga-service/pkg/data/queue"
	"github.com/illyasch/saga-service/pkg/sys/logger"
	"github.com/illyasch/saga-service/pkg/sys/service"
)

const configPrefix = "SAGA"

type config struct {
	conf.Version
	DB struct {
		User         string `conf:"default:postgres"`
		Password     string `conf:"default:postgres,mask"`
		Host         string `conf:"default:localhost"`
		Name         string `conf:"default:postgres"`
		MaxIdleConns int    `conf:"default:0"`
		MaxOpenConns int    `conf:"default:0"`
		DisableTLS   bool   `conf:"default:true"`
	}
	Web struct {
		ReadTimeout     time.Duration `conf:"default:5s"`
		WriteTimeout    time.Duration `conf:"default:10s"`
		IdleTimeout     time.Duration `conf:"default:120s"`
		ShutdownTimeout time.Duration `conf:"default:20s"`
		APIHost         string        `conf:"default:0.0.0.0:3000"`
	}
	Queue struct {
		AWSEndpoint string `conf:"default:http://localhost:4566"`
		AWSRegion   string `conf:"default:us-west-1a"`
		MaxMessages int64  `conf:"default:10"`
		WaitTime    int64  `conf:"default:20"`
	}
}

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	log, err := logger.New("saga")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()
	app, err := initialize(log)
	if err != nil {
		log.Errorw("startup", "ERROR", err)
	}

	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	os.Exit(app.Run(ctx))
}

func initialize(log *zap.SugaredLogger) (*service.App, error) {
	app := &service.App{}

	cfg, err := parseConfig(configPrefix, log)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			return app, nil
		}
		return app, fmt.Errorf("parsing config: %w", err)
	}

	out, err := conf.String(&cfg)
	if err != nil {
		return app, fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	expvar.NewString("build").Set(build)

	// Create connectivity to the database.
	log.Infow("startup", "status", "initializing database support", "host", cfg.DB.Host)

	db, err := database.Open(database.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		return app, fmt.Errorf("connecting to db: %w", err)
	}

	// Create connectivity to the queue.
	awsConfig := aws.NewConfig().WithRegion(cfg.Queue.AWSRegion)
	if cfg.Queue.AWSEndpoint != "" {
		awsConfig.WithEndpoint(cfg.Queue.AWSEndpoint)
	}
	// Generic AWS service container with credentials.
	awsSQS := sqs.New(session.Must(session.NewSession()), awsConfig)

	// Create saga business logic.
	workflow, err := saga.NewWorkflowWithSQS(saga.SampleWorkflow, awsSQS)
	if err != nil {
		return app, fmt.Errorf("creating saga workflow: %w", err)
	}
	sga := saga.New(workflow, database.NewStorage(db))
	// Create queue receiver.
	r, err := queue.NewReceiver(awsSQS, saga.QueueName, cfg.Queue.MaxMessages, cfg.Queue.WaitTime)
	if err != nil {
		return app, fmt.Errorf("creating receiver(%s): %w", saga.QueueName, err)
	}
	// Create saga response queue poller.
	poller, err := queue.NewPoll[queue.Response](&r, sga, log)
	if err != nil {
		return app, fmt.Errorf("creating poller: %w", err)
	}

	// Construct the mux for the API calls.
	apiMux := handlers.APIConfig{
		DB:   db,
		Log:  log,
		Saga: sga,
	}.Router()

	// Construct a server to service the requests against the mux.
	httpServer := http.Server{
		Addr:         cfg.Web.APIHost,
		Handler:      apiMux,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	// Adding tasks to App.
	// Spin up HTTP server.
	app.Add(func(ctx context.Context) error {
		err := httpServer.ListenAndServe()
		if err != nil {
			log.Errorw("shutdown", "ERROR", fmt.Errorf("error starting server: %w", err))
		}

		return httpServer.ListenAndServe()
	})
	// Spin up a queue poller.
	app.Add(func(ctx context.Context) error {
		return poller.Start(ctx)
	})
	// Defer HTTP server shutdown on the server exit.
	app.Add(func(ctx context.Context) error {
		<-ctx.Done()
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()
		log.Infow("shutdown", "status", "stopping HTTP server", "host", httpServer.Addr)

		return httpServer.Shutdown(ctxWithTimeout)
	})
	// Defer database connection closing on the server exit.
	app.Add(func(ctx context.Context) error {
		<-ctx.Done()
		log.Infow("shutdown", "status", "stopping database support", "host", cfg.DB.Host)

		return db.Close()
	})

	return app, nil
}

func parseConfig(prefix string, logger *zap.SugaredLogger) (config, error) {
	cfg := config{
		Version: conf.Version{
			Build: build,
			Desc:  "Copyright Ilya Scheblanov",
		},
	}

	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		fmt.Println(help)
		return cfg, err
	}

	out, err := conf.String(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("generating config for output: %w", err)
	}
	logger.Infow("startup", "config", out)

	return cfg, nil
}
