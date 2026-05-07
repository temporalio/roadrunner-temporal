package aggregatedpool

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/roadrunner-server/pool/payload"
	staticPool "github.com/roadrunner-server/pool/pool/static_pool"
	poolWorker "github.com/roadrunner-server/pool/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/roadrunner-temporal/v5/canceller"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	"github.com/temporalio/roadrunner-temporal/v5/queue"
	"github.com/temporalio/roadrunner-temporal/v5/registry"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

// ── Service registration ──────────────────────────────────────

// putPld must clear the payload before returning it to the pool — otherwise
// stale Body/Context bytes leak into the next Encode call.
func TestNexusHandler_PayloadPoolResetsOnPut(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())

	pld := handler.getPld()
	pld.Body = []byte("test")
	pld.Context = []byte("ctx")
	pld.Codec = 7

	handler.putPld(pld)
	assert.Nil(t, pld.Body)
	assert.Nil(t, pld.Context)
	assert.Equal(t, uint8(0), pld.Codec)
}

func TestNexusHandler_CreateNexusService_RegistersOperations(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	svc := handler.CreateNexusService("tq", "GreetingService", []string{"greet", "farewell"})

	require.NotNil(t, svc.Operation("greet"))
	require.NotNil(t, svc.Operation("farewell"))
	assert.Nil(t, svc.Operation("missing"))
}

func TestNexusHandler_CreateNexusService_AcceptsEmptyAndNilOperations(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	require.NotPanics(t, func() { handler.CreateNexusService("tq", "S", nil) })
	require.NotPanics(t, func() { handler.CreateNexusService("tq", "S", []string{}) })
}

func TestNexusHandler_CreateNexusService_IsolatesOperationsBetweenServices(t *testing.T) {
	handler := NewNexusHandler(nil, nil, zap.NewNop())
	a := handler.CreateNexusService("tq", "ServiceA", []string{"opA"})
	b := handler.CreateNexusService("tq", "ServiceB", []string{"opB"})

	assert.NotNil(t, a.Operation("opA"))
	assert.Nil(t, a.Operation("opB"))
	assert.NotNil(t, b.Operation("opB"))
	assert.Nil(t, b.Operation("opA"))
}

// Compile-time guarantee that nexusOperation satisfies the nexus SDK interfaces.
var (
	_ nexus.RegisterableOperation                             = (*nexusOperation)(nil)
	_ nexus.Operation[converter.RawValue, converter.RawValue] = (*nexusOperation)(nil)
)

// TaskQueue from CreateNexusService must reach each operation — that's the
// link the dispatch path follows when a task arrives.
func TestNexusOperation_TaskQueuePropagation(t *testing.T) {
	log := zap.NewNop()
	handler := NewNexusHandler(nil, nil, log)

	taskQueue := "my-special-queue"
	svc := handler.CreateNexusService(taskQueue, "Svc", []string{"op1", "op2", "op3"})
	require.NotNil(t, svc)

	for _, opName := range []string{"op1", "op2", "op3"} {
		op := svc.Operation(opName)
		require.NotNil(t, op)

		concrete, ok := op.(*nexusOperation)
		require.True(t, ok, "operation %q is not *nexusOperation", opName)
		assert.Equal(t, taskQueue, concrete.taskQueue, "operation %q has wrong task queue", opName)
		assert.Equal(t, "Svc", concrete.serviceName, "operation %q has wrong service name", opName)
		assert.Equal(t, opName, concrete.name)
	}
}

// ── Mock codec ─────────────────────────────────────────────

type mockCodec struct {
	encodeCalled int32
	decodeCalled int32
	encodeErr    error
	decodeErr    error
	encodedCtx   *internal.Context
	encodedMsg   *internal.Message
	decodeMsgs   []*internal.Message
}

func (m *mockCodec) Encode(ctx *internal.Context, p *payload.Payload, msgs ...*internal.Message) error {
	atomic.AddInt32(&m.encodeCalled, 1)
	m.encodedCtx = ctx
	if len(msgs) > 0 {
		m.encodedMsg = msgs[0]
	}
	if m.encodeErr != nil {
		return m.encodeErr
	}
	p.Body = []byte("encoded")
	return nil
}

func (m *mockCodec) Decode(p *payload.Payload, msgs *[]*internal.Message) error {
	atomic.AddInt32(&m.decodeCalled, 1)
	if m.decodeErr != nil {
		return m.decodeErr
	}
	*msgs = append(*msgs, m.decodeMsgs...)
	return nil
}

func (m *mockCodec) DecodeWorkerInfo(_ *payload.Payload, _ *[]*internal.WorkerInfo) error {
	return nil
}

// recordingCodec is a thread-safe codec stub that pushes every Encode'd
// message ID into a channel. Used by concurrency tests.
type recordingCodec struct {
	ids     chan<- uint64
	stopErr error
}

func (c *recordingCodec) Encode(_ *internal.Context, _ *payload.Payload, msgs ...*internal.Message) error {
	if len(msgs) > 0 {
		c.ids <- msgs[0].ID
	}
	return c.stopErr
}

func (c *recordingCodec) Decode(_ *payload.Payload, _ *[]*internal.Message) error { return nil }
func (c *recordingCodec) DecodeWorkerInfo(_ *payload.Payload, _ *[]*internal.WorkerInfo) error {
	return nil
}

// ── Encoding behavior tests (no pool needed) ───────────────────────

func TestStartOperation_EncodesTaskQueue(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, err := handler.startOperation(
		context.Background(),
		"my-task-queue",
		"GreetingService",
		"greet",
		nil,
		nexus.StartOperationOptions{},
	)

	require.Error(t, err)
	assert.NotNil(t, codec.encodedCtx, "Context should be set on encode call")
	assert.Equal(t, "my-task-queue", codec.encodedCtx.TaskQueue)
}

func TestStartOperation_EncodesAllFields(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, _ = handler.startOperation(
		context.Background(),
		"tq",
		"MyService",
		"myOp",
		nil,
		nexus.StartOperationOptions{
			RequestID:   "req-123",
			CallbackURL: "http://callback.example.com",
			Header: nexus.Header{
				"Content-Type":  "application/json",
				"Authorization": "Bearer xyz",
			},
			CallbackHeader: nexus.Header{
				"X-Token": "callback-token",
			},
		},
	)

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.InvokeNexusOperation)
	require.True(t, ok, "Command should be InvokeNexusOperation, got %T", codec.encodedMsg.Command)

	assert.Equal(t, "MyService", cmd.Service)
	assert.Equal(t, "myOp", cmd.Operation)
	assert.Equal(t, "req-123", cmd.RequestID)
	assert.Equal(t, "http://callback.example.com", cmd.Callback)
	assert.Equal(t, "application/json", cmd.Headers["Content-Type"])
	assert.Equal(t, "Bearer xyz", cmd.Headers["Authorization"])
	assert.Equal(t, "callback-token", cmd.CallbackHeaders["X-Token"])
}

func TestStartOperation_EncodesPayload(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	input := &commonpb.Payload{
		Data:     []byte("hello"),
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
	}

	_, _ = handler.startOperation(
		context.Background(),
		"tq",
		"Svc",
		"op",
		input,
		nexus.StartOperationOptions{},
	)

	require.NotNil(t, codec.encodedMsg)
	require.NotNil(t, codec.encodedMsg.Payloads)
	require.Len(t, codec.encodedMsg.Payloads.Payloads, 1)
	assert.Equal(t, []byte("hello"), codec.encodedMsg.Payloads.Payloads[0].Data)
}

func TestStartOperation_NilInputProducesNilPayloads(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, _ = handler.startOperation(
		context.Background(),
		"tq",
		"Svc",
		"op",
		nil,
		nexus.StartOperationOptions{},
	)

	require.NotNil(t, codec.encodedMsg)
	assert.Nil(t, codec.encodedMsg.Payloads, "Payloads should be nil when input is nil")
}

func TestStartOperation_EncodeErrorReturnsError(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("boom"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	result, err := handler.startOperation(
		context.Background(),
		"tq",
		"Svc",
		"op",
		nil,
		nexus.StartOperationOptions{},
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "boom")
}

func TestStartOperation_IncrementsSeqID(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, _ = handler.startOperation(context.Background(), "tq", "S", "o", nil, nexus.StartOperationOptions{})
	firstID := codec.encodedMsg.ID

	_, _ = handler.startOperation(context.Background(), "tq", "S", "o", nil, nexus.StartOperationOptions{})
	secondID := codec.encodedMsg.ID

	assert.Greater(t, secondID, firstID, "seqID should increment between calls")
}

func TestStartOperation_EncodesCallerLinks(t *testing.T) {
	codec := &mockCodec{encodeErr: errors.New("stop")}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	u1, err := url.Parse("https://caller.example/res/1")
	require.NoError(t, err)
	u2, err := url.Parse("https://caller.example/res/2")
	require.NoError(t, err)

	_, _ = handler.startOperation(context.Background(), "tq", "S", "op", nil, nexus.StartOperationOptions{
		Links: []nexus.Link{
			{URL: u1, Type: "example.one"},
			{URL: u2, Type: "example.two"},
		},
	})

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.InvokeNexusOperation)
	require.True(t, ok, "Command should be InvokeNexusOperation, got %T", codec.encodedMsg.Command)
	require.Len(t, cmd.Links, 2)
	assert.Equal(t, "https://caller.example/res/1", cmd.Links[0].URL)
	assert.Equal(t, "example.one", cmd.Links[0].Type)
	assert.Equal(t, "https://caller.example/res/2", cmd.Links[1].URL)
	assert.Equal(t, "example.two", cmd.Links[1].Type)
}

func TestStartOperation_NoLinksOmitsField(t *testing.T) {
	codec := &mockCodec{encodeErr: errors.New("stop")}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, _ = handler.startOperation(context.Background(), "tq", "S", "op", nil, nexus.StartOperationOptions{})

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.InvokeNexusOperation)
	require.True(t, ok, "Command should be InvokeNexusOperation, got %T", codec.encodedMsg.Command)
	assert.Empty(t, cmd.Links, "Links should be empty when options.Links is nil")
}

// ── Cancel encoding tests ──────────────────────────────────────

func TestCancelOperation_EncodesTaskQueue(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	err := handler.cancelOperation(
		context.Background(),
		"my-tq",
		"Svc",
		"op",
		"token-xyz",
		nexus.CancelOperationOptions{},
	)

	require.Error(t, err)
	require.NotNil(t, codec.encodedCtx)
	assert.Equal(t, "my-tq", codec.encodedCtx.TaskQueue)
}

func TestCancelOperation_EncodesAllFields(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_ = handler.cancelOperation(
		context.Background(),
		"tq",
		"GreetingService",
		"greet",
		"async-token-123",
		nexus.CancelOperationOptions{},
	)

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.CancelNexusOperation)
	require.True(t, ok, "Command should be CancelNexusOperation, got %T", codec.encodedMsg.Command)

	assert.Equal(t, "GreetingService", cmd.Service)
	assert.Equal(t, "greet", cmd.Operation)
	assert.Equal(t, "async-token-123", cmd.OperationToken)
}

func TestCancelOperation_EncodeError(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("encode failed"),
	}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	err := handler.cancelOperation(context.Background(), "tq", "S", "o", "t", nexus.CancelOperationOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encode failed")
}

// ── Concurrent seqID test ──────────────────────────────────────

func TestStartOperation_ConcurrentSeqIDIncrement(t *testing.T) {
	const goroutines = 20

	idCh := make(chan uint64, goroutines)
	codec := &recordingCodec{ids: idCh, stopErr: errors.New("stop")}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = handler.startOperation(context.Background(), "tq", "S", "o", nil, nexus.StartOperationOptions{})
		}()
	}
	wg.Wait()
	close(idCh)

	unique := make(map[uint64]struct{}, goroutines)
	for id := range idCh {
		assert.Greater(t, id, uint64(0))
		unique[id] = struct{}{}
	}
	assert.Equal(t, goroutines, len(unique), "every concurrent startOperation must get a unique seqID")
}

// ── Method cancellation tests ──────────────────────────────────

// InvocationID must equal the wire envelope ID — that's the correlation key
// CancelNexusOperationMethod uses to find the in-flight handler.
func TestStartOperation_SetsInvocationID(t *testing.T) {
	codec := &mockCodec{encodeErr: errors.New("stop")}
	handler := NewNexusHandler(codec, nil, zap.NewNop())

	_, _ = handler.startOperation(
		context.Background(), "tq", "S", "op",
		nil, nexus.StartOperationOptions{},
	)

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.InvokeNexusOperation)
	require.True(t, ok)
	assert.NotZero(t, cmd.InvocationID)
	assert.Equal(t, codec.encodedMsg.ID, cmd.InvocationID,
		"InvocationID must match the wire message ID used for correlation")
}

// ── sendCancelMethod tests (requires a minimal Pool mock) ──────

type recordingPool struct {
	execCalls        int32
	lastCtx          context.Context
	lastCtxErrAtExec error
	lastPld          *payload.Payload
	execCh           chan struct{}
}

func (p *recordingPool) Exec(ctx context.Context, pld *payload.Payload, _ chan struct{}) (chan *staticPool.PExec, error) {
	atomic.AddInt32(&p.execCalls, 1)
	p.lastCtx = ctx
	p.lastCtxErrAtExec = ctx.Err()
	p.lastPld = pld
	if p.execCh != nil {
		<-p.execCh
	}
	ch := make(chan *staticPool.PExec, 1)
	return ch, nil
}

func (p *recordingPool) Workers() []*poolWorker.Process     { panic("not used") }
func (p *recordingPool) RemoveWorker(context.Context) error { panic("not used") }
func (p *recordingPool) AddWorker() error                   { panic("not used") }
func (p *recordingPool) QueueSize() uint64                  { panic("not used") }
func (p *recordingPool) Reset(context.Context) error        { panic("not used") }

// sendCancelMethod must use a fresh context — the caller's ctx is the one
// that was just cancelled, so reusing it would mean the cancel never lands.
func TestSendCancelMethod_EncodesCorrectCommand(t *testing.T) {
	codec := &mockCodec{}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	handler.sendCancelMethod(77, "deadline exceeded")

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.CancelNexusOperationMethod)
	require.True(t, ok, "Command should be CancelNexusOperationMethod, got %T", codec.encodedMsg.Command)
	assert.Equal(t, uint64(77), cmd.InvocationID)
	assert.Equal(t, "deadline exceeded", cmd.Reason)

	assert.EqualValues(t, 1, atomic.LoadInt32(&pool.execCalls))
	require.NotNil(t, pool.lastCtx)
	assert.NoError(t, pool.lastCtxErrAtExec, "sendCancelMethod must use a live ctx at Exec time")
}

// ctx cancel while a Nexus invocation is in-flight must emit a
// CancelNexusOperationMethod so the PHP-side handler stops promptly.
func TestStartOperation_CtxCancelTriggersMethodCancel(t *testing.T) {
	codec := &mockCodec{}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	handler.inFlight.Store(uint64(5), struct{}{})
	defer handler.inFlight.Delete(uint64(5))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		handler.watchForMethodCancel(ctx, 5, done)
		close(finished)
	}()

	cancel()
	<-finished

	require.NotNil(t, codec.encodedMsg)
	cmd, ok := codec.encodedMsg.Command.(internal.CancelNexusOperationMethod)
	require.True(t, ok, "expected CancelNexusOperationMethod, got %T", codec.encodedMsg.Command)
	assert.Equal(t, uint64(5), cmd.InvocationID)
	assert.Contains(t, cmd.Reason, "canceled")
}

func TestStartOperation_DoneClosedSkipsMethodCancel(t *testing.T) {
	codec := &mockCodec{}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		handler.watchForMethodCancel(ctx, 5, done)
		close(finished)
	}()

	close(done)
	<-finished

	assert.Nil(t, codec.encodedMsg, "no cancel should have been emitted")
	assert.EqualValues(t, 0, atomic.LoadInt32(&pool.execCalls))
}

// Race guard: ctx cancels AFTER the invocation completed (inFlight already
// cleared). Watcher must swallow the cancel rather than target a gone handler.
func TestStartOperation_CtxCancelAfterCompletionNoop(t *testing.T) {
	codec := &mockCodec{}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	finished := make(chan struct{})
	go func() {
		handler.watchForMethodCancel(ctx, 777, done)
		close(finished)
	}()

	cancel()
	<-finished

	assert.Nil(t, codec.encodedMsg, "cancel must be swallowed when inFlight entry is absent")
}

// sendCancelMethod is fire-and-forget — encode failures are logged, never propagated.
func TestSendCancelMethod_EncodeErrorSwallowed(t *testing.T) {
	codec := &mockCodec{encodeErr: errors.New("encode boom")}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	handler.sendCancelMethod(1, "x")

	assert.EqualValues(t, 0, atomic.LoadInt32(&pool.execCalls))
}

// ── Failure → Nexus error mapping ──────────────────────────────────

func TestNexusErrorFromFailure_HandlerFailureInfoPreservesType(t *testing.T) {
	f := &failurepb.Failure{
		Message: "payload parsing failed",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          "BAD_REQUEST",
				RetryBehavior: enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE,
			},
		},
	}

	err := nexusErrorFromFailure(f)

	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok, "expected *nexus.HandlerError, got %T", err)
	assert.Equal(t, nexus.HandlerErrorTypeBadRequest, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorNonRetryable, he.RetryBehavior)
	assert.Equal(t, "payload parsing failed", he.Message)
}

func TestNexusErrorFromFailure_RetryBehaviorRetryable(t *testing.T) {
	f := &failurepb.Failure{
		Message: "try again",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          "INTERNAL",
				RetryBehavior: enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE,
			},
		},
	}

	err := nexusErrorFromFailure(f)
	he := err.(*nexus.HandlerError)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorRetryable, he.RetryBehavior)
}

func TestNexusErrorFromFailure_RetryBehaviorUnspecifiedDefaults(t *testing.T) {
	f := &failurepb.Failure{
		Message: "",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type: "NOT_FOUND",
			},
		},
	}

	he := nexusErrorFromFailure(f).(*nexus.HandlerError)
	assert.Equal(t, nexus.HandlerErrorTypeNotFound, he.Type)
	assert.Equal(t, nexus.HandlerErrorRetryBehaviorUnspecified, he.RetryBehavior)
}

func TestNexusErrorFromFailure_AllSpecErrorTypesRoundTrip(t *testing.T) {
	cases := []struct {
		wire string
		want nexus.HandlerErrorType
	}{
		{"BAD_REQUEST", nexus.HandlerErrorTypeBadRequest},
		{"UNAUTHENTICATED", nexus.HandlerErrorTypeUnauthenticated},
		{"UNAUTHORIZED", nexus.HandlerErrorTypeUnauthorized},
		{"NOT_FOUND", nexus.HandlerErrorTypeNotFound},
		{"REQUEST_TIMEOUT", nexus.HandlerErrorTypeRequestTimeout},
		{"CONFLICT", nexus.HandlerErrorTypeConflict},
		{"RESOURCE_EXHAUSTED", nexus.HandlerErrorTypeResourceExhausted},
		{"INTERNAL", nexus.HandlerErrorTypeInternal},
		{"NOT_IMPLEMENTED", nexus.HandlerErrorTypeNotImplemented},
		{"UNAVAILABLE", nexus.HandlerErrorTypeUnavailable},
		{"UPSTREAM_TIMEOUT", nexus.HandlerErrorTypeUpstreamTimeout},
	}

	for _, c := range cases {
		t.Run(c.wire, func(t *testing.T) {
			f := &failurepb.Failure{
				Message: c.wire,
				FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
					NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
						Type: c.wire,
					},
				},
			}
			he := nexusErrorFromFailure(f).(*nexus.HandlerError)
			assert.Equal(t, c.want, he.Type)
		})
	}
}

func TestNexusErrorFromFailure_OperationErrorFailed(t *testing.T) {
	f := &failurepb.Failure{
		Message: "user rejected",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.failed",
			},
		},
	}

	err := nexusErrorFromFailure(f)
	oe, ok := err.(*nexus.OperationError)
	require.True(t, ok, "expected *nexus.OperationError, got %T", err)
	assert.Equal(t, nexus.OperationStateFailed, oe.State)
}

func TestNexusErrorFromFailure_OperationErrorCanceled(t *testing.T) {
	f := &failurepb.Failure{
		Message: "user canceled",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.canceled",
			},
		},
	}

	oe := nexusErrorFromFailure(f).(*nexus.OperationError)
	assert.Equal(t, nexus.OperationStateCanceled, oe.State)
}

func TestNexusErrorFromFailure_OperationErrorUnknownStateFallsBackToFailed(t *testing.T) {
	f := &failurepb.Failure{
		Message: "weird",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "nexus.OperationError.weird",
			},
		},
	}

	oe := nexusErrorFromFailure(f).(*nexus.OperationError)
	assert.Equal(t, nexus.OperationStateFailed, oe.State, "unknown state must not leak to the wire")
}

func TestNexusErrorFromFailure_UntaggedApplicationFailureFallsBackToInternal(t *testing.T) {
	f := &failurepb.Failure{
		Message: "boom",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: "SomeUserType",
			},
		},
	}

	he, ok := nexusErrorFromFailure(f).(*nexus.HandlerError)
	require.True(t, ok)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type, "unknown failure shape must collapse to Internal")
	assert.Equal(t, "boom", he.Message)
}

func TestNexusErrorFromFailure_NoFailureInfoIsInternal(t *testing.T) {
	f := &failurepb.Failure{Message: "bare failure"}

	he, ok := nexusErrorFromFailure(f).(*nexus.HandlerError)
	require.True(t, ok)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Equal(t, "bare failure", he.Message)
}

// ── RetryBehavior enum mapping ──────────────────────────────────────

func TestMapNexusRetryBehavior_AllValues(t *testing.T) {
	cases := []struct {
		in   enumspb.NexusHandlerErrorRetryBehavior
		want nexus.HandlerErrorRetryBehavior
	}{
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_UNSPECIFIED, nexus.HandlerErrorRetryBehaviorUnspecified},
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE, nexus.HandlerErrorRetryBehaviorRetryable},
		{enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE, nexus.HandlerErrorRetryBehaviorNonRetryable},
	}

	for _, c := range cases {
		t.Run(c.in.String(), func(t *testing.T) {
			got := mapNexusRetryBehavior(c.in)
			assert.Equal(t, c.want, got)
		})
	}
}

// ── Failure-cause preservation tests ───────────────────────────

// Regression: PHP stack trace must surface in Cause, not just the message.
func TestNexusErrorFromFailure_PreservesStackTrace(t *testing.T) {
	f := &failurepb.Failure{
		Message:    "boom",
		StackTrace: "#0 /app/Handler.php(42): run()\n#1 {main}",
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{
				Type:          "INTERNAL",
				RetryBehavior: enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE,
			},
		},
	}

	err := nexusErrorFromFailure(f)
	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok)

	assert.Equal(t, "boom", he.Message)
	causeStr := he.Cause.Error()
	assert.Contains(t, causeStr, "boom")
	assert.Contains(t, causeStr, "/app/Handler.php(42)")
	assert.Contains(t, causeStr, "NexusHandlerError")
}

func TestNexusErrorFromFailure_PreservesNestedCauses(t *testing.T) {
	inner := &failurepb.Failure{
		Message:    "db connection failed",
		StackTrace: "at Db->connect()",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{Type: "PDOException"},
		},
	}
	outer := &failurepb.Failure{
		Message:    "handler failed",
		StackTrace: "at EchoService->echo()",
		Cause:      inner,
		FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
			NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{Type: "INTERNAL"},
		},
	}

	err := nexusErrorFromFailure(outer)
	causeStr := err.(*nexus.HandlerError).Cause.Error()

	assert.Contains(t, causeStr, "handler failed")
	assert.Contains(t, causeStr, "EchoService->echo()")
	assert.Contains(t, causeStr, "Caused by:")
	assert.Contains(t, causeStr, "db connection failed")
	assert.Contains(t, causeStr, "Db->connect()")
	assert.Contains(t, causeStr, "PDOException")
}

func TestNexusErrorFromFailure_OperationErrorPreservesCause(t *testing.T) {
	f := &failurepb.Failure{
		Message:    "order rejected",
		StackTrace: "at OrderService->process()",
		FailureInfo: &failurepb.Failure_ApplicationFailureInfo{
			ApplicationFailureInfo: &failurepb.ApplicationFailureInfo{
				Type: nexusOperationErrorTypePrefix + "failed",
			},
		},
	}

	err := nexusErrorFromFailure(f)
	opErr, ok := err.(*nexus.OperationError)
	require.True(t, ok)

	assert.Equal(t, nexus.OperationStateFailed, opErr.State)
	causeStr := opErr.Cause.Error()
	assert.Contains(t, causeStr, "order rejected")
	assert.Contains(t, causeStr, "OrderService->process()")
}

func TestFailureToCauseString_NilIsEmpty(t *testing.T) {
	assert.Equal(t, "", failureToCauseString(nil))
}

// Pathological deep cause chain from PHP must truncate, not grow unbounded.
func TestFailureToCauseString_TruncatesDeepChain(t *testing.T) {
	depth := failureChainMaxDepth*3 + 5
	var head *failurepb.Failure
	for i := 0; i < depth; i++ {
		head = &failurepb.Failure{
			Message: "lvl",
			Cause:   head,
		}
	}

	out := failureToCauseString(head)
	assert.Contains(t, out, "... (cause chain truncated)")
	assert.LessOrEqual(t, strings.Count(out, "lvl"), failureChainMaxDepth)
}

// ── nexusLinksFromInternal tests ───────────────────────────────

func TestNexusLinksFromInternal_EmptyInputReturnsNil(t *testing.T) {
	assert.Nil(t, nexusLinksFromInternal(nil, zap.NewNop()))
	assert.Nil(t, nexusLinksFromInternal([]internal.NexusLink{}, zap.NewNop()))
}

func TestNexusLinksFromInternal_DropsEntriesWithEmptyFields(t *testing.T) {
	in := []internal.NexusLink{
		{URL: "", Type: "t"},
		{URL: "http://a/", Type: ""},
		{URL: "http://b/", Type: "t"},
	}
	out := nexusLinksFromInternal(in, zap.NewNop())
	require.Len(t, out, 1)
	assert.Equal(t, "http://b/", out[0].URL.String())
	assert.Equal(t, "t", out[0].Type)
}

func TestNexusLinksFromInternal_DropsUnparseableURLs(t *testing.T) {
	in := []internal.NexusLink{
		{URL: "http://[::bad", Type: "t"},
		{URL: "http://ok/", Type: "t"},
	}
	out := nexusLinksFromInternal(in, zap.NewNop())
	require.Len(t, out, 1)
	assert.Equal(t, "http://ok/", out[0].URL.String())
}

func TestNexusLinksFromInternal_PreservesOrderingAndFields(t *testing.T) {
	in := []internal.NexusLink{
		{URL: "http://a/", Type: "x.one"},
		{URL: "http://b/", Type: "x.two"},
	}
	out := nexusLinksFromInternal(in, zap.NewNop())
	require.Len(t, out, 2)
	assert.Equal(t, "http://a/", out[0].URL.String())
	assert.Equal(t, "x.one", out[0].Type)
	assert.Equal(t, "http://b/", out[1].URL.String())
	assert.Equal(t, "x.two", out[1].Type)
}

// ── decodeStartReply tests ─────────────────────────────────────

// Sync reply: Command=*NexusOperationStarted{Async:false}, Payloads carries
// the result. Decoder must wrap it as HandlerStartOperationResultSync with
// the payload preserved on the RawValue.
func TestDecodeStartReply_SyncSuccessUnwrapsPayload(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	resultPayload := &commonpb.Payload{
		Data:     []byte(`{"ok":true}`),
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
	}
	msg := &internal.Message{
		Command: &internal.NexusOperationStarted{
			Async: false,
			Links: []internal.NexusLink{{URL: "http://x/y", Type: "t"}},
		},
		Payloads: &commonpb.Payloads{Payloads: []*commonpb.Payload{resultPayload}},
	}

	res, err := handler.decodeStartReply(context.Background(), msg)
	require.NoError(t, err)
	sync, ok := res.(*nexus.HandlerStartOperationResultSync[converter.RawValue])
	require.True(t, ok, "expected HandlerStartOperationResultSync, got %T", res)
	assert.Equal(t, resultPayload, sync.Value.Payload())
}

// Sync reply with empty Payloads slice: still returns a sync result with
// nil Value — matches the pre-refactor empty-payload contract.
func TestDecodeStartReply_SyncSuccessEmptyPayloads(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	msg := &internal.Message{
		Command:  &internal.NexusOperationStarted{Async: false},
		Payloads: &commonpb.Payloads{},
	}

	res, err := handler.decodeStartReply(context.Background(), msg)
	require.NoError(t, err)
	sync, ok := res.(*nexus.HandlerStartOperationResultSync[converter.RawValue])
	require.True(t, ok)
	assert.Nil(t, sync.Value.Payload())
}

// Async reply: Command=*NexusOperationStarted{Async:true, Token}, no Payloads.
// Decoder returns HandlerStartOperationResultAsync with the token preserved.
func TestDecodeStartReply_AsyncSuccessReturnsToken(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	msg := &internal.Message{
		Command: &internal.NexusOperationStarted{
			Async: true,
			Token: "tok-1",
			Links: []internal.NexusLink{{URL: "http://x/y", Type: "t"}},
		},
	}

	res, err := handler.decodeStartReply(context.Background(), msg)
	require.NoError(t, err)
	async, ok := res.(*nexus.HandlerStartOperationResultAsync)
	require.True(t, ok, "expected HandlerStartOperationResultAsync, got %T", res)
	assert.Equal(t, "tok-1", async.OperationToken)
}

// nil Command + Failure set → existing failurepb mapping path. The mapping
// itself is exercised in ── Failure → Nexus error mapping ── above; here we
// verify routing.
func TestDecodeStartReply_NilCommandWithFailureRoutesToMapping(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	msg := &internal.Message{
		Failure: &failurepb.Failure{
			Message: "boom",
			FailureInfo: &failurepb.Failure_NexusHandlerFailureInfo{
				NexusHandlerFailureInfo: &failurepb.NexusHandlerFailureInfo{Type: "INTERNAL"},
			},
		},
	}

	res, err := handler.decodeStartReply(context.Background(), msg)
	assert.Nil(t, res)
	require.Error(t, err)
	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok, "expected *nexus.HandlerError, got %T", err)
	assert.Equal(t, "boom", he.Message)
}

// nil Command + nil Failure: malformed reply → HandlerError(Internal).
func TestDecodeStartReply_EmptyReplyIsHandlerError(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	res, err := handler.decodeStartReply(context.Background(), &internal.Message{})
	assert.Nil(t, res)
	require.Error(t, err)
	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok, "expected *nexus.HandlerError, got %T", err)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Contains(t, he.Message, "neither command nor failure")
}

// Unknown reply command → HandlerError(Internal). Defends against PHP
// emitting a command name Go doesn't recognize.
func TestDecodeStartReply_UnknownCommandIsHandlerError(t *testing.T) {
	handler := NewNexusHandler(&mockCodec{}, nil, zap.NewNop())

	msg := &internal.Message{Command: &internal.CancelNexusOperation{}}

	res, err := handler.decodeStartReply(context.Background(), msg)
	assert.Nil(t, res)
	require.Error(t, err)
	he, ok := err.(*nexus.HandlerError)
	require.True(t, ok)
	assert.Equal(t, nexus.HandlerErrorTypeInternal, he.Type)
	assert.Contains(t, he.Message, "unexpected")
}

// ── Caller-side: NexusStartEnvelope JSON wire shape ──────────────

func TestNexusStartEnvelope_AsyncShape(t *testing.T) {
	data, err := json.Marshal(NexusStartEnvelope{Async: true, Token: "tok-123"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":true,"token":"tok-123"}`, string(data))
}

func TestNexusStartEnvelope_SyncShape(t *testing.T) {
	data, err := json.Marshal(NexusStartEnvelope{Async: false, Token: ""})
	require.NoError(t, err)
	assert.JSONEq(t, `{"async":false}`, string(data))
	assert.NotContains(t, string(data), "token")
}

// Round-trip through the actual data converter PHP receives the envelope
// from — guards against silent breakage if the default converter changes.
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

// ── Caller-side callbacks fixture ─────────────────────────────────

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

func TestMakeNexusStartedRegistryCallback_InLoopPushesImmediately(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	cb := wp.makeNexusStartedRegistryCallback(101)
	cb("tok-async", nil)

	assert.Empty(t, wp.callbacks, "in-loop callback must not be deferred")

	var gotToken string
	wp.nexusStarted.Listen(101, func(token string, err error) {
		gotToken = token
	})
	assert.Equal(t, "tok-async", gotToken, "registry must contain the pushed entry")
}

func TestMakeNexusStartedRegistryCallback_OutOfLoopDefers(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 0)

	cb := wp.makeNexusStartedRegistryCallback(202)
	cb("tok-deferred", nil)

	require.Len(t, wp.callbacks, 1, "out-of-loop callback must be deferred")

	var fired bool
	wp.nexusStarted.Listen(202, func(string, error) { fired = true })
	assert.False(t, fired)

	require.NoError(t, wp.callbacks[0]())
	assert.True(t, fired, "registry listener must fire once the deferred callback runs")
}

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

func TestMakeNexusCompletionResponseCallback_InLoopSuccess(t *testing.T) {
	wp := newCallerWorkflow(t)
	atomic.StoreUint32(&wp.inLoop, 1)

	pl := &commonpb.Payload{
		Metadata: map[string][]byte{"encoding": []byte("json/plain")},
		Data:     []byte(`"hello"`),
	}

	cb := wp.makeNexusCompletionResponseCallback(404)
	cb(pl, nil)

	assert.Empty(t, wp.callbacks, "in-loop callback must not be deferred")

	msgs := wp.mq.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, uint64(404), msgs[0].ID)
	assert.Nil(t, msgs[0].Failure, "success path must not push a failure")
	require.NotNil(t, msgs[0].Payloads)
	require.Len(t, msgs[0].Payloads.Payloads, 1)
	assert.Same(t, pl, msgs[0].Payloads.Payloads[0])
}

// Sync ops can complete with nil payload — PHP still needs a response with
// a zero-Payloads bag, not no message at all.
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
