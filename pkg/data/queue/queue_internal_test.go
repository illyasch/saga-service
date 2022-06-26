package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_transformTemplates(t *testing.T) {
	notification := worker.SendNotification{
		To:      "john@doe.com",
		ReplyTo: "john@doe.com",
		Subject: "a message from johndoe",
		Text:    "",
		HTML:    "Hi there!",
	}

	queueNotification := SendNotification{
		From:    nil,
		To:      "john@doe.com",
		ReplyTo: "john@doe.com",
		Subject: "a message from johndoe",
		Text:    "",
		HTML:    "Hi there!",
	}

	tests := []struct {
		name     string
		template worker.CompiledTemplate
		want     CompiledTemplate
	}{
		{
			name: "correctly transforms a worker.CompiledTemplate with both notifications",
			template: worker.CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: worker.SendNotifications{
					Self:       &notification,
					Respondent: &notification,
				},
				Ctx: worker.Context{},
			},
			want: CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: SendNotifications{
					Self:       &queueNotification,
					Respondent: &queueNotification,
				},
			},
		},
		{
			name: "correctly transforms a worker.CompiledTemplate with only self notification",
			template: worker.CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: worker.SendNotifications{
					Self: &notification,
				},
				Ctx: worker.Context{},
			},
			want: CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: SendNotifications{
					Self: &queueNotification,
				},
			},
		},
		{
			name: "correctly transforms a worker.CompiledTemplate with only respondent notifications",
			template: worker.CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: worker.SendNotifications{
					Respondent: &notification,
				},
				Ctx: worker.Context{},
			},
			want: CompiledTemplate{
				Token:  "token",
				FormID: "formid",
				Notifications: SendNotifications{
					Respondent: &queueNotification,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformTemplates(tt.template)
			assert.Equal(t, got, tt.want)
		})
	}
}
