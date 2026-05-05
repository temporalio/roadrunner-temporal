package aggregatedpool

import (
	"context"
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
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.uber.org/zap"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

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
	// Always capture inputs for assertions
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

// ── Encoding behavior tests (no pool needed) ───────────────────────

func TestStartOperation_EncodesTaskQueue(t *testing.T) {
	codec := &mockCodec{
		encodeErr: errors.New("stop after encode"), // stop before pool.Exec
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
		nil, // nil input
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

	// Thread-safe codec that records every ID it sees, then aborts to keep the
	// path short. A single shared handler exercises the atomic seqID counter.
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

// ── Method cancellation tests ──────────────────────────────────

// TestStartOperation_SetsInvocationID verifies that every Start carries an
// InvocationID matching the wire envelope ID (`msg.ID`) — that's the
// correlation key for CancelNexusOperationMethod.
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
	lastCtxErrAtExec error // ctx.Err() captured at Exec time (caller may cancel afterwards)
	lastPld          *payload.Payload
	execCh           chan struct{} // optional: if non-nil, Exec blocks until drained
}

func (p *recordingPool) Exec(ctx context.Context, pld *payload.Payload, _ chan struct{}) (chan *staticPool.PExec, error) {
	atomic.AddInt32(&p.execCalls, 1)
	p.lastCtx = ctx
	p.lastCtxErrAtExec = ctx.Err()
	p.lastPld = pld
	if p.execCh != nil {
		<-p.execCh
	}
	// Return an empty buffered channel so the caller's select falls into `default`.
	ch := make(chan *staticPool.PExec, 1)
	return ch, nil
}

// The other methods on api.Pool are unused here; panic to catch accidental
// calls rather than silently no-op.
func (p *recordingPool) Workers() []*poolWorker.Process     { panic("not used") }
func (p *recordingPool) RemoveWorker(context.Context) error { panic("not used") }
func (p *recordingPool) AddWorker() error                   { panic("not used") }
func (p *recordingPool) QueueSize() uint64                  { panic("not used") }
func (p *recordingPool) Reset(context.Context) error        { panic("not used") }

// TestSendCancelMethod_EncodesCorrectCommand verifies the CancelNexusOperationMethod
// message built by sendCancelMethod carries the right invocation id and reason
// and uses a background context (not the cancelled one).
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

	// Exec must have been called with a fresh ctx — the cancel must still go
	// through even when the caller's ctx is already done.
	assert.EqualValues(t, 1, atomic.LoadInt32(&pool.execCalls))
	require.NotNil(t, pool.lastCtx)
	assert.NoError(t, pool.lastCtxErrAtExec, "sendCancelMethod must use a live ctx at Exec time")
}

// TestStartOperation_CtxCancelTriggersMethodCancel exercises the full
// goroutine that watches for ctx cancellation while Start is blocked in the
// PHP pool. We block the pool indefinitely, then cancel the caller's ctx,
// and verify that the watcher emits a CancelNexusOperationMethod to the pool.
func TestStartOperation_CtxCancelTriggersMethodCancel(t *testing.T) {
	// Encode fails so Start returns quickly — we only care that the method
	// cancel path fires. Without a cooperating pool stub we can't easily
	// exercise the "pool blocks, then ctx cancels, then unblocks" path from
	// Start's perspective, so we call watchForMethodCancel directly with the
	// inFlight bookkeeping set up as Start would.
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

// TestStartOperation_DoneClosedSkipsMethodCancel verifies the watcher exits
// cleanly (without sending a cancel) when Start completes before ctx
// cancellation.
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

// TestStartOperation_CtxCancelAfterCompletionNoop verifies the race where
// the invocation completed (inFlight cleared) before ctx cancellation is
// handled. The second-check on inFlight inside watchForMethodCancel must
// swallow the cancel to avoid targeting a handler that's already gone.
func TestStartOperation_CtxCancelAfterCompletionNoop(t *testing.T) {
	codec := &mockCodec{}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	// Deliberately do NOT populate inFlight — models "handler already done".
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

// ── Failure-cause preservation tests ───────────────────────────

// TestNexusErrorFromFailure_PreservesStackTrace verifies the Go side surfaces
// the PHP stack trace inside the nexus error's Cause. Before the fix the Cause
// held only the message, so PHP tracebacks were silently dropped at the RR
// boundary.
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
	// Cause must carry both the type tag and the full stack.
	causeStr := he.Cause.Error()
	assert.Contains(t, causeStr, "boom")
	assert.Contains(t, causeStr, "/app/Handler.php(42)")
	assert.Contains(t, causeStr, "NexusHandlerError")
}

func TestNexusErrorFromFailure_PreservesNestedCauses(t *testing.T) {
	// Two-level cause chain: outer Handler wraps inner Application.
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

// TestFailureToCauseString_TruncatesDeepChain guards against pathological
// PHP-side cause chains causing unbounded growth (or, with the old recursive
// version, stack overflow).
func TestFailureToCauseString_TruncatesDeepChain(t *testing.T) {
	// Build a chain ~3x deeper than the limit.
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
	// Sanity: number of "lvl" lines must not exceed the bound.
	assert.LessOrEqual(t, strings.Count(out, "lvl"), failureChainMaxDepth)
}

// ── extractNexusLinks tests ────────────────────────────────────

func TestExtractNexusLinks_ParsesValidMetadata(t *testing.T) {
	p := &commonpb.Payload{
		Data: []byte("result"),
		Metadata: map[string][]byte{
			"encoding":            []byte("json/plain"),
			nexusLinksMetadataKey: []byte(`[{"url":"http://a/b","type":"x.y"},{"url":"http://c/d","type":"p.q"}]`),
		},
	}

	links := extractNexusLinks(p, zap.NewNop())

	require.Len(t, links, 2)
	assert.Equal(t, "http://a/b", links[0].URL.String())
	assert.Equal(t, "x.y", links[0].Type)
	assert.Equal(t, "http://c/d", links[1].URL.String())
	assert.Equal(t, "p.q", links[1].Type)

	// The marker must be stripped so user-visible metadata never leaks the
	// internal `_rr_nexus_*` namespace.
	_, leaked := p.Metadata[nexusLinksMetadataKey]
	assert.False(t, leaked, "links metadata key must be removed after extraction")
	// Unrelated metadata must be preserved.
	assert.Equal(t, []byte("json/plain"), p.Metadata["encoding"])
}

func TestExtractNexusLinks_AbsentMarkerReturnsNil(t *testing.T) {
	p := &commonpb.Payload{Metadata: map[string][]byte{"encoding": []byte("json/plain")}}

	assert.Nil(t, extractNexusLinks(p, zap.NewNop()))
}

func TestExtractNexusLinks_MalformedJSONDropsAndStrips(t *testing.T) {
	p := &commonpb.Payload{
		Metadata: map[string][]byte{
			nexusLinksMetadataKey: []byte("not json"),
		},
	}

	assert.Nil(t, extractNexusLinks(p, zap.NewNop()))
	_, leaked := p.Metadata[nexusLinksMetadataKey]
	assert.False(t, leaked, "bad marker must still be stripped to avoid leaking internal bytes")
}

func TestExtractNexusLinks_SkipsMalformedEntries(t *testing.T) {
	p := &commonpb.Payload{
		Metadata: map[string][]byte{
			// One empty url, one empty type, one bad URL, one valid.
			nexusLinksMetadataKey: []byte(
				`[{"url":"","type":"t"},{"url":"u","type":""},{"url":"http://ok/","type":"t"}]`,
			),
		},
	}

	links := extractNexusLinks(p, zap.NewNop())
	require.Len(t, links, 1, "only well-formed entries survive")
	assert.Equal(t, "http://ok/", links[0].URL.String())
}

func TestExtractNexusLinks_NilPayloadSafe(t *testing.T) {
	assert.Nil(t, extractNexusLinks(nil, zap.NewNop()))
}

// TestSendCancelMethod_EncodeErrorSwallowed verifies that a codec error during
// cancel encoding is logged but not propagated — this is fire-and-forget.
func TestSendCancelMethod_EncodeErrorSwallowed(t *testing.T) {
	codec := &mockCodec{encodeErr: errors.New("encode boom")}
	pool := &recordingPool{}
	handler := NewNexusHandler(codec, pool, zap.NewNop())

	// Must not panic.
	handler.sendCancelMethod(1, "x")

	// Pool.Exec should NOT have been called because encoding failed first.
	assert.EqualValues(t, 0, atomic.LoadInt32(&pool.execCalls))
}
