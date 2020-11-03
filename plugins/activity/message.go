package activity

import (
	jsoniter "github.com/json-iterator/go"

	"go.temporal.io/sdk/activity"
)

const (
	InvokeActivityCommand = "InvokeActivity"
)

// InvokeActivity requests worker to execute activity.
type InvokeActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Info contains execution context.
	Info activity.Info `json:"info"`

	// Args contain activity call arguments.
	Args []jsoniter.RawMessage `json:"args"`
}
