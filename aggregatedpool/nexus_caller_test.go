package aggregatedpool

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/roadrunner-server/pool/payload"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	poolWorker "github.com/roadrunner-server/pool/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v5/canceller"
	"github.com/temporalio/roadrunner-temporal/v5/queue"
	"github.com/temporalio/roadrunner-temporal/v5/registry"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"
)

// ── NexusStartEnvelope JSON wire shape ────────────────────────────────

// TestNexusStartEnvelope_AsyncShape pins the JSON contract PHP relies on:
// {"async":true,"token":"…"} for async ops.
func TestNexusStartEnvelope_AsyncShape(t *testing.T) {
	data, err := json.Marshal(NexusStartEnvelope{Async: true, Token: "tok-123"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":true,"token":"tok-123"}`, string(data))
}

// TestNexusStartEnvelope_SyncShape pins the JSON contract for sync ops:
// {"async":false} — token is omitempty so the field is absent.
func TestNexusStartEnvelope_SyncShape(t *testing.T) {
	data, err := json.Marshal(NexusStartEnvelope{Async: false, Token: ""})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":false}`, string(data))
	assert.NotContains(t, string(data), "token")
}

// TestNexusStartEnvelope_RoundTripViaTemporalConverter goes through the same
// codec PHP receives the envelope from (`env.GetDataConverter().ToPayloads`).
// Guards against breaking the wire shape if the default converter ever
// changes its JSON serialisation rules.
func TestNexusStartEnvelope_RoundTripViaTemporalConverter(t *testing.T) {
	conv := converter.GetDefaultDataConverter()
	payloads, err := conv.ToPayloads(NexusStartEnvelope{Async: true, Token: "round-trip"})
	require.NoError(t, err)
	require.Len(t, payloads.Payloads, 1)

	var back NexusStartEnvelope
	require.NoError(t, conv.FromPayloads(payloads, &back))
	assert.True(t, back.Async)
	assert.Equal(t, "round-trip", back.Token)
}

// ── Callback factories — common test fixture ──────────────────────────

// stubPool is a minimal api.Pool that returns no workers. Only Workers() is
// exercised (via getWorkflowWorkerPid → pid 0 when empty); the rest panic.
type stubPool struct{}

func (stubPool) Workers() []*poolWorker.Process     { return nil }
func (stubPool) RemoveWorker(context.Context) error { panic("not used") }
func (stubPool) AddWorker() error                   { panic("not used") }
func (stubPool) QueueSize() uint64                  { panic("not used") }
func (stubPool) Reset(context.Context) error        { panic("not used") }
func (stubPool) Exec(context.Context, *payload.Payload, chan struct{}) (chan *staticPool.PExec, error) {
	panic("not used")
}

// newCallerWorkflow builds the minimum Workflow needed to exercise the
// nexus-caller callbacks: queue, canceller, callbacks slice, inLoop flag,
// nexusStarted registry. No env, no codec — those are unused on these paths.
func newCallerWorkflow(t *testing.T) *Workflow {
	t.Helper()
	return &Workflow{
		log:          zap.NewNop(),
		mq:           queue.NewMessageQueue(func() uint64 { return 0 }),
		canceller:    new(canceller.Canceller),
		nexusStarted: new(registry.NexusStartedRegistry),
		pool:         stubPool{},
	}
}

// ── makeNexusStartedRegistryCallback ──────────────────────────────────

// TestMakeNexusStartedRegistryCallback_InLoopPushesImmediately — callback
// called while inLoop==1 must Push to nexusStarted synchronously, not defer.
func TestMakeNexusStartedRegistryCallback_InLoopPushesImmediately(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	cb := wp.makeNexusStartedRegistryCallback(101)
	cb("tok-async", nil)

	// Queue must remain empty — the callback only seeds the registry.
	assert.Empty(t, wp.callbacks, "in-loop callback must not be deferred")

	var gotToken string
	wp.nexusStarted.Listen(101, func(token string, err error) {
		gotToken = token
	})
	assert.Equal(t, "tok-async", gotToken, "registry must contain the pushed entry")
}

// TestMakeNexusStartedRegistryCallback_OutOfLoopDefers — callback called
// while inLoop==0 must enqueue itself in wp.callbacks; the registry stays
// empty until the queued callback runs.
func TestMakeNexusStartedRegistryCallback_OutOfLoopDefers(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 0)

	cb := wp.makeNexusStartedRegistryCallback(202)
	cb("tok-deferred", nil)

	require.Len(t, wp.callbacks, 1, "out-of-loop callback must be deferred")

	// Listener registered before the deferred callback runs sees nothing.
	var fired bool
	wp.nexusStarted.Listen(202, func(string, error) { fired = true })
	assert.False(t, fired)

	// Run the deferred callback to simulate the next OnWorkflowTaskStarted.
	require.NoError(t, wp.callbacks[0]())
	assert.True(t, fired, "registry listener must fire once the deferred callback runs")
}

// TestMakeNexusStartedRegistryCallback_ErrorIsForwarded — start errors flow
// through the registry verbatim so the listener can surface them as a
// PushError on the GetNexusOperationStarted response.
func TestMakeNexusStartedRegistryCallback_ErrorIsForwarded(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	startErr := errors.New("start blew up")
	cb := wp.makeNexusStartedRegistryCallback(303)
	cb("", startErr)

	var gotErr error
	wp.nexusStarted.Listen(303, func(token string, err error) {
		gotErr = err
	})
	assert.Same(t, startErr, gotErr)
}

// ── makeNexusCompletionResponseCallback ───────────────────────────────

// TestMakeNexusCompletionResponseCallback_InLoopSuccess — success completion
// while inLoop==1 pushes a single-payload response and discards the canceller.
func TestMakeNexusCompletionResponseCallback_InLoopSuccess(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	payload := &commonpb.Payload{
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
		Data:     []byte(`"hello"`),
	}

	cb := wp.makeNexusCompletionResponseCallback(404)
	cb(payload, nil)

	assert.Empty(t, wp.callbacks, "in-loop callback must not be deferred")

	msgs := wp.mq.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, uint64(404), msgs[0].ID)
	assert.Nil(t, msgs[0].Failure, "success path must not push a failure")
	require.NotNil(t, msgs[0].Payloads)
	require.Len(t, msgs[0].Payloads.Payloads, 1)
	assert.Same(t, payload, msgs[0].Payloads.Payloads[0])
}

// TestMakeNexusCompletionResponseCallback_NilPayloadStillPushesEmptyResponse —
// some sync ops complete with no result payload; the callback must still push
// a response so the PHP side resolves cleanly with a 0-payload Values.
func TestMakeNexusCompletionResponseCallback_NilPayloadStillPushesEmptyResponse(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	cb := wp.makeNexusCompletionResponseCallback(505)
	cb(nil, nil)

	msgs := wp.mq.Messages()
	require.Len(t, msgs, 1)
	require.NotNil(t, msgs[0].Payloads)
	assert.Empty(t, msgs[0].Payloads.Payloads, "nil result must produce zero-payload Payloads")
	assert.Nil(t, msgs[0].Failure)
}

// TestMakeNexusCompletionResponseCallback_ErrorPath — failure flows as a
// Failure on the response; no Payloads bag is produced.
func TestMakeNexusCompletionResponseCallback_ErrorPath(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	cb := wp.makeNexusCompletionResponseCallback(606)
	cb(nil, errors.New("nexus operation failed"))

	msgs := wp.mq.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, uint64(606), msgs[0].ID)
	require.NotNil(t, msgs[0].Failure)
	assert.Equal(t, "nexus operation failed", msgs[0].Failure.Message)
	assert.Nil(t, msgs[0].Payloads)
}

// TestMakeNexusCompletionResponseCallback_OutOfLoopDefers — completion
// arriving outside the workflow loop is queued; nothing reaches the message
// queue until the deferred callback runs.
func TestMakeNexusCompletionResponseCallback_OutOfLoopDefers(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 0)

	cb := wp.makeNexusCompletionResponseCallback(707)
	cb(&commonpb.Payload{Data: []byte("x")}, nil)

	assert.Empty(t, wp.mq.Messages(), "deferred completion must not produce a message yet")
	require.Len(t, wp.callbacks, 1)

	require.NoError(t, wp.callbacks[0]())
	assert.Len(t, wp.mq.Messages(), 1, "deferred callback run produces one queued message")
}
