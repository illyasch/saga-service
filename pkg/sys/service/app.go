package service

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	// ExitOK returned by Run() when a graceful shutdown achieved without an error.
	ExitOK = 0
	// ExitFail returned by Run() when an error happened.
	ExitFail = 1
)

// Runner can be added to the App as a task for concurrent running.
type Runner func(context.Context) error

// App keeps and concurrently run tasks. Intended for use in the main() function.
type App struct {
	wg    sync.WaitGroup
	tasks []Runner
}

// Add a new task to the App instance.
func (a *App) Add(r Runner) {
	a.tasks = append(a.tasks, r)
}

// Run puts together the service and starts it. It also handles a graceful shutdown.
func (a *App) Run(ctx context.Context) int {
	serviceErr := make(chan error)
	ctx, cancel := context.WithCancel(ctx)

	for _, t := range a.tasks {
		a.wg.Add(1)
		go func(task Runner) {
			defer a.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					serviceErr <- fmt.Errorf("panic: %v", r)
				}
			}()

			if err := task(ctx); err != nil {
				serviceErr <- err
			}
		}(t)
	}

	exitCode := ExitOK
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	monitorExit := make(chan struct{})
	go func() {
		for {
			select {
			case sErr := <-serviceErr:
				if sErr != nil {
					stdlog.Printf("service error: %s\n", sErr)
					exitCode = ExitFail
				}
			case osSignal := <-stop:
				stdlog.Printf("os signal received: %s. Stopping the server...\n", osSignal)
			case <-monitorExit:
				return
			}
			cancel()
		}
	}()

	a.wg.Wait()
	monitorExit <- struct{}{}

	return exitCode
}
