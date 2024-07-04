package aggregatedpool

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/goridge/v3/pkg/frame"
	"github.com/roadrunner-server/pool/payload"
	"github.com/temporalio/roadrunner-temporal/v5/common"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	commonpb "go.temporal.io/api/common/v1"
	tActivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.uber.org/zap"
)

type LocalActivityFn struct {
	codec common.Codec
	pool  common.Pool
	log   *zap.Logger
	seqID uint64
}

func NewLocalActivityFn(codec common.Codec, pool common.Pool, log *zap.Logger) *LocalActivityFn {
	return &LocalActivityFn{
		codec: codec,
		pool:  pool,
		log:   log,
	}
}

func (la *LocalActivityFn) ExecuteLA(ctx context.Context, hdr *commonpb.Header, args *commonpb.Payloads) (*commonpb.Payloads, error) {
	const op = errors.Op("activity_pool_execute_activity")

	var info = tActivity.GetInfo(ctx)
	info.TaskToken = []byte(uuid.NewString())
	mh := tActivity.GetMetricsHandler(ctx)
	// if the mh is not nil, record the RR metric
	if mh != nil {
		mh.Gauge(RrMetricName).Update(float64(la.pool.QueueSize()))
		defer mh.Gauge(RrMetricName).Update(float64(la.pool.QueueSize()))
	}

	var msg = &internal.Message{
		ID: atomic.AddUint64(&la.seqID, 1),
		Command: internal.InvokeLocalActivity{
			Name: info.ActivityType.Name,
			Info: info,
		},
		Payloads: args,
		Header:   hdr,
	}

	la.log.Debug("executing local activity fn", zap.Uint64("ID", msg.ID), zap.String("task-queue", info.TaskQueue), zap.String("la ID", info.ActivityID))

	pl := getPld()
	defer putPld(pl)

	err := la.codec.Encode(&internal.Context{TaskQueue: info.TaskQueue}, pl, msg)
	if err != nil {
		return nil, err
	}

	ch := make(chan struct{}, 1)
	result, err := la.pool.Exec(ctx, pl, ch)
	if err != nil {
		return nil, errors.E(op, err)
	}

	var r *payload.Payload
	select {
	case pld := <-result:
		if pld.Error() != nil {
			return nil, errors.E(op, pld.Error())
		}
		// streaming is not supported
		if pld.Payload().Flags&frame.STREAM != 0 {
			ch <- struct{}{}
			return nil, errors.E(op, errors.Str("streaming is not supported"))
		}

		// assign the payload
		r = pld.Payload()
	default:
		return nil, errors.E(op, errors.Str("worker empty response"))
	}

	out := make([]*internal.Message, 0, 2)
	err = la.codec.Decode(r, &out)
	if err != nil {
		return nil, err
	}

	if len(out) != 1 {
		return nil, errors.E(op, errors.Str("invalid local activity worker response"))
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

var pldP = sync.Pool{ //nolint:gochecknoglobals
	New: func() any {
		return &payload.Payload{}
	},
}

func getPld() *payload.Payload {
	return pldP.Get().(*payload.Payload)
}

func putPld(pld *payload.Payload) {
	pld.Codec = 0
	pld.Context = nil
	pld.Body = nil
	pldP.Put(pld)
}
