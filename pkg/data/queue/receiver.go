package queue

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// Receiver struct holds functionality to receive messages off of an SQS queue.
type Receiver struct {
	sqs   *sqs.SQS
	input *sqs.ReceiveMessageInput
}

// NewReceiver function returns a configured Receiver with sqs connection, queue name, config, and logger used.
func NewReceiver(svc *sqs.SQS, name string, maxMessages, waitTime int64) (Receiver, error) {
	queueURL, err := getQueueURL(svc, name)
	if err != nil {
		return Receiver{}, fmt.Errorf("sqs input: %w", err)
	}

	return Receiver{
		sqs: svc,
		input: &sqs.ReceiveMessageInput{
			MaxNumberOfMessages: aws.Int64(maxMessages),
			QueueUrl:            &queueURL,
			WaitTimeSeconds:     aws.Int64(waitTime),
		},
	}, nil
}

// ReceiveMessages returns slice of messages from the queue.
func (r Receiver) ReceiveMessages(ctx context.Context) ([]*sqs.Message, error) {
	resp, err := r.sqs.ReceiveMessageWithContext(ctx, r.input)
	if err != nil {
		return nil, fmt.Errorf("sqs input: receive messages: %w", err)
	}

	return resp.Messages, nil
}

// DeleteMessage deletes a message with a receipt handle.
func (r Receiver) DeleteMessage(ctx context.Context, receiptHandle *string) error {
	_, err := r.sqs.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      r.input.QueueUrl,
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}

	return nil
}
