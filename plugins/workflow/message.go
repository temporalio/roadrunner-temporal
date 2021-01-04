package workflow

import (
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
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
	ExecuteChildWorkflowCommand = "ExecuteChildWorkflow"
	GetRunIDCommand             = "GetRunID"

	NewTimerCommand         = "NewTimer"
	SideEffectCommand       = "SideEffect"
	GetVersionCommand       = "GetVersion"
	CompleteWorkflowCommand = "CompleteWorkflow"
	ContinueAsNewCommand    = "ContinueAsNew"

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
		Input []*commonpb.Payload `json:"args"`
	}

	// InvokeQuery invokes signal with a set of arguments.
	InvokeSignal struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the signal.
		Name string `json:"name"`
		// Args of the call.
		Args []*commonpb.Payload `json:"args"`
	}

	// InvokeQuery invokes query with a set of arguments.
	InvokeQuery struct {
		// RunID workflow run id.
		RunID string `json:"runId"`
		// Name of the query.
		Name string `json:"name"`
		// Args of the call.
		Args []*commonpb.Payload `json:"args"`
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
		Args []*commonpb.Payload `json:"arguments"`
		// Options to run activity.
		Options bindings.ExecuteActivityOptions `json:"options,omitempty"`
	}

	// ExecuteChildWorkflow executes child workflow.
	ExecuteChildWorkflow struct {
		// Name defines workflow name.
		Name string `json:"name"`
		// Input to pass to the workflow.
		Input []*commonpb.Payload `json:"input"`
		// Options to run activity.
		Options bindings.WorkflowOptions `json:"options,omitempty"`
	}

	// GetRunID returns the WorkflowID and RunId of child workflow.
	GetRunID struct {
		// ID of child workflow command.
		ID uint64
	}

	// NewTimer starts new timer.
	NewTimer struct {
		// Milliseconds defines timer duration.
		Milliseconds int `json:"ms"`
	}

	// SideEffect to be recorded into the history.
	SideEffect struct {
		Value *commonpb.Payload `json:"value"`
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
		Result []*commonpb.Payload `json:"result"`

		// Error (if any).
		Error *rrt.Error `json:"error"`
	}

	// ContinueAsNew restarts workflow with new running instance.
	ContinueAsNew struct {
		// Result defines workflow execution result.
		Name string `json:"name"`

		// Result defines workflow execution result. todo: needed to be in form of Payload
		Input []*commonpb.Payload `json:"input"`
	}

	// SignalExternalWorkflow sends signal to external workflow.
	SignalExternalWorkflow struct {
		Namespace         string              `json:"namespace"`
		WorkflowID        string              `json:"workflowID"`
		RunID             string              `json:"runID"`
		Signal            string              `json:"signal"`
		ChildWorkflowOnly bool                `json:"childWorkflowOnly"`
		Args              []*commonpb.Payload `json:"args"`

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
		Input:                  &commonpb.Payloads{Payloads: cmd.Args},
	}

	if params.TaskQueueName == "" {
		params.TaskQueueName = env.WorkflowInfo().TaskQueueName
	}

	return params
}

// ActivityParams maps activity command to activity params.
func (cmd ExecuteChildWorkflow) WorkflowParams(env bindings.WorkflowEnvironment) bindings.ExecuteWorkflowParams {
	params := bindings.ExecuteWorkflowParams{
		WorkflowOptions: cmd.Options,
		WorkflowType:    &bindings.WorkflowType{Name: cmd.Name},
		Input:           &commonpb.Payloads{Payloads: cmd.Input},
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
func parseCommand(name string, params jsoniter.RawMessage) (interface{}, error) {
	var err error

	// todo: switch to Protobuf
	switch name {
	case ExecuteActivityCommand:
		cmd := ExecuteActivity{}
		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case ExecuteChildWorkflowCommand:
		cmd := ExecuteChildWorkflow{}
		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case GetRunIDCommand:
		cmd := GetRunID{}
		err = jsoniter.Unmarshal(params, &cmd)
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
		cmd := SideEffect{}
		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case CompleteWorkflowCommand:
		cmd := CompleteWorkflow{}
		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case ContinueAsNewCommand:
		cmd := ContinueAsNew{}
		err = jsoniter.Unmarshal(params, &cmd)
		if err != nil {
			return nil, err
		}

		return cmd, nil

	case SignalExternalWorkflowCommand:
		cmd := SignalExternalWorkflow{}
		err = jsoniter.Unmarshal(params, &cmd)
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
