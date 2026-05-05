package aggregatedpool

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/goridge/v3/pkg/frame"
	"github.com/roadrunner-server/pool/payload"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"

	nexus "github.com/nexus-rpc/sdk-go/nexus"
)

// Wire contract with PHP — must match FailureConverter::NEXUS_OPERATION_ERROR_TYPE_PREFIX.
const nexusOperationErrorTypePrefix = "nexus.OperationError."

// Wire contract with PHP NexusTaskHandler. Async result: kind="async", payload data == operation token.
const (
	nexusKindMetadataKey  = "_rr_nexus_kind"
	nexusKindAsync        = "async"
	nexusLinksMetadataKey = "_rr_nexus_links" // JSON array of {url, type}
)

const (
	// Hard limit on cause-chain depth to guard against pathological PHP-side payloads.
	failureChainMaxDepth = 32
	// Timeout for the fire-and-forget CancelNexusOperationMethod RPC.
	nexusCancelMethodTimeout = 5 * time.Second
)

// NexusHandler forwards Nexus Start/Cancel to PHP workers via the activity pool.
type NexusHandler struct {
	codec api.Codec
	pool  api.Pool
	log   *zap.Logger
	// seqID is the wire-envelope ID counter for outgoing internal.Message.ID.
	seqID uint64
	// invocationSeq is the InvocationID counter, paired only with the
	// CancelNexusOperationMethod cooperative-cancel protocol. Kept separate from
	// seqID so envelope-ID generation can evolve without touching the
	// PHP-visible InvocationID semantics.
	invocationSeq uint64
	pldPool       *sync.Pool

	// inFlight gates CancelNexusOperationMethod emission to avoid racing a cancel past completion.
	inFlight sync.Map
}

// NewNexusHandler creates a new Nexus handler that forwards to PHP.
func NewNexusHandler(codec api.Codec, pool api.Pool, log *zap.Logger) *NexusHandler {
	return &NexusHandler{
		codec: codec,
		pool:  pool,
		log:   log,
		pldPool: &sync.Pool{
			New: func() any {
				return new(payload.Payload)
			},
		},
	}
}

// nexusOperation forwards Start/Cancel to PHP. methodCancelSupported gates
// InvocationID + CancelNexusOperationMethod emission. Modern workers that
// register Nexus services always set this to true; the parameter is retained
// to keep the per-operation behavior pinned at registration time and to allow
// unit-level testing of both code paths.
type nexusOperation struct {
	nexus.UnimplementedOperation[converter.RawValue, converter.RawValue]
	name                  string
	serviceName           string
	taskQueue             string
	handler               *NexusHandler
	methodCancelSupported bool
}

func (op *nexusOperation) Name() string {
	return op.name
}

func (op *nexusOperation) Start(ctx context.Context, input converter.RawValue, options nexus.StartOperationOptions) (nexus.HandlerStartOperationResult[converter.RawValue], error) {
	return op.handler.startOperation(ctx, op.taskQueue, op.serviceName, op.name, input.Payload(), options, op.methodCancelSupported)
}

func (op *nexusOperation) Cancel(ctx context.Context, token string, options nexus.CancelOperationOptions) error {
	return op.handler.cancelOperation(ctx, op.taskQueue, op.serviceName, op.name, token, options)
}

// CreateNexusService builds a nexus.Service with pass-through operations.
func (h *NexusHandler) CreateNexusService(taskQueue string, serviceName string, operationNames []string, methodCancelSupported bool) *nexus.Service {
	svc := nexus.NewService(serviceName)
	ops := make([]nexus.RegisterableOperation, 0, len(operationNames))
	for _, name := range operationNames {
		ops = append(ops, &nexusOperation{
			name:                  name,
			serviceName:           serviceName,
			taskQueue:             taskQueue,
			handler:               h,
			methodCancelSupported: methodCancelSupported,
		})
	}
	svc.MustRegister(ops...)
	return svc
}

func (h *NexusHandler) startOperation(
	ctx context.Context,
	taskQueue string,
	serviceName string,
	operationName string,
	input *commonpb.Payload,
	options nexus.StartOperationOptions,
	methodCancelSupported bool,
) (nexus.HandlerStartOperationResult[converter.RawValue], error) {
	h.log.Debug("nexus start operation", zap.String("service", serviceName), zap.String("operation", operationName), zap.String(tq, taskQueue))

	headers := make(map[string]string)
	for k, v := range options.Header {
		headers[k] = v
	}

	callbackHeaders := make(map[string]string)
	for k, v := range options.CallbackHeader {
		callbackHeaders[k] = v
	}

	links := make([]internal.NexusLink, 0, len(options.Links))
	for _, l := range options.Links {
		u := ""
		if l.URL != nil {
			u = l.URL.String()
		}
		links = append(links, internal.NexusLink{
			URL:  u,
			Type: l.Type,
		})
	}

	// invocationID is the cooperative-cancel correlation ID; emitted only when
	// the worker advertises method-cancel support. Distinct from the wire
	// envelope ID below — they are separate counters to keep their semantics
	// independent.
	var invocationID uint64
	cmd := internal.InvokeNexusOperation{
		Service:         serviceName,
		Operation:       operationName,
		RequestID:       options.RequestID,
		Callback:        options.CallbackURL,
		CallbackHeaders: callbackHeaders,
		Headers:         headers,
		Links:           links,
	}
	if methodCancelSupported {
		invocationID = atomic.AddUint64(&h.invocationSeq, 1)
		cmd.InvocationID = invocationID
	}
	msg := &internal.Message{
		ID:      atomic.AddUint64(&h.seqID, 1),
		Command: cmd,
	}

	if input != nil {
		msg.Payloads = &commonpb.Payloads{Payloads: []*commonpb.Payload{input}}
	}

	// Watch for ctx cancellation while Start is in flight; emit method cancel on PHP side.
	// Order matters on cleanup: clear inFlight BEFORE closing done, so the watcher
	// can't observe a stale entry in the brief window between the two.
	if methodCancelSupported {
		h.inFlight.Store(invocationID, struct{}{})
		done := make(chan struct{})
		defer func() {
			h.inFlight.Delete(invocationID)
			close(done)
		}()
		go h.watchForMethodCancel(ctx, invocationID, done)
	}

	pl := h.getPld()
	defer h.putPld(pl)

	if err := h.codec.Encode(&internal.Context{TaskQueue: taskQueue}, pl, msg); err != nil {
		// Encoding our own request is a deterministic local bug, not a transient
		// fault — don't ask the server to retry it.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "encode nexus request", err)
	}

	ch := make(chan struct{}, 1)
	result, err := h.pool.Exec(ctx, pl, ch)
	if err != nil {
		// Pool returned before queueing — typically pool busy / exec rejected; retryable.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeUnavailable, nexus.HandlerErrorRetryBehaviorRetryable, "exec nexus request", err)
	}

	var r *payload.Payload
	select {
	case pld := <-result:
		if pld.Error() != nil {
			// Worker-side execution failure: retryable per Nexus spec for INTERNAL.
			return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorUnspecified, "nexus worker exec error", pld.Error())
		}
		if pld.Payload().Flags&frame.STREAM != 0 {
			ch <- struct{}{}
			// Protocol-level violation by worker; non-retryable client error.
			return nil, newNexusHandlerError(nexus.HandlerErrorTypeBadRequest, nexus.HandlerErrorRetryBehaviorNonRetryable, "streaming is not supported", nil)
		}
		r = pld.Payload()
	default:
		// Pool returned a result channel without a value — should not happen on a
		// healthy pool. Treat as transient.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorRetryable, "nexus worker empty response", nil)
	}

	out := make([]*internal.Message, 0, 1)
	if err := h.codec.Decode(r, &out); err != nil {
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "decode nexus response", err)
	}

	if len(out) != 1 {
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "invalid nexus worker response", nil)
	}

	retMsg := out[0]
	if retMsg.Failure != nil {
		return nil, nexusErrorFromFailure(retMsg.Failure)
	}

	if retMsg.Payloads != nil && len(retMsg.Payloads.Payloads) > 0 {
		p := retMsg.Payloads.Payloads[0]

		// Handler links piggyback on payload metadata; strip before forwarding.
		if links := extractNexusLinks(p, h.log); len(links) > 0 {
			if nexus.IsHandlerContext(ctx) {
				nexus.AddHandlerLinks(ctx, links...)
			} else {
				h.log.Warn("nexus handler ctx missing; response links dropped", zap.Int("links", len(links)))
			}
		}

		if isAsyncPayload(p) {
			return &nexus.HandlerStartOperationResultAsync{
				OperationToken: string(p.GetData()),
			}, nil
		}

		return &nexus.HandlerStartOperationResultSync[converter.RawValue]{
			Value: converter.NewRawValue(p),
		}, nil
	}

	return &nexus.HandlerStartOperationResultSync[converter.RawValue]{}, nil
}

// extractNexusLinks reads `_rr_nexus_links` from payload metadata and strips
// the marker. Mutates p.Metadata. Bad JSON / bad URL → empty slice + warn.
func extractNexusLinks(p *commonpb.Payload, log *zap.Logger) []nexus.Link {
	if p == nil {
		return nil
	}
	md := p.GetMetadata()
	raw, ok := md[nexusLinksMetadataKey]
	if !ok {
		return nil
	}
	// Strip even on parse failure so internal markers never leak to caller.
	delete(md, nexusLinksMetadataKey)

	var entries []struct {
		URL  string `json:"url"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		log.Warn("nexus links metadata is not valid JSON", zap.Error(err))
		return nil
	}

	links := make([]nexus.Link, 0, len(entries))
	for _, e := range entries {
		if e.URL == "" || e.Type == "" {
			continue
		}
		u, err := url.Parse(e.URL)
		if err != nil {
			log.Warn("nexus link URL is malformed; skipping", zap.String("url", e.URL), zap.Error(err))
			continue
		}
		links = append(links, nexus.Link{URL: u, Type: e.Type})
	}
	return links
}

// isAsyncPayload reports whether a payload returned from the PHP worker should
// be interpreted as an async-start marker rather than a sync result.
func isAsyncPayload(p *commonpb.Payload) bool {
	if p == nil {
		return false
	}
	md := p.GetMetadata()
	if len(md) == 0 {
		return false
	}
	return string(md[nexusKindMetadataKey]) == nexusKindAsync
}

// nexusErrorFromFailure maps a PHP-emitted Failure to a Nexus SDK error:
//
//	NexusHandlerFailureInfo                       → nexus.HandlerError
//	ApplicationFailureInfo (type=nexus.Operation*) → nexus.OperationError
//	anything else                                  → HandlerError(Internal)
func nexusErrorFromFailure(f *failurepb.Failure) error {
	cause := errors.Str(failureToCauseString(f))

	if nhf := f.GetNexusHandlerFailureInfo(); nhf != nil {
		return &nexus.HandlerError{
			Type:          nexus.HandlerErrorType(nhf.GetType()),
			Message:       f.GetMessage(),
			RetryBehavior: mapNexusRetryBehavior(nhf.GetRetryBehavior()),
			Cause:         cause,
		}
	}

	if app := f.GetApplicationFailureInfo(); app != nil {
		if t := app.GetType(); strings.HasPrefix(t, nexusOperationErrorTypePrefix) {
			stateStr := strings.TrimPrefix(t, nexusOperationErrorTypePrefix)
			state := nexus.OperationState(stateStr)
			// Spec only allows failed/canceled; coerce anything else.
			if state != nexus.OperationStateFailed && state != nexus.OperationStateCanceled {
				state = nexus.OperationStateFailed
			}
			return &nexus.OperationError{
				State: state,
				Cause: cause,
			}
		}
	}

	return &nexus.HandlerError{
		Type:    nexus.HandlerErrorTypeInternal,
		Message: f.GetMessage(),
		Cause:   cause,
	}
}

// failureToCauseString flattens a Failure (incl. cause chain + stacks) into
// a Throwable.printStackTrace-style string for Nexus error Cause.
func failureToCauseString(f *failurepb.Failure) string {
	if f == nil {
		return ""
	}
	var b strings.Builder
	// Iterative + bound to guard against pathological PHP-side cause chains.
	for depth := 0; f != nil && depth < failureChainMaxDepth; depth++ {
		if depth > 0 {
			b.WriteString("Caused by: ")
		}
		if t := failureTypeTag(f); t != "" {
			b.WriteString(t)
			b.WriteString(": ")
		}
		b.WriteString(f.GetMessage())
		b.WriteByte('\n')
		if st := f.GetStackTrace(); st != "" {
			b.WriteString(st)
			if !strings.HasSuffix(st, "\n") {
				b.WriteByte('\n')
			}
		}
		f = f.GetCause()
	}
	if f != nil {
		b.WriteString("... (cause chain truncated)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// failureTypeTag returns a short label for the failure_info variant, "" if untagged.
func failureTypeTag(f *failurepb.Failure) string {
	switch {
	case f.GetApplicationFailureInfo() != nil:
		if t := f.GetApplicationFailureInfo().GetType(); t != "" {
			return t
		}
		return "ApplicationFailure"
	case f.GetNexusHandlerFailureInfo() != nil:
		if t := f.GetNexusHandlerFailureInfo().GetType(); t != "" {
			return "NexusHandlerError." + t
		}
		return "NexusHandlerError"
	case f.GetTimeoutFailureInfo() != nil:
		return "Timeout"
	case f.GetCanceledFailureInfo() != nil:
		return "Canceled"
	case f.GetTerminatedFailureInfo() != nil:
		return "Terminated"
	case f.GetServerFailureInfo() != nil:
		return "ServerFailure"
	case f.GetActivityFailureInfo() != nil:
		return "ActivityFailure"
	case f.GetChildWorkflowExecutionFailureInfo() != nil:
		return "ChildWorkflowExecutionFailure"
	default:
		return ""
	}
}

func mapNexusRetryBehavior(b enumspb.NexusHandlerErrorRetryBehavior) nexus.HandlerErrorRetryBehavior {
	switch b {
	case enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_RETRYABLE:
		return nexus.HandlerErrorRetryBehaviorRetryable
	case enumspb.NEXUS_HANDLER_ERROR_RETRY_BEHAVIOR_NON_RETRYABLE:
		return nexus.HandlerErrorRetryBehaviorNonRetryable
	default:
		return nexus.HandlerErrorRetryBehaviorUnspecified
	}
}

func (h *NexusHandler) cancelOperation(
	ctx context.Context,
	taskQueue string,
	serviceName string,
	operationName string,
	token string,
	options nexus.CancelOperationOptions,
) error {
	h.log.Debug("nexus cancel operation", zap.String("service", serviceName), zap.String("operation", operationName), zap.String("token", token), zap.String(tq, taskQueue))

	msg := &internal.Message{
		ID: atomic.AddUint64(&h.seqID, 1),
		Command: internal.CancelNexusOperation{
			Service:        serviceName,
			Operation:      operationName,
			OperationToken: token,
		},
	}

	pl := h.getPld()
	defer h.putPld(pl)

	if err := h.codec.Encode(&internal.Context{TaskQueue: taskQueue}, pl, msg); err != nil {
		return newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "encode nexus cancel request", err)
	}

	ch := make(chan struct{}, 1)
	result, err := h.pool.Exec(ctx, pl, ch)
	if err != nil {
		return newNexusHandlerError(nexus.HandlerErrorTypeUnavailable, nexus.HandlerErrorRetryBehaviorRetryable, "exec nexus cancel request", err)
	}

	select {
	case pld := <-result:
		if pld.Error() != nil {
			return newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorUnspecified, "nexus worker exec error", pld.Error())
		}
	default:
		return newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorRetryable, "nexus worker empty response", nil)
	}

	return nil
}

// newNexusHandlerError constructs a *nexus.HandlerError with explicit type and
// retry behavior. cause may be nil. The cause string is appended to Message so
// it surfaces via Error() (HandlerError.Error() doesn't print Cause), while the
// original error is preserved for errors.Unwrap / errors.Is.
func newNexusHandlerError(typ nexus.HandlerErrorType, retry nexus.HandlerErrorRetryBehavior, message string, cause error) *nexus.HandlerError {
	full := message
	if cause != nil {
		full = message + ": " + cause.Error()
	}
	return &nexus.HandlerError{
		Type:          typ,
		Message:       full,
		RetryBehavior: retry,
		Cause:         cause,
	}
}

// watchForMethodCancel: one goroutine per in-flight invocation; emits cancel on ctx.Done.
func (h *NexusHandler) watchForMethodCancel(ctx context.Context, invocationID uint64, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		if _, ok := h.inFlight.Load(invocationID); !ok {
			return
		}
		h.sendCancelMethod(invocationID, ctx.Err().Error())
	case <-done:
	}
}

// sendCancelMethod is fire-and-forget; failures are logged and swallowed.
func (h *NexusHandler) sendCancelMethod(invocationID uint64, reason string) {
	msg := &internal.Message{
		ID: atomic.AddUint64(&h.seqID, 1),
		Command: internal.CancelNexusOperationMethod{
			InvocationID: invocationID,
			Reason:       reason,
		},
	}

	pl := h.getPld()
	defer h.putPld(pl)

	if err := h.codec.Encode(&internal.Context{}, pl, msg); err != nil {
		h.log.Warn("nexus cancel method encode failed", zap.Uint64("invocationID", invocationID), zap.Error(err))
		return
	}

	// Original ctx is already cancelled; use a fresh one with timeout so we don't
	// hang forever if the pool is shutting down.
	ctx, cancel := context.WithTimeout(context.Background(), nexusCancelMethodTimeout)
	defer cancel()
	ch := make(chan struct{}, 1)
	if _, err := h.pool.Exec(ctx, pl, ch); err != nil {
		h.log.Warn("nexus cancel method exec failed", zap.Uint64("invocationID", invocationID), zap.Error(err))
	}
}

func (h *NexusHandler) getPld() *payload.Payload {
	pld, ok := h.pldPool.Get().(*payload.Payload)
	if !ok {
		return new(payload.Payload)
	}
	return pld
}

func (h *NexusHandler) putPld(pld *payload.Payload) {
	pld.Codec = 0
	pld.Context = nil
	pld.Body = nil
	h.pldPool.Put(pld)
}
