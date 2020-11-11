package workflow

import (
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	"go.temporal.io/api/common/v1"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/internalbindings"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
)

const (
	DestroyWorkflowCommand  = "DestroyWorkflow"
	StartWorkflowCommand    = "StartWorkflow"
	ExecuteActivityCommand  = "ExecuteActivity"
	NewTimerCommand         = "NewTimer"
	CompleteWorkflowCommand = "CompleteWorkflow"
	SideEffectCommand       = "SideEffect"
	InvokeSignalCommand     = "InvokeSignal"
	InvokeQueryCommand      = "InvokeQuery"
	StackTraceCommand       = "StackTrace"
	GetVersionCommand       = "GetVersion"

	// desert
	ExecuteChildWorkflowCommand = "ExecuteChildWorkflow"

	// cancels
	CancelTimerCommand    = "CancelTimer"
	CancelActivityCommand = "CancelActivity"
)

// GetBacktrace asks worker to offload workflow from memory.
type GetBacktrace struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
}

// DestroyWorkflow asks worker to offload workflow from memory.
type DestroyWorkflow struct {
	// RunID workflow run id.
	RunID string `json:"runId"`
}

// StartWorkflow sends worker command to start workflow.
type StartWorkflow struct {
	Info  *workflow.Info        `json:"info"`
	Input []jsoniter.RawMessage `json:"args"`
}

// FromEnvironment maps start command from environment.
func (start *StartWorkflow) FromEnvironment(env internalbindings.WorkflowEnvironment, input *common.Payloads) error {
	start.Info = env.WorkflowInfo()

	return rrt.FromPayloads(env.GetDataConverter(), input, &start.Input)
}

type InvokeQuery struct {
	RunID string                `json:"runId"`
	Name  string                `json:"name"`
	Args  []jsoniter.RawMessage `json:"args"`
}

type InvokeSignal struct {
	RunID string                `json:"runId"`
	Name  string                `json:"name"`
	Args  []jsoniter.RawMessage `json:"args"`
}

// ExecuteActivity command by workflow worker.
type ExecuteActivity struct {
	// Name defines activity name.
	Name string `json:"name"`

	// Args to pass to the activity.
	Args []jsoniter.RawMessage `json:"arguments"`

	// Info to run activity as.
	// todo: implement
	//Info workflow.ActivityOptions `json:"options,omitempty"`

	// ArgsPayload represents Args converted into Temporal payload format.
	ArgsPayload *commonpb.Payloads
}

// ActivityParams maps activity command to activity params.
func (cmd ExecuteActivity) ActivityParams(env bindings.WorkflowEnvironment) bindings.ExecuteActivityParams {
	return bindings.ExecuteActivityParams{
		// todo: implement mapping
		ExecuteActivityOptions: bindings.ExecuteActivityOptions{
			TaskQueueName:          env.WorkflowInfo().TaskQueueName,
			ScheduleToCloseTimeout: time.Minute * 60,
			ScheduleToStartTimeout: time.Minute * 60,
			HeartbeatTimeout:       time.Minute * 60, // WTF?
			StartToCloseTimeout:    time.Minute * 5,  // WTF?
			RetryPolicy: &commonpb.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
		ActivityType: bindings.ActivityType{Name: cmd.Name},
		Input:        cmd.ArgsPayload,
	}
}

// NewTimer starts new timer.
type NewTimer struct {
	// Milliseconds defines timer duration.
	Milliseconds int `json:"ms"`
}

// ToDuration converts timer command to time.Duration.
func (t NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(t.Milliseconds)
}

// CompleteWorkflow sent by worker to complete workflow.
type CompleteWorkflow struct {
	// Result defines workflow execution result.
	Result []jsoniter.RawMessage `json:"result"`

	// todo: need error!!

	// ResultPayload represents Result converted into Temporal payload format.
	ResultPayload *commonpb.Payloads
}

// maps worker parameters into internal command representation.
func parseCommand(dc converter.DataConverter, name string, params jsoniter.RawMessage) (interface{}, error) {
	switch name {
	case ExecuteActivityCommand:
		cmd := ExecuteActivity{}

		if err := jsoniter.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		cmd.ArgsPayload = &commonpb.Payloads{}
		if err := rrt.ToPayloads(dc, cmd.Args, cmd.ArgsPayload); err != nil {
			return nil, err
		}

		return cmd, nil

	case NewTimerCommand:
		cmd := NewTimer{}
		if err := jsoniter.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		return cmd, nil

	case CompleteWorkflowCommand:
		cmd := CompleteWorkflow{}
		if err := jsoniter.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		cmd.ResultPayload = &commonpb.Payloads{}
		if err := rrt.ToPayloads(dc, cmd.Result, cmd.ResultPayload); err != nil {
			return nil, err
		}

		return cmd, nil

	// todo: map other commands

	case GetVersionCommand:
		cmd := GetVersion{}
		if err := jsoniter.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		return cmd, nil

	case SideEffectCommand:
		cmd := SideEffect{}
		if err := jsoniter.Unmarshal(params, &cmd); err != nil {
			return nil, err
		}

		cmd.Payloads = &commonpb.Payloads{}
		if err := rrt.ToPayloads(dc, []jsoniter.RawMessage{cmd.Value}, cmd.Payloads); err != nil {
			return nil, err
		}

		return cmd, nil

	default:
		return nil, errors.E(errors.Op("parseCommand"), "undefined command type", errors.Str(name))
	}
}

// SideEffect to be recorded into the history.
type SideEffect struct {
	Value    jsoniter.RawMessage `json:"value"`
	Payloads *commonpb.Payloads
}

// NewTimer starts new timer.
type GetVersion struct {
	ChangeID     string `json:"changeID"`
	MinSupported int    `json:"minSupported"`
	MaxSupported int    `json:"maxSupported"`
}
