package saga

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/illyasch/saga-service/pkg/data/queue"
)

// Workflow represents a saga workflow with a list of services in order of their sequential execution.
type Workflow struct {
	Services []Service
}

var SampleWorkflow = []Service{
	{
		"service1", "commands1", nil,
	},
	{
		"service2", "commands2", nil,
	},
	{
		"service3", "commands3", nil,
	},
}

// NewWorkflowWithSQS initializes a new workflow with a SQS queues for each service.
func NewWorkflowWithSQS(services []Service, awsSQS *sqs.SQS) (Workflow, error) {
	w := Workflow{Services: services}

	var err error
	for i := range services {
		services[i].Sender, err = queue.NewSender(awsSQS, services[i].Topic)
		if err != nil {
			return w, fmt.Errorf("new sender(%s): %w", services[i].Topic, err)
		}
	}

	return w, nil
}
