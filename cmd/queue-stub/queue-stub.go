package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/zap"

	"github.com/illyasch/saga-service/pkg/business/saga"
	"github.com/illyasch/saga-service/pkg/data/queue"
	"github.com/illyasch/saga-service/pkg/sys/logger"
)

const configPrefix = "STUB"

type config struct {
	conf.Version
	ServiceName string `conf:"default:queue-stub"`
	Queue       struct {
		AWSEndpoint     string        `conf:"default:http://localhost:4566"`
		AWSRegion       string        `conf:"default:us-west-1a"`
		CommandsQueue   string        `conf:"default:commands1"`
		ResponsesQueue  string        `conf:"default:responses"`
		MaxMessages     int64         `conf:"default:10"`
		WaitTime        int64         `conf:"default:20"`
		ShutdownTimeout time.Duration `conf:"default:20s"`
	}
}

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

func main() {
	log, err := logger.New("queue-stub")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		_ = log.Sync()
		os.Exit(1)
	}
}

func run(log *zap.SugaredLogger) error {
	cfg, err := parseConfig(configPrefix, log)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	log.Infow("starting stub service", "version", build)
	defer log.Infow("shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	// Create connectivity to the queue.
	awsConfig := aws.NewConfig().WithRegion(cfg.Queue.AWSRegion)
	if cfg.Queue.AWSEndpoint != "" {
		awsConfig.WithEndpoint(cfg.Queue.AWSEndpoint)
	}
	// Generic AWS service container with credentials.
	awsSQS := sqs.New(session.Must(session.NewSession()), awsConfig)
	// Create queue sender.
	sender, err := queue.NewSender(awsSQS, cfg.Queue.ResponsesQueue)
	if err != nil {
		return fmt.Errorf("creating receiver(%s): %w", saga.QueueName, err)
	}
	// Create queue receiver.
	r, err := queue.NewReceiver(awsSQS, cfg.Queue.CommandsQueue, cfg.Queue.MaxMessages, cfg.Queue.WaitTime)
	if err != nil {
		return fmt.Errorf("creating receiver(%s): %w", saga.QueueName, err)
	}
	// Create queue poller.
	poller, err := queue.NewPoll[queue.Command](
		&r,
		queueStub{service: cfg.ServiceName, sender: sender, log: log},
		log,
	)
	if err != nil {
		return fmt.Errorf("new poller: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the poller. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	pollerErrors := make(chan error, 1)

	// Start the service listening for incoming queue messages.
	go func() {
		log.Infow("startup", "status", "queue listener started")
		pollerErrors <- poller.Start(ctx)
	}()

	// Shutdown
	// Blocking main and waiting for shutdown.
	select {
	case err := <-pollerErrors:
		return fmt.Errorf("listener error: %w", err)

	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		// Asking poller to shut down.
		cancel()
		time.Sleep(cfg.Queue.ShutdownTimeout)
	}

	return nil
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

type queueStub struct {
	service string
	sender  *queue.Sender
	log     *zap.SugaredLogger
}

func (q queueStub) ProcessMessage(_ context.Context, inp any) error {
	command, ok := inp.(queue.Command)
	if !ok {
		return fmt.Errorf("malformed command")
	}

	q.log.Infow("processing", "command", command.Name, "saga", command.SagaID)
	err := q.sender.Send(queue.Response{
		SagaID:  command.SagaID,
		Service: q.service,
		Status:  saga.StatusWorkDone,
	})
	if err != nil {
		return fmt.Errorf("sending response: %w", err)
	}

	return nil
}
