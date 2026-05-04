package aggregatedpool

import (
	"sync/atomic"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/temporal"
)

// NexusStartEnvelope is the discriminated DTO returned to PHP in response to
// GetNexusOperationStarted{startMsgID}. Encoded via the Temporal data converter
// (JSON), decoded on the PHP side via
// EncodedValues::getValue(0, NexusStartEnvelope::class).
//
// Wire-protocol with PHP (caller-side) — register-and-wait, no polling:
//
//   - PHP issues two requests in the same workflow task:
//     ExecuteNexusOperation{...}  (waits for completion)
//     GetNexusOperationStarted{startMsgID = msg.ID of ExecuteNexusOperation}
//
//   - SDK's started callback → wp.nexusStarted.Push(msg.ID, token, err)
//     fires the listener registered by GetNexusOperationStarted, which pushes
//     this envelope back as the start response.
//
//   - SDK's completion callback → pushes the result Payload (or Failure) as
//     the response to the original ExecuteNexusOperation request.
//
// Mirrors the ChildWorkflowStub shape (ExecuteChildWorkflow + GetChildWorkflowExecution).
type NexusStartEnvelope struct {
	Async bool   `json:"async"`
	Token string `json:"token,omitempty"`
}

// makeNexusStartedRegistryCallback adapts the SDK's `func(string, error)`
// started callback to push into the workflow's NexusStartedRegistry under
// startMsgID. Listeners registered via GetNexusOperationStarted fire when the
// entry is pushed (or immediately if already pushed).
//
// Defers via wp.callbacks if invoked from outside the workflow loop, matching
// the standard SDK callback dispatch pattern.
func (wp *Workflow) makeNexusStartedRegistryCallback(startMsgID uint64) func(string, error) {
	push := func(token string, err error) {
		wp.nexusStarted.Push(startMsgID, token, err)
	}
	return func(token string, err error) {
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			push(token, err)
			return
		}
		wp.callbacks = append(wp.callbacks, func() error {
			push(token, err)
			return nil
		})
	}
}

// makeNexusCompletionResponseCallback adapts the SDK's
// `func(*commonpb.Payload, error)` completion callback to push a result/failure
// response under startMsgID. The result Payload is wrapped in Payloads so PHP
// receives the standard ValuesInterface shape.
//
// Defers via wp.callbacks if invoked from outside the workflow loop.
func (wp *Workflow) makeNexusCompletionResponseCallback(startMsgID uint64) func(*commonpb.Payload, error) {
	deliver := func(result *commonpb.Payload, err error) {
		wp.canceller.Discard(startMsgID)
		if err != nil {
			wp.mq.PushError(startMsgID, temporal.GetDefaultFailureConverter().ErrorToFailure(err), wp.getWorkflowWorkerPid())
			return
		}
		payloads := &commonpb.Payloads{}
		if result != nil {
			payloads.Payloads = []*commonpb.Payload{result}
		}
		wp.mq.PushResponse(startMsgID, payloads, wp.getWorkflowWorkerPid())
	}
	return func(result *commonpb.Payload, err error) {
		if atomic.LoadUint32(&wp.inLoop) == 1 {
			deliver(result, err)
			return
		}
		wp.callbacks = append(wp.callbacks, func() error {
			deliver(result, err)
			return nil
		})
	}
}

// pushStartEnvelope encodes the start envelope via the data converter (JSON)
// and pushes it as a single-payload response to the GetNexusOperationStarted
// request. Used by the listener registered in the GetNexusOperationStarted
// case in handler.go.
func (wp *Workflow) pushStartEnvelope(awaitMsgID uint64, envelope NexusStartEnvelope) {
	payloads, err := wp.env.GetDataConverter().ToPayloads(envelope)
	if err != nil {
		wp.mq.PushError(awaitMsgID, temporal.GetDefaultFailureConverter().ErrorToFailure(err), wp.getWorkflowWorkerPid())
		return
	}
	wp.mq.PushResponse(awaitMsgID, payloads, wp.getWorkflowWorkerPid())
}
