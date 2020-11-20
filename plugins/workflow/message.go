package workflow

import (
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/workflow"
)

const (
	// Commands send by the host process to the worker.
	StartWorkflowCommand   = "StartWorkflow"
	InvokeSignalCommand    = "InvokeSignal"
	InvokeQueryCommand     = "InvokeQuery"
	DestroyWorkflowCommand = "DestroyWorkflow"
	CancelWorkflowCommand  = "CancelWorkflow"
	GetStackTraceCommand   = "StackTrace"

	// Commands send by worker to host process.
	ExecuteActivityCommand      = "ExecuteActivity"
	ExecuteLocalActivityCommand = "ExecuteLocalActivity"
	ExecuteChildWorkflowCommand = "ExecuteChildWorkflow"
	NewTimerCommand             = "NewTimer"
	SideEffectCommand           = "SideEffect"
	GetVersionCommand           = "GetVersion"
	CompleteWorkflowCommand     = "CompleteWorkflow"

	// External workflows
	SignalExternalWorkflowCommand = "SignalExternalWorkflow"
	CancelExternalWorkflowCommand = "CancelExternalWorkflow"

	// Unified.
	CancelCommand = "Cancel"
)

type (
	// StartWorkflow sends worker command to start workflow.
	StartWorkflow struct {
		// Info to define workflow context.
		Info *workflow.Info `json:"info"`
		// Input arguments.
		Input []jsoniter.RawMessage `json:"args"`
	}

	// InvokeQuery invokes signal with a set of arguments.
	InvokeSignal struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the signal.
		Name string `json:"name"`
		// Args of the call.
		Args []jsoniter.RawMessage `json:"args"`
	}

	// InvokeQuery invokes query with a set of arguments.
	InvokeQuery struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the query.
		Name string `json:"name"`
		// Args of the call.
		Args []jsoniter.RawMessage `json:"args"`
	}

	// CancelWorkflow asks worker to gracefully stop workflow, if possible (signal).
	CancelWorkflow struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// DestroyWorkflow asks worker to offload workflow from memory.
	DestroyWorkflow struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// GetStackTrace asks worker to offload workflow from memory.
	GetStackTrace struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
	}

	// ExecuteActivity command by workflow worker.
	ExecuteActivity struct {
		// Name defines activity name.
		Name string `json:"name"`

		// Args to pass to the activity.
		Args []jsoniter.RawMessage `json:"arguments"`

		// Options to run activity.
		Options bindings.ExecuteActivityOptions `json:"options,omitempty"`

		// rawPayload represents Args converted into Temporal payload format.
		rawPayload *commonpb.Payloads
	}

	// ExecuteLocalActivity executes activity on the same node.
	ExecuteLocalActivity struct {
	}

	// ExecuteChildWorkflow executes child workflow.
	ExecuteChildWorkflow struct {
	}

	// NewTimer starts new timer.
	NewTimer struct {
		// Milliseconds defines timer duration.
		Milliseconds int `json:"ms"`
	}

	// SideEffect to be recorded into the history.
	SideEffect struct {
		Value      jsoniter.RawMessage `json:"value"`
		rawPayload *commonpb.Payloads
	}

	// NewTimer starts new timer.
	GetVersion struct {
		ChangeID     string `json:"changeID"`
		MinSupported int    `json:"minSupported"`
		MaxSupported int    `json:"maxSupported"`
	}

	// CompleteWorkflow sent by worker to complete workflow.
	CompleteWorkflow struct {
		// Result defines workflow execution result.
		Result []jsoniter.RawMessage `json:"result"`

		// Error (if any).
		Error *rrt.Error `json:"error"`

		// rawPayload represents Result converted into Temporal payload format.
		rawPayload *commonpb.Payloads
	}

	// SignalExternalWorkflow sends signal to external workflow.
	SignalExternalWorkflow struct {
		Namespace         string                `json:"namespace"`
		WorkflowID        string                `json:"workflowID"`
		RunID             string                `json:"runID"`
		Signal            string                `json:"signal"`
		ChildWorkflowOnly bool                  `json:"childWorkflowOnly"`
		Args              []jsoniter.RawMessage `json:"args"`

		// rawPayload represents Args converted into Temporal payload format.
		rawPayload *commonpb.Payloads
	}

	// CancelExternalWorkflow canceller external workflow.
	CancelExternalWorkflow struct {
		Namespace  string `json:"namespace"`
		WorkflowID string `json:"workflowID"`
		RunID      string `json:"runID"`
	}

	// Cancel one or multiple internal promises (activities, local activities, timers, child workflows).
	Cancel struct {
		// CommandIDs to be cancelled.
		CommandIDs []uint64 `json:"ids"`
	}
)

// ActivityParams maps activity command to activity params.
func (cmd ExecuteActivity) ActivityParams(env bindings.WorkflowEnvironment) bindings.ExecuteActivityParams {
	params := bindings.ExecuteActivityParams{
		ExecuteActivityOptions: cmd.Options,
		ActivityType:           bindings.ActivityType{Name: cmd.Name},
		Input:                  cmd.rawPayload,
	}

	if params.TaskQueueName == "" {
		params.TaskQueueName = env.WorkflowInfo().TaskQueueName
	}

	return params
}

// ToDuration converts timer command to time.Duration.
func (cmd NewTimer) ToDuration() time.Duration {
	return time.Millisecond * time.Duration(cmd.Milliseconds)
}

// maps worker parameters into internal command representation.
func parseCommand(dc converter.DataConverter, name string, params jsoniter.RawMessage) (interface{}, error) {
	var err error

	switch name {
	case ExecuteActivityCommand:
		cmd := ExecuteActivity{
			rawPayload: &commonpb.Payloads{},
		}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		err := rrt.ToPayloads(dc, cmd.Args, cmd.rawPayload)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case NewTimerCommand:
		cmd := NewTimer{}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case GetVersionCommand:
		cmd := GetVersion{}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case SideEffectCommand:
		cmd := SideEffect{
			rawPayload: &commonpb.Payloads{},
		}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		err = rrt.ToPayloads(dc, []jsoniter.RawMessage{cmd.Value}, cmd.rawPayload)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case CompleteWorkflowCommand:
		cmd := CompleteWorkflow{
			rawPayload: &commonpb.Payloads{},
		}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		err = rrt.ToPayloads(dc, cmd.Result, cmd.rawPayload)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case SignalExternalWorkflowCommand:
		cmd := SignalExternalWorkflow{
			rawPayload: &commonpb.Payloads{},
		}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		err = rrt.ToPayloads(dc, cmd.Args, cmd.rawPayload)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case CancelExternalWorkflowCommand:
		cmd := CancelExternalWorkflow{}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case CancelCommand:
		cmd := Cancel{}

		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	default:
		return nil, errors.E(errors.Op("parseCommand"), "undefined command type", name)
	}
}
