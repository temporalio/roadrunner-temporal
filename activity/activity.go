package activity

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v2/utils"
	tActivity "github.com/spiral/sdk-go/activity"
	temporalClient "github.com/spiral/sdk-go/client"
	"github.com/spiral/sdk-go/converter"
	"github.com/spiral/sdk-go/internalbindings"
	"github.com/spiral/sdk-go/worker"
	"github.com/temporalio/roadrunner-temporal/internal"
	"github.com/temporalio/roadrunner-temporal/internal/codec"
	commonpb "go.temporal.io/api/common/v1"
	"go.uber.org/zap"
)

const doNotCompleteOnReturn = "doNotCompleteOnReturn"

var (
	_ internal.Activity = (*activity)(nil)
)

type activity struct {
	codec   codec.Codec
	client  temporalClient.Client
	log     *zap.Logger
	dc      converter.DataConverter
	seqID   uint64
	running sync.Map

	activities  []string
	tWorkers    []worker.Worker
	graceTimout time.Duration
}

func NewActivityDefinition(ac codec.Codec, log *zap.Logger, dc converter.DataConverter, client temporalClient.Client, gt time.Duration) internal.Activity {
	return &activity{
		log:         log,
		client:      client,
		codec:       ac,
		dc:          dc,
		graceTimout: gt,
		activities:  make([]string, 0, 2),
		tWorkers:    make([]worker.Worker, 0, 2),
	}
}

func (a *activity) Init() ([]worker.Worker, error) {
	const op = errors.Op("activity_pool_init")

	a.log.Debug("start fetching worker info for the activity")
	wi := make([]*internal.WorkerInfo, 0, 2)
	err := a.codec.FetchWorkerInfo(&wi)
	if err != nil {
		return nil, errors.E(op, err)
	}

	if len(wi) == 0 {
		return nil, errors.E(op, errors.Str("no activities in the worker info"))
	}

	for i := 0; i < len(wi); i++ {
		wi[i].Options.WorkerStopTimeout = a.graceTimout
		if wi[i].TaskQueue == "" {
			wi[i].TaskQueue = temporalClient.DefaultNamespace
		}

		if wi[i].Options.Identity == "" {
			wi[i].Options.Identity = fmt.Sprintf(
				"%s:%s",
				wi[i].TaskQueue,
				uuid.NewString(),
			)
		}

		wrk := worker.New(a.client, wi[i].TaskQueue, wi[i].Options)

		for j := 0; j < len(wi[i].Activities); j++ {
			wrk.RegisterActivityWithOptions(a.execute, tActivity.RegisterOptions{
				Name:                          wi[i].Activities[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			a.log.Debug("activity registered", zap.String("name", wi[i].Activities[j].Name))
			a.activities = append(a.activities, wi[i].Activities[j].Name)
		}

		a.tWorkers = append(a.tWorkers, wrk)
	}

	for i := 0; i < len(a.tWorkers); i++ {
		err = a.tWorkers[i].Start()
		if err != nil {
			return nil, errors.E(op, err)
		}
	}

	a.log.Debug("activity workers started", zap.Int("num_workers", len(a.tWorkers)))

	return a.tWorkers, nil
}

func (a *activity) GetActivityContext(taskToken []byte) (context.Context, error) {
	const op = errors.Op("activity_pool_get_activity_context")
	c, ok := a.running.Load(utils.AsString(taskToken))
	if !ok {
		return nil, errors.E(op, errors.Str("heartbeat on non running activity"))
	}

	return c.(context.Context), nil
}

// ActivityNames returns list of all available activity names.
func (a *activity) ActivityNames() []string {
	return a.activities
}

func (a *activity) execute(ctx context.Context, args *commonpb.Payloads) (*commonpb.Payloads, error) {
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

	result, err := a.codec.Execute(&internal.Context{TaskQueue: info.TaskQueue}, msg)
	if err != nil {
		a.running.Delete(utils.AsString(info.TaskToken))
		return nil, errors.E(op, err)
	}

	a.running.Delete(utils.AsString(info.TaskToken))

	if len(result) != 1 {
		return nil, errors.E(op, errors.Str("invalid activity worker response"))
	}

	out := result[0]
	if out.Failure != nil {
		if out.Failure.Message == doNotCompleteOnReturn {
			return nil, tActivity.ErrResultPending
		}

		return nil, internalbindings.ConvertFailureToError(out.Failure, a.dc)
	}

	return out.Payloads, nil
}
