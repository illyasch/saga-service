package service_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/ardanlabs/service/foundation/worker"

	"github.com/golang/mock/gomock"
	"github.com/illyasch/saga-service/pkg/sys/service"
	"github.com/openzipkin/zipkin-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_Run(t *testing.T) {
	t.Run("A task with an error", func(t *testing.T) {
		const tasks = 5
		cntCh, a := initApp(tasks)
		a.Add(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return errors.New("test error raised")
		})

		got := a.Run(context.Background())
		assert.Equal(t, service.ExitFail, got)
		assert.Equal(t, tasks, len(cntCh))
	})

	t.Run("External cancellation", func(t *testing.T) {
		const tasks = 6
		cntCh, a := initApp(tasks)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		got := a.Run(ctx)
		assert.Equal(t, service.ExitOK, got)
		assert.Equal(t, tasks, len(cntCh))
	})

	t.Run("A task with panic", func(t *testing.T) {
		const tasks = 4
		cntCh, a := initApp(tasks)

		a.Add(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			panic("test panic raised")
		})

		got := a.Run(context.Background())
		assert.Equal(t, service.ExitFail, got)
		assert.Equal(t, tasks, len(cntCh))
	})

	t.Run("A task with SIGINT", func(t *testing.T) {
		const tasks = 8
		cntCh, a := initApp(tasks)

		a.Add(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		})

		got := a.Run(context.Background())
		assert.Equal(t, service.ExitOK, got)
		assert.Equal(t, tasks, len(cntCh))
	})
}

func initApp(tasks int) (chan struct{}, *service.App) {
	cntCh := make(chan struct{}, tasks)
	a := &service.App{}

	for i := 0; i < tasks; i++ {
		a.Add(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				cntCh <- struct{}{}
				return nil
			}
		})
	}
	return cntCh, a
}

func TestApp_Integration(t *testing.T) {
	app, err := initialize(t)
	require.NoError(t, err)

	exit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		pid := syscall.Getpid()
		_ = syscall.Kill(pid, syscall.SIGINT)

		select {
		case <-exit:
			t.Log("Application has been stopped gracefully with SIGINT.")
			done <- struct{}{}
			return
		case <-time.After(time.Second):
			panic("Application cannot stop gracefully with SIGINT!")
		}
	}()

	got := app.Run(context.Background())
	assert.Equal(t, service.ExitOK, got)
	exit <- struct{}{}
	<-done
}

func initialize(t *testing.T) (*service.App, error) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &service.App{}

	logger := mocks.NewContextLogger(ctrl)
	metrics := mocks.NewCollector(ctrl)
	mLogger := mocks.NewMetricLogger(ctrl)
	tracer, err := zipkin.NewTracer(nil, zipkin.WithLocalEndpoint(nil))
	if !assert.NoError(t, err, "failed to create test tracer") {
		return nil, err
	}

	jobStore := mocks.NewJobStore(ctrl)
	httpClient := mocks.NewClient(ctrl)
	ws := mocks.NewWorkspaceLister(ctrl)

	logger.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	logger.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().Timing(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mLogger.EXPECT().LogAndMeasure(gomock.Any(), gomock.Any()).AnyTimes()
	jobStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes()
	jobStore.EXPECT().GetJobs(gomock.Any()).AnyTimes()
	jobStore.EXPECT().GetJobsByStatus(gomock.Any(), gomock.Any()).AnyTimes()

	cfg := config.Config{}
	version := &healthcheck.StatusVersion{
		CommitSha: "1.0",
		BuildDate: "01-02-2020",
		Release:   "1.0",
	}
	formsAPI := forms.New(httpClient, "http://forms")
	responsesAPI := responses.New(httpClient, "http://responses")

	httpServer := server.New(logger, metrics, tracer, version, ws, tfjwt.NoValidate, jobStore, &cfg, mLogger)

	app.Add(func(lctx context.Context) error {
		select {
		case <-lctx.Done():
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			return httpServer.Shutdown(ctxWithTimeout)
		}
	})

	app.Add(func(lctx context.Context) error {
		return service.New(lctx, httpServer, cfg, logger).Start()
	})

	app.Add(func(lctx context.Context) error {
		worker.New(lctx, logger, metrics, jobStore, formsAPI, responsesAPI).Loop()
		return nil
	})

	return app, nil
}
