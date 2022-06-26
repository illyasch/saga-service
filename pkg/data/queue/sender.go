package queue

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// Sender struct holds functionality to send notifications to an SQS queue.
type Sender struct {
	sqs      *sqs.SQS
	queueURL string
}

// NewSender returns a new Sender instance with SQS connection, queue name, logger, and notification store configured.
func NewSender(q *sqs.SQS, queueName string) (*Sender, error) {
	queueURL, err := getQueueURL(q, queueName)
	if err != nil {
		return nil, fmt.Errorf("sqs input: %w", err)
	}

	return &Sender{sqs: q, queueURL: queueURL}, nil
}

// Send method sends a passed message to the queue.
func (s Sender) Send(msg interface{}) error {
	m := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(m).Encode(msg); err != nil {
		return fmt.Errorf("queue: sender: json marshal: %w", err)
	}

	_, err := s.sqs.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(m.String()),
		QueueUrl:    &s.queueURL,
	})
	if err != nil {
		return fmt.Errorf("queue: sender: send message: %w", err)
	}

	return nil
}
