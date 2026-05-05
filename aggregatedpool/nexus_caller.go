package aggregatedpool

import (
	"sync/atomic"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/temporal"
)

// NexusStartEnvelope is the response shape for GetNexusOperationStarted.
// Encoded via the workflow's data converter (default: JSON); PHP decodes via
// EncodedValues::getValue(0, NexusStartEnvelope::class).
type NexusStartEnvelope struct {
	Async bool   `json:"async"`
	Token string `json:"token,omitempty"`
}

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

func (wp *Workflow) pushStartEnvelope(awaitMsgID uint64, envelope NexusStartEnvelope) {
	payloads, err := wp.env.GetDataConverter().ToPayloads(envelope)
	if err != nil {
		wp.mq.PushError(awaitMsgID, temporal.GetDefaultFailureConverter().ErrorToFailure(err), wp.getWorkflowWorkerPid())
		return
	}
	wp.mq.PushResponse(awaitMsgID, payloads, wp.getWorkflowWorkerPid())
}
