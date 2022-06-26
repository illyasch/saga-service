package queue

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
)

type Message interface {
	Command | Response
}

type Command struct {
	SagaID uuid.UUID `json:"saga_id"`
	Name   string    `json:"name"`
}

type Response struct {
	SagaID  uuid.UUID `json:"saga_id"`
	Service string    `json:"service"`
	Status  string    `json:"status"`
}

func getQueueURL(svc *sqs.SQS, queueName string) (string, error) {
	output, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return "", fmt.Errorf("can't get url of the queue (%s): %w", queueName, err)
	}

	return *output.QueueUrl, nil
}
