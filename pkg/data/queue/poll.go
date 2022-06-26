package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type Processor interface {
	ProcessMessage(context.Context, any) error
}

type Poll[M Message] struct {
	incoming *Receiver
	target   Processor
	logger   *zap.SugaredLogger
}

func NewPoll[M Message](incoming *Receiver, target Processor, logger *zap.SugaredLogger) (Poll[M], error) {
	return Poll[M]{incoming: incoming, target: target, logger: logger}, nil
}

// Start polling the incoming queue and calls target's ProcessMessage method for each received message.
// The message is removed from the queue if the processing was successful.
// TODO Add concurrency calling ProcessMessage with limiting goroutines number.
func (p Poll[M]) Start(ctx context.Context) error {
	for !isDone(ctx) {
		msgs, err := p.incoming.ReceiveMessages(ctx)
		if err != nil {
			p.logger.Errorw("poll", "ERROR", fmt.Errorf("listener: receiving messages: %w", err))
			continue
		}

		for _, m := range msgs {
			if isDone(ctx) {
				return nil
			}
			var msg M

			err := json.Unmarshal([]byte(*m.Body), &msg)
			if err != nil {
				p.logger.Errorw("poll", "ERROR", fmt.Errorf("listener: could not unmarshal response event"))
				continue
			}
			p.logger.Infow("poll", "INFO", "listener", "message", m.String())

			if err = p.target.ProcessMessage(ctx, msg); err != nil {
				p.logger.Errorw("poll", "ERROR", fmt.Errorf("listener: processing response: %w", err))
				p.logger.Infow("poll", "INFO", "listener: poll has stopped", "message", m.String())
				continue
			}

			if err = p.incoming.DeleteMessage(ctx, m.ReceiptHandle); err != nil {
				p.logger.Errorw("poll", "ERROR", fmt.Errorf("listener: %w", err))
			}
		}
	}

	return nil
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	return false
}
