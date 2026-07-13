package aggregatedpool

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/goridge/v3/pkg/frame"
	"github.com/roadrunner-server/pool/payload"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.uber.org/zap"

	"github.com/nexus-rpc/sdk-go/nexus"
)

// Wire contract with PHP — must match FailureConverter::NEXUS_OPERATION_ERROR_TYPE_PREFIX.
const nexusOperationErrorTypePrefix = "nexus.OperationError."

// Timeout for the fire-and-forget CancelNexusOperationMethod RPC.
const nexusCancelMethodTimeout = 5 * time.Second

// NexusHandler forwards handler-side Nexus Start/Cancel to PHP via the activity pool.
type NexusHandler struct {
	codec     api.Codec
	pool      api.Pool
	log       *zap.Logger
	namespace string
	// seqID is the wire envelope ID; invocationSeq is the InvocationID seen by
	// PHP and used by CancelNexusOperationMethod. Kept separate so wire format
	// can evolve without touching cooperative-cancel semantics.
	seqID         uint64
	invocationSeq uint64
	pldPool       *sync.Pool
	// inFlight gates CancelNexusOperationMethod emission so we don't race a
	// cancel past Start completion.
	inFlight sync.Map
}

func NewNexusHandler(codec api.Codec, pool api.Pool, log *zap.Logger, namespace string) *NexusHandler {
	return &NexusHandler{
		codec:     codec,
		pool:      pool,
		log:       log,
		namespace: namespace,
		pldPool: &sync.Pool{
			New: func() any {
				return new(payload.Payload)
			},
		},
	}
}

type nexusOperation struct {
	nexus.UnimplementedOperation[converter.RawValue, converter.RawValue]
	name        string
	serviceName string
	taskQueue   string
	handler     *NexusHandler
}

func (op *nexusOperation) Name() string {
	return op.name
}

func (op *nexusOperation) Start(ctx context.Context, input converter.RawValue, options nexus.StartOperationOptions) (nexus.HandlerStartOperationResult[converter.RawValue], error) {
	return op.handler.startOperation(ctx, op.taskQueue, op.serviceName, op.name, input.Payload(), options)
}

func (op *nexusOperation) Cancel(ctx context.Context, token string, options nexus.CancelOperationOptions) error {
	return op.handler.cancelOperation(ctx, op.taskQueue, op.serviceName, op.name, token, options)
}

// CreateNexusService builds a nexus.Service with pass-through operations.
func (h *NexusHandler) CreateNexusService(taskQueue string, serviceName string, operationNames []string) *nexus.Service {
	svc := nexus.NewService(serviceName)
	ops := make([]nexus.RegisterableOperation, 0, len(operationNames))
	for _, name := range operationNames {
		ops = append(ops, &nexusOperation{
			name:        name,
			serviceName: serviceName,
			taskQueue:   taskQueue,
			handler:     h,
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
) (nexus.HandlerStartOperationResult[converter.RawValue], error) {
	h.log.Debug("nexus start operation", zap.String("service", serviceName), zap.String("operation", operationName), zap.String(tq, taskQueue))

	links := nexusLinksToInternal(options.Links)

	invocationID := atomic.AddUint64(&h.invocationSeq, 1)
	msg := &internal.Message{
		ID: atomic.AddUint64(&h.seqID, 1),
		Command: internal.InvokeNexusOperation{
			Service:         serviceName,
			Operation:       operationName,
			Namespace:       h.namespace,
			TaskQueue:       taskQueue,
			RequestID:       options.RequestID,
			Callback:        options.CallbackURL,
			CallbackHeaders: maps.Clone(options.CallbackHeader),
			Headers:         maps.Clone(options.Header),
			Links:           links,
			InvocationID:    invocationID,
		},
	}

	if input != nil {
		msg.Payloads = &commonpb.Payloads{Payloads: []*commonpb.Payload{input}}
	}

	// Watch ctx cancellation during Start; emit method cancel to PHP. The inFlight-before-done
	// ordering only shrinks the stale-observe window; a stale cancel is harmless (best-effort).
	h.inFlight.Store(invocationID, struct{}{})
	done := make(chan struct{})
	defer func() {
		h.inFlight.Delete(invocationID)
		close(done)
	}()
	go h.watchForMethodCancel(ctx, invocationID, done)

	r, err := h.roundTrip(ctx, taskQueue, msg, "nexus request")
	if err != nil {
		return nil, err
	}

	out := make([]*internal.Message, 0, 1)
	if err := h.codec.Decode(r, &out); err != nil {
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "decode nexus response", err)
	}

	if len(out) != 1 {
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "invalid nexus worker response", nil)
	}

	return h.decodeStartReply(ctx, out[0])
}

// roundTrip encodes msg, executes it on the worker pool, and returns the raw
// reply payload. Failures come back as *nexus.HandlerError; what distinguishes
// the request kind in messages ("nexus request" / "nexus cancel request").
func (h *NexusHandler) roundTrip(ctx context.Context, taskQueue string, msg *internal.Message, what string) (*payload.Payload, error) {
	pl := h.getPld()
	defer h.putPld(pl)

	if err := h.codec.Encode(&internal.Context{TaskQueue: taskQueue}, pl, msg); err != nil {
		// Encoding our own request is a deterministic local bug, not a transient
		// fault — don't ask the server to retry it.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "encode "+what, err)
	}

	ch := make(chan struct{}, 1)
	result, err := h.pool.Exec(ctx, pl, ch)
	if err != nil {
		// Pool returned before queueing — typically pool busy / exec rejected; retryable.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeUnavailable, nexus.HandlerErrorRetryBehaviorRetryable, "exec "+what, err)
	}

	select {
	case pld := <-result:
		if pld.Error() != nil {
			// Worker-side execution failure: retryable per Nexus spec for INTERNAL.
			return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorUnspecified, "nexus worker exec error", pld.Error())
		}
		if pld.Payload().Flags&frame.STREAM != 0 {
			ch <- struct{}{}
			// Streaming worker replies violate the protocol; server-side fault.
			return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "streaming is not supported", nil)
		}
		return pld.Payload(), nil
	default:
		// Pool returned a result channel without a value — should not happen on a
		// healthy pool. Treat as transient.
		return nil, newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorRetryable, "nexus worker empty response", nil)
	}
}

// decodeStartReply maps a PHP→Go reply into the SDK start-result shape.
// Variants: *NexusOperationStarted (sync/async), nil+Failure, nil+nil → HandlerError.
func (h *NexusHandler) decodeStartReply(ctx context.Context, retMsg *internal.Message) (nexus.HandlerStartOperationResult[converter.RawValue], error) {
	switch reply := retMsg.Command.(type) {
	case *internal.NexusOperationStarted:
		forwardNexusLinks(ctx, reply.Links, h.log)
		if reply.Async {
			return &nexus.HandlerStartOperationResultAsync{
				OperationToken: reply.Token,
			}, nil
		}
		var p *commonpb.Payload
		if pls := retMsg.Payloads.GetPayloads(); len(pls) > 0 {
			p = pls[0]
		}
		return &nexus.HandlerStartOperationResultSync[converter.RawValue]{
			Value: converter.NewRawValue(p),
		}, nil
	case nil:
		if retMsg.Failure != nil {
			return nil, nexusErrorFromFailure(retMsg.Failure)
		}
		return nil, newNexusHandlerError(
			nexus.HandlerErrorTypeInternal,
			nexus.HandlerErrorRetryBehaviorNonRetryable,
			"nexus worker reply has neither command nor failure", nil)
	default:
		return nil, newNexusHandlerError(
			nexus.HandlerErrorTypeInternal,
			nexus.HandlerErrorRetryBehaviorNonRetryable,
			fmt.Sprintf("unexpected nexus reply command %T", retMsg.Command), nil)
	}
}

// nexusLinksToInternal converts SDK links to wire form, dropping URL-less entries.
func nexusLinksToInternal(links []nexus.Link) []internal.NexusLink {
	out := make([]internal.NexusLink, 0, len(links))
	for _, l := range links {
		if l.URL == nil {
			continue
		}
		out = append(out, internal.NexusLink{URL: l.URL.String(), Type: l.Type})
	}
	return out
}

// nexusLinksFromInternal: drop entries with empty url/type or unparseable URL.
func nexusLinksFromInternal(links []internal.NexusLink, log *zap.Logger) []nexus.Link {
	if len(links) == 0 {
		return nil
	}
	out := make([]nexus.Link, 0, len(links))
	for _, l := range links {
		if l.URL == "" || l.Type == "" {
			continue
		}
		u, err := url.Parse(l.URL)
		if err != nil {
			log.Warn("nexus link URL is malformed; skipping", zap.String("url", l.URL), zap.Error(err))
			continue
		}
		out = append(out, nexus.Link{URL: u, Type: l.Type})
	}
	return out
}

// forwardNexusLinks ships valid links to handler ctx; bare ctx → warn+drop.
func forwardNexusLinks(ctx context.Context, links []internal.NexusLink, log *zap.Logger) {
	out := nexusLinksFromInternal(links, log)
	if len(out) == 0 {
		return
	}
	if !nexus.IsHandlerContext(ctx) {
		log.Warn("nexus handler ctx missing; response links dropped", zap.Int("links", len(out)))
		return
	}
	nexus.AddHandlerLinks(ctx, out...)
}

// Cause holds the original proto via failureHolder so SDK-Go's ErrorToFailure
// round-trips it back without losing structure (same contract as activity.go).
func nexusErrorFromFailure(f *failurepb.Failure) error {
	cause := temporal.GetDefaultFailureConverter().FailureToError(f)

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
				State:   state,
				Message: f.GetMessage(),
				Cause:   cause,
			}
		}
	}

	return &nexus.HandlerError{
		Type:    nexus.HandlerErrorTypeInternal,
		Message: f.GetMessage(),
		Cause:   cause,
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
			Namespace:      h.namespace,
			TaskQueue:      taskQueue,
			OperationToken: token,
			Headers:        maps.Clone(options.Header),
		},
	}

	r, err := h.roundTrip(ctx, taskQueue, msg, "nexus cancel request")
	if err != nil {
		return err
	}

	return h.decodeCancelReply(r)
}

// decodeCancelReply maps the PHP cancel reply: always exactly one message; a
// rejected cancel carries a Failure (HandlerException on the PHP side), a
// resolved cancel carries none.
func (h *NexusHandler) decodeCancelReply(r *payload.Payload) error {
	out := make([]*internal.Message, 0, 1)
	if err := h.codec.Decode(r, &out); err != nil {
		return newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "decode nexus cancel response", err)
	}

	if len(out) != 1 {
		return newNexusHandlerError(nexus.HandlerErrorTypeInternal, nexus.HandlerErrorRetryBehaviorNonRetryable, "invalid nexus worker cancel response", nil)
	}

	if out[0].Failure != nil {
		return nexusErrorFromFailure(out[0].Failure)
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
	result, err := h.pool.Exec(ctx, pl, ch)
	if err != nil {
		h.log.Warn("nexus cancel method exec failed", zap.Uint64("invocationID", invocationID), zap.Error(err))
		return
	}

	select {
	case pld := <-result:
		if pld != nil && pld.Error() != nil {
			h.log.Warn("nexus method cancel delivery failed", zap.Uint64("invocationID", invocationID), zap.Error(pld.Error()))
		}
	case <-ctx.Done():
		h.log.Warn("nexus method cancel delivery failed", zap.Uint64("invocationID", invocationID), zap.Error(ctx.Err()))
	}
}

func (h *NexusHandler) getPld() *payload.Payload {
	return h.pldPool.Get().(*payload.Payload)
}

func (h *NexusHandler) putPld(pld *payload.Payload) {
	pld.Codec = 0
	pld.Context = nil
	pld.Body = nil
	h.pldPool.Put(pld)
}
