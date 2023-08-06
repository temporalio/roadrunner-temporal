package aggregatedpool

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v4/payload"
	"github.com/roadrunner-server/sdk/v4/utils"
	"github.com/temporalio/roadrunner-temporal/v4/common"
	"github.com/temporalio/roadrunner-temporal/v4/internal"
	commonpb "go.temporal.io/api/common/v1"
	tActivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.uber.org/zap"
)

const (
	doNotCompleteOnReturn        = "doNotCompleteOnReturn"
	RrMetricName          string = "rr_activities_pool_queue_size"
	RrWorkflowsMetricName string = "rr_workflows_pool_queue_size"
)

type Activity struct {
	codec   common.Codec
	pool    common.Pool
	log     *zap.Logger
	seqID   uint64
	running sync.Map

	pldPool *sync.Pool
}

func NewActivityDefinition(ac common.Codec, p common.Pool, log *zap.Logger) *Activity {
	return &Activity{
		log:   log,
		codec: ac,
		pool:  p,
		pldPool: &sync.Pool{
			New: func() any {
				return new(payload.Payload)
			},
		},
	}
}

func (a *Activity) GetActivityContext(taskToken []byte) (context.Context, error) {
	const op = errors.Op("activity_pool_get_activity_context")
	c, ok := a.running.Load(utils.AsString(taskToken))
	if !ok {
		return nil, errors.E(op, errors.Str("heartbeat on non running activity"))
	}

	return c.(context.Context), nil
}

func (a *Activity) execute(ctx context.Context, args *commonpb.Payloads) (*commonpb.Payloads, error) {
	const op = errors.Op("activity_pool_execute_activity")

	heartbeatDetails := &commonpb.Payloads{}
	if tActivity.HasHeartbeatDetails(ctx) {
		err := tActivity.GetHeartbeatDetails(ctx, &heartbeatDetails)
		if err != nil {
			return nil, errors.E(op, err)
		}
	}

	var info = tActivity.GetInfo(ctx)
	a.running.Store(utils.AsString(info.TaskToken), ctx)
	mh := tActivity.GetMetricsHandler(ctx)
	// if the mh is not nil, record the RR metric
	if mh != nil {
		mh.Gauge(RrMetricName).Update(float64(a.pool.QueueSize()))
		defer mh.Gauge(RrMetricName).Update(float64(a.pool.QueueSize()))
	}

	var msg = &internal.Message{
		ID: atomic.AddUint64(&a.seqID, 1),
		Command: internal.InvokeActivity{
			Name:             info.ActivityType.Name,
			Info:             info,
			HeartbeatDetails: len(heartbeatDetails.Payloads),
		},
		Payloads: args,
		Header:   common.ActivityHeadersFromCtx(ctx),
	}

	if len(heartbeatDetails.Payloads) != 0 {
		msg.Payloads.Payloads = append(msg.Payloads.Payloads, heartbeatDetails.Payloads...)
	}

	pld := a.getPld()
	defer a.putPld(pld)

	err := a.codec.Encode(&internal.Context{TaskQueue: info.TaskQueue}, pld, msg)
	if err != nil {
		return nil, err
	}

	result, err := a.pool.Exec(ctx, pld, nil)
	if err != nil {
		a.running.Delete(utils.AsString(info.TaskToken))
		return nil, errors.E(op, err)
	}

	a.running.Delete(utils.AsString(info.TaskToken))

	var r *payload.Payload

	select {
	case pld := <-result:
		if pld.Error() != nil {
			return nil, errors.E(op, pld.Error())
		}
		// streaming is not supported
		if pld.Payload().IsStream {
			return nil, errors.E(op, errors.Str("streaming is not supported"))
		}

		// assign the payload
		r = pld.Payload()
	default:
		return nil, errors.E(op, errors.Str("activity worker empty response"))
	}

	out := make([]*internal.Message, 0, 2)
	err = a.codec.Decode(r, &out)
	if err != nil {
		return nil, err
	}

	if len(out) != 1 {
		return nil, errors.E(op, errors.Str("invalid activity worker response"))
	}

	retPld := out[0]
	if retPld.Failure != nil {
		if retPld.Failure.Message == doNotCompleteOnReturn {
			return nil, tActivity.ErrResultPending
		}

		return nil, temporal.GetDefaultFailureConverter().FailureToError(retPld.Failure)
	}

	return retPld.Payloads, nil
}

func (a *Activity) getPld() *payload.Payload {
	return a.pldPool.Get().(*payload.Payload)
}

func (a *Activity) putPld(pld *payload.Payload) {
	pld.Codec = 0
	pld.Context = nil
	pld.Body = nil
	a.pldPool.Put(pld)
}
