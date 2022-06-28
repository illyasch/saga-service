package app_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/illyasch/saga-service/pkg/sys/app"
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
		assert.Equal(t, app.ExitFail, got)
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
		assert.Equal(t, app.ExitOK, got)
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
		assert.Equal(t, app.ExitFail, got)
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
		assert.Equal(t, app.ExitOK, got)
		assert.Equal(t, tasks, len(cntCh))
	})
}

func initApp(tasks int) (chan struct{}, *app.App) {
	cntCh := make(chan struct{}, tasks)
	a := &app.App{}

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
