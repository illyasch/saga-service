//go:generate mockgen -destination=mock_storer_test.go -package=saga_test github.com/illyasch/saga-service/pkg/business/saga Storer
//go:generate mockgen -destination=mock_sender_test.go -package=saga_test github.com/illyasch/saga-service/pkg/business/saga Sender
package saga_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/illyasch/saga-service/pkg/business/saga"
	"github.com/illyasch/saga-service/pkg/data/queue"
)

func TestSaga_Start(t *testing.T) {
	t.Run("successful saga start", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			InsertSaga(gomock.Any(), sagaID, workflow.Services[0].Name, saga.StatusStarted).
			Return(nil)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(queue.Command{
			SagaID: sagaID,
			Name:   saga.CommandStart,
		}).Return(nil)
		workflow.Services[0].Sender = sender

		s := saga.New(workflow, storage)

		err := s.Start(context.Background(), sagaID)
		require.NoError(t, err)
	})

	t.Run("storing saga error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		dbErr := errors.New("DB error")
		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			InsertSaga(gomock.Any(), sagaID, workflow.Services[0].Name, saga.StatusStarted).
			Return(dbErr)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(gomock.Any()).Times(0)
		workflow.Services[0].Sender = sender

		s := saga.New(workflow, storage)

		err := s.Start(context.Background(), sagaID)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("sending message error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		qErr := errors.New("queue error")
		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			InsertSaga(gomock.Any(), sagaID, workflow.Services[0].Name, saga.StatusStarted).
			Return(nil)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(queue.Command{
			SagaID: sagaID,
			Name:   saga.CommandStart,
		}).Return(qErr)
		workflow.Services[0].Sender = sender

		s := saga.New(workflow, storage)

		err := s.Start(context.Background(), sagaID)
		assert.ErrorIs(t, err, qErr)
	})
}

func TestSaga_ProcessMessage(t *testing.T) {
	t.Run("successful saga step", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			UpdateService(gomock.Any(), sagaID, workflow.Services[0].Name, workflow.Services[1].Name).
			Return(nil)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(queue.Command{
			SagaID: sagaID,
			Name:   saga.CommandStart,
		}).Return(nil)
		workflow.Services[1].Sender = sender

		s := saga.New(workflow, storage)

		err := s.ProcessMessage(context.Background(), queue.Response{
			SagaID:  sagaID,
			Service: workflow.Services[0].Name,
			Status:  saga.StatusWorkDone,
		})
		require.NoError(t, err)
	})

	t.Run("successful saga end", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			UpdateStatus(gomock.Any(), sagaID, saga.StatusCompleted).
			Return(nil)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(gomock.Any()).Times(0)
		workflow.Services[2].Sender = sender

		s := saga.New(workflow, storage)

		err := s.ProcessMessage(context.Background(), queue.Response{
			SagaID:  sagaID,
			Service: workflow.Services[2].Name,
			Status:  saga.StatusWorkDone,
		})
		require.NoError(t, err)
	})

	t.Run("error of a saga service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sagaID := uuid.New()
		workflow := saga.Workflow{
			Services: saga.SampleWorkflow,
		}

		storage := NewMockStorer(ctrl)
		storage.EXPECT().
			UpdateStatus(gomock.Any(), sagaID, saga.StatusError).
			Return(nil)

		sender := NewMockSender(ctrl)
		sender.EXPECT().Send(gomock.Any()).Times(0)
		workflow.Services[0].Sender = sender

		s := saga.New(workflow, storage)

		err := s.ProcessMessage(context.Background(), queue.Response{
			SagaID:  sagaID,
			Service: workflow.Services[0].Name,
			Status:  saga.StatusError,
		})

		require.ErrorContains(t, err, fmt.Sprintf("response error %s", sagaID))
	})
}
