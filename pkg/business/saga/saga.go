// Package saga implements saga orchestration logic.
package saga

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/illyasch/saga-service/pkg/data/queue"
)

const (
	CommandStart    = "start"
	QueueName       = "responses"
	StatusStarted   = "started"
	StatusError     = "error"
	StatusWorkDone  = "done"
	StatusCompleted = "completed"
)

// Storer interface abstracts data access operations for persisting a saga.
type Storer interface {
	InsertSaga(context.Context, uuid.UUID, string, string) error
	UpdateStatus(context.Context, uuid.UUID, string) error
	UpdateService(context.Context, uuid.UUID, string, string) error
}

// Sender interface abstracts sending a message to queue.
type Sender interface {
	Send(any) error
}

// Service type has data for a service which is orchestrated by a saga.
type Service struct {
	Name   string
	Topic  string
	Sender Sender
}

// Saga contains the database for storing URLs.
type Saga struct {
	storage  Storer
	workflow Workflow
}

var (
	ErrEndOfWorkflow   = fmt.Errorf("end of workflow")
	ErrServiceNotFound = fmt.Errorf("service not found")
)

// New constructs a new Saga.
func New(workflow Workflow, storage Storer) Saga {
	return Saga{storage: storage, workflow: workflow}
}

// Start starts a new saga with a given ID. The method can be called by HTTP handler.
func (s Saga) Start(ctx context.Context, sagaID uuid.UUID) error {
	if len(s.workflow.Services) == 0 {
		return fmt.Errorf("empty workflow")
	}
	service := s.workflow.Services[0]

	if err := s.storage.InsertSaga(ctx, sagaID, service.Name, StatusStarted); err != nil {
		return err
	}

	err := service.send(queue.Command{
		SagaID: sagaID,
		Name:   CommandStart,
	})
	if err != nil {
		return fmt.Errorf("service send: %w", err)
	}

	return nil
}

// ProcessMessage receives a message with a response from a service and decides which service has to
// be called next according to the saga workflow.
func (s Saga) ProcessMessage(ctx context.Context, inp any) error {
	response, ok := inp.(queue.Response)
	if !ok {
		return fmt.Errorf("malformed response")
	}

	switch response.Status {
	case StatusWorkDone:
		return s.startNextService(ctx, response)

	case StatusError:
		if err := s.storage.UpdateStatus(ctx, response.SagaID, response.Status); err != nil {
			return fmt.Errorf("save status: %w", err)
		}

		return fmt.Errorf("response error %s", response.SagaID)

	default:
		return fmt.Errorf("unknown status %s saga %s", response.Status, response.SagaID)
	}
}

func (s Saga) startNextService(ctx context.Context, r queue.Response) error {
	next, err := s.findNextService(r.Service)
	if err != nil {
		if err == ErrEndOfWorkflow {
			if err := s.storage.UpdateStatus(ctx, r.SagaID, StatusCompleted); err != nil {
				return fmt.Errorf("update status: %w", err)
			}
			return nil
		}

		if err := s.storage.UpdateStatus(ctx, r.SagaID, StatusError); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return fmt.Errorf("find next: %w", err)
	}

	if err := s.storage.UpdateService(ctx, r.SagaID, r.Service, next.Name); err != nil {
		// if service was not updated we don't send a command to the service.
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("update service: %w", err)
	}

	err = next.send(queue.Command{
		SagaID: r.SagaID,
		Name:   CommandStart,
	})
	if err != nil {
		return fmt.Errorf("service send: %w", err)
	}

	return nil
}

func (s Saga) findNextService(service string) (Service, error) {
	var i int
	found := false
	for i = range s.workflow.Services {
		if s.workflow.Services[i].Name == service {
			found = true
			break
		}
	}

	if !found {
		return Service{}, ErrServiceNotFound
	}
	if i+1 >= len(s.workflow.Services) {
		return Service{}, ErrEndOfWorkflow
	}

	return s.workflow.Services[i+1], nil
}

func (s Service) send(msg queue.Command) error {
	return s.Sender.Send(msg)
}
