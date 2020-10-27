package workflow

import (
	"encoding/json"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
	"time"
)

// StartWorkflow sends worker command to start workflow.
type StartWorkflow struct {
	Name      string            `json:"name"`
	Wid       string            `json:"wid"`
	Rid       string            `json:"rid"`
	TaskQueue string            `json:"taskQueue"`
	Input     []json.RawMessage `json:"args"`
}

// FromEnvironment maps start command from environment.
func (start *StartWorkflow) FromEnvironment(env internalbindings.WorkflowEnvironment, input *common.Payloads) error {
	info := env.WorkflowInfo()

	start.Name = info.WorkflowType.Name
	start.Wid = info.WorkflowExecution.ID
	start.Rid = info.WorkflowExecution.RunID
	start.TaskQueue = info.TaskQueueName

	return temporal.ParsePayload(env.GetDataConverter(), input, &start.Input)
}

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
