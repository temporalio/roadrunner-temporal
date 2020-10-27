package workflow

import (
	"go.temporal.io/sdk/workflow"
	"time"
)

// ExecuteActivity command by workflow worker.
type ExecuteActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Options to run activity as.
	Options workflow.ActivityOptions `json:"options"`

	// todo: chnage to
	Args []interface{} `json:"arguments"`
}

type CompleteWorkflow struct {
	Result []interface{} `json:"result"`
}

type NewTimer struct {
	Milliseconds int `json:"ms"`
}

func (t NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(t.Milliseconds)
}
