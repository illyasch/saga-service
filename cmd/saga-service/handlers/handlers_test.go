package handlers_test

import (
	"log"
	"os"
	"testing"

	"github.com/ardanlabs/conf/v3"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/illyasch/saga-service/pkg/data/database"
	"github.com/illyasch/saga-service/pkg/sys/logger"
)

var (
	postgresDB *sqlx.DB
	stdLgr     *zap.SugaredLogger
)

func TestMain(m *testing.M) {
	var err error
	stdLgr, err = logger.New("saga")
	if err != nil {
		log.Fatal(err)
	}

	cfg := struct {
		conf.Version
		DB struct {
			User         string `conf:"default:postgres"`
			Password     string `conf:"default:nimda,mask"`
			Host         string `conf:"default:localhost"`
			Name         string `conf:"default:postgres"`
			MaxIdleConns int    `conf:"default:0"`
			MaxOpenConns int    `conf:"default:0"`
			DisableTLS   bool   `conf:"default:true"`
		}
	}{
		Version: conf.Version{
			Build: "test",
			Desc:  "Copyright Ilya Scheblanov",
		},
	}

	const prefix = "SHORTENER"
	_, err = conf.Parse(prefix, &cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(cfg)

	postgresDB, err = database.Open(database.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(m.Run())
}
