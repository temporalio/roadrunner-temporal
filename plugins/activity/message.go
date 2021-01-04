package activity

import (
	"go.temporal.io/api/common/v1"

	"go.temporal.io/sdk/activity"
)

const (
	InvokeActivityCommand = "InvokeActivity"
)

type (
	InvokeActivity struct {
		// Name defines activity name.
		Name string `json:"name"`

		// Info contains execution context.
		Info activity.Info `json:"info"`

		// Args contain activity call arguments.
		Args []*common.Payload `json:"args"`
	}
)
