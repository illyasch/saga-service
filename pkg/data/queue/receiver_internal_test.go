package queue

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewReceiver(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQS test in short mode")
	}

	svc := th.InitTestSQSClient(t)

	cfg, err := config.New(th.InitConfigProvider())
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	t.Run("if queue exists, it returns new receiver", func(t *testing.T) {
		qname := fmt.Sprintf("test-email-generator-incoming-%v", th.Randstr(6))
		qURL := th.CreateSQSQueue(t, svc, qname)
		defer th.DeleteSQSQueue(t, svc, qURL)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logger := mocks.NewMockLogger(ctrl)
		got, _ := NewReceiver(svc, qname, cfg, logger)
		assert.IsType(t, Receiver{}, got)
		assert.Equal(t, svc, got.sqs)
		assert.Equal(t, &sqs.ReceiveMessageInput{
			MaxNumberOfMessages: aws.Int64(int64(cfg.SQSMaxMessages())),
			QueueUrl:            &qURL,
			WaitTimeSeconds:     aws.Int64(int64(cfg.SQSWaitTime())),
		}, got.input)
	})

	t.Run("if queue does not exist, it returns error", func(t *testing.T) {
		qname := "non-existing-queue"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logger := mocks.NewMockLogger(ctrl)
		got, err := NewReceiver(svc, qname, cfg, logger)
		assert.Equal(t, Receiver{}, got)
		assert.Regexp(t, regexp.MustCompile("sqs input: can't get url of the queue"), err.Error())
	})
}

func TestReceiveMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQS test in short mode")
	}

	svc := th.InitTestSQSClient(t)

	os.Setenv("EMAIL_GENERATOR_MAX_SQS_MESSAGES", "1")
	os.Setenv("EMAIL_GENERATOR_SQS_WAIT_TIME", "1")
	config, err := config.New(th.InitConfigProvider())
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	t.Run("if received messages successfully", func(t *testing.T) {
		t.Run("if unmarshal message successfully, it returns a slice of submissions", func(t *testing.T) {
			qname := fmt.Sprintf("test-email-generator-incoming-%v", th.Randstr(6))
			qURL := th.CreateSQSQueue(t, svc, qname)
			defer th.DeleteSQSQueue(t, svc, qURL)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := mocks.NewMockLogger(ctrl)

			logger.EXPECT().Log(gomock.Any()).AnyTimes()

			receiver, err := NewReceiver(svc, qname, config, logger)
			if err != nil {
				assert.FailNow(t, fmt.Sprintf("create Receiver: %+v", err))
			}

			msg, err := ioutil.ReadFile("../../cmd/email-generator/testdata/submission_completed_event.json")
			if err != nil {
				assert.FailNow(t, "read file")
			}

			th.PutInSQS(t, svc, qname, string(msg))

			submissions, err := receiver.ReceiveMessages()
			if err != nil {
				assert.FailNow(t, "expected nil, got "+err.Error())
			}

			if len(submissions) == 0 {
				assert.FailNowf(t, "", "expected submissions to be loaded, got 0 submissions")
			}

			for _, sub := range submissions {
				assert.Equal(t, "eSaAEOAo", sub.FormID)
			}
		})

		t.Run("if unable to unmarshal message, it does not load submission", func(t *testing.T) {
			qname := fmt.Sprintf("test-email-generator-incoming-%v", th.Randstr(6))
			qURL := th.CreateSQSQueue(t, svc, qname)
			defer th.DeleteSQSQueue(t, svc, qURL)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := mocks.NewMockLogger(ctrl)
			logger.EXPECT().Log(gomock.Any()).AnyTimes()

			receiver, err := NewReceiver(svc, qname, config, logger)
			if err != nil {
				assert.FailNow(t, fmt.Sprintf("create Receiver: %+v", err))
			}

			th.PutInSQS(t, svc, qname, `{"foo":"bar"}`)

			submissions, _ := receiver.ReceiveMessages()

			if len(submissions) > 0 {
				assert.FailNowf(t, "", "expected submissions to be empty, got %d submissions", len(submissions))
			}
		})
	})
}

func TestReceiver_DeleteMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQS test in short mode")
	}

	svc := th.InitTestSQSClient(t)
	cfg, err := config.New(th.InitConfigProvider())
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mocks.NewMockLogger(ctrl)

	qname := fmt.Sprintf("test-email-generator-incoming-%v", th.Randstr(6))
	qURL := th.CreateSQSQueue(t, svc, qname)
	receiver, _ := NewReceiver(svc, qname, cfg, logger)

	t.Run("if receipt handle is missing in context, it returns error", func(t *testing.T) {
		err := receiver.DeleteMessage(worker.Context{})
		assert.Regexp(t, "missing receipt handle in context", err.Error())
	})

	t.Run("if queue URL is missing in context, it returns error", func(t *testing.T) {
		err := receiver.DeleteMessage(worker.Context{
			receiptHandleKey: "foo",
		})
		assert.Regexp(t, "missing queue URL in context", err.Error())
	})

	t.Run("it deletes the message from the queue", func(t *testing.T) {
		inMsg, err := ioutil.ReadFile("../../cmd/email-generator/testdata/submission_completed_event.json")
		if err != nil {
			assert.FailNow(t, "read file")
		}
		_ = th.PutInSQS(t, svc, qname, string(inMsg))
		msg, ok := th.ReadLastMsgFromSQS(t, svc, qname)
		if !ok {
			assert.FailNow(t, fmt.Sprintf("read message"))
		}

		err = receiver.DeleteMessage(worker.Context{
			receiptHandleKey: *msg.ReceiptHandle,
			queueURLKey:      qURL,
		})
		if err != nil {
			assert.FailNow(t, err.Error())
		}

		submissions, err := receiver.ReceiveMessages()
		if len(submissions) > 0 {
			assert.FailNow(t, fmt.Sprintf("expect no messages in the queue"))
		}
	})
}

func TestReceivedAt(t *testing.T) {
	t.Run("if received at is present in context, it returns time from context and boolean", func(t *testing.T) {
		tm := time.Time{}.UTC()

		receivedAt, ok := ReceivedAt(worker.Context{
			receivedAtKey: tm,
		})

		assert.Equal(t, receivedAt, tm)
		assert.Equal(t, ok, true)
	})

	t.Run("if received at is not present in context, it returns nil time and fals boolean", func(t *testing.T) {
		receivedAt, ok := ReceivedAt(worker.Context{})

		assert.Equal(t, receivedAt, time.Time{})
		assert.Equal(t, ok, false)
	})
}
