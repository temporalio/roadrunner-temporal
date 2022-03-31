package aggregatedpool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/api/v2/payload"
	"github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/utils"
	"github.com/temporalio/roadrunner-temporal/internal"
	commonpb "go.temporal.io/api/common/v1"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
)

const (
	doNotCompleteOnReturn        = "doNotCompleteOnReturn"
	RrMetricName          string = "rr_activities_pool_queue_size"
	RrWorkflowsMetricName string = "rr_workflows_pool_queue_size"
)

type Activity struct {
	codec   Codec
	pool    pool.Pool
	client  temporalClient.Client
	log     *zap.Logger
	dc      converter.DataConverter
	seqID   uint64
	running sync.Map

	activities  []string
	tWorkers    []worker.Worker
	graceTimout time.Duration
}

func NewActivityDefinition(ac Codec, p pool.Pool, log *zap.Logger, dc converter.DataConverter, client temporalClient.Client, gt time.Duration) *Activity {
	return &Activity{
		log:         log,
		client:      client,
		codec:       ac,
		pool:        p,
		dc:          dc,
		graceTimout: gt,
		activities:  make([]string, 0, 2),
		tWorkers:    make([]worker.Worker, 0, 2),
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

func (a *Activity) ExecuteA(ctx context.Context, args *commonpb.Payloads) (*commonpb.Payloads, error) {
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
		mh.Gauge(RrMetricName).Update(float64(a.pool.(pool.Queuer).QueueSize()))
		defer mh.Gauge(RrMetricName).Update(float64(a.pool.(pool.Queuer).QueueSize()))
	}

	var msg = &internal.Message{
		ID: atomic.AddUint64(&a.seqID, 1),
		Command: internal.InvokeActivity{
			Name:             info.ActivityType.Name,
			Info:             info,
			HeartbeatDetails: len(heartbeatDetails.Payloads),
		},
		Payloads: args,
	}

	if len(heartbeatDetails.Payloads) != 0 {
		msg.Payloads.Payloads = append(msg.Payloads.Payloads, heartbeatDetails.Payloads...)
	}

	pld := &payload.Payload{}
	err := a.codec.Encode(&internal.Context{TaskQueue: info.TaskQueue}, pld, msg)
	if err != nil {
		return nil, err
	}

	result, err := a.pool.Exec(pld)
	if err != nil {
		a.running.Delete(utils.AsString(info.TaskToken))
		return nil, errors.E(op, err)
	}

	a.running.Delete(utils.AsString(info.TaskToken))

	out := make([]*internal.Message, 0, 2)
	err = a.codec.Decode(result, &out)
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

		return nil, internalbindings.ConvertFailureToError(retPld.Failure, a.dc)
	}

	return retPld.Payloads, nil
}

func (a *Activity) ExecuteLA(ctx context.Context, args *commonpb.Payloads) (*commonpb.Payloads, error) {
	const op = errors.Op("activity_pool_execute_activity")

	var info = tActivity.GetInfo(ctx)
	info.TaskToken = []byte(uuid.NewString())
	a.running.Store(utils.AsString(info.TaskToken), ctx)
	mh := tActivity.GetMetricsHandler(ctx)
	// if the mh is not nil, record the RR metric
	if mh != nil {
		mh.Gauge(RrMetricName).Update(float64(a.pool.(pool.Queuer).QueueSize()))
		defer mh.Gauge(RrMetricName).Update(float64(a.pool.(pool.Queuer).QueueSize()))
	}

	var msg = &internal.Message{
		ID: atomic.AddUint64(&a.seqID, 1),
		Command: internal.InvokeLocalActivity{
			Name: info.ActivityType.Name,
			Info: info,
		},
		Payloads: args,
	}

	pld := &payload.Payload{}
	err := a.codec.Encode(&internal.Context{TaskQueue: info.TaskQueue}, pld, msg)
	if err != nil {
		return nil, err
	}

	result, err := a.pool.Exec(pld)
	if err != nil {
		a.running.Delete(utils.AsString(info.TaskToken))
		return nil, errors.E(op, err)
	}

	a.running.Delete(utils.AsString(info.TaskToken))

	out := make([]*internal.Message, 0, 2)
	err = a.codec.Decode(result, &out)
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

		return nil, internalbindings.ConvertFailureToError(retPld.Failure, a.dc)
	}

	return retPld.Payloads, nil
}
