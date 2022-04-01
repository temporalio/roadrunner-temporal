package aggregatedpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/api/v2/payload"
	"github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/canceller"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/queue"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/registry"
	"github.com/temporalio/roadrunner-temporal/internal"
	commonpb "go.temporal.io/api/common/v1"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.uber.org/zap"
)

// implements WorkflowDefinition interface
// keep in sync with the temporal.io/sdk-go/internal/internal_worker_base.go:111
/*
	// WorkflowDefinition wraps the code that can execute a workflow.
	WorkflowDefinition interface {
		Execute(env WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads)
		OnWorkflowTaskStarted(deadlockDetectionTimeout time.Duration)
		StackTrace() string
		Close()
	}
*/

type Callback func() error

type Workflow struct {
	codec  Codec
	pool   pool.Pool
	client temporalClient.Client

	env       bindings.WorkflowEnvironment
	header    *commonpb.Header
	mq        *queue.MessageQueue
	ids       *registry.IDRegistry
	seqID     uint64
	sID       func() uint64
	runID     string
	pipeline  []*internal.Message
	callbacks []Callback
	canceller *canceller.Canceller
	inLoop    uint32

	dc converter.DataConverter

	log          *zap.Logger
	graceTimeout time.Duration
	mh           temporalClient.MetricsHandler
}

func NewWorkflowDefinition(codec Codec, dc converter.DataConverter, pool pool.Pool, log *zap.Logger, seqID func() uint64, client temporalClient.Client, gt time.Duration) *Workflow {
	return &Workflow{
		client:       client,
		log:          log,
		sID:          seqID,
		codec:        codec,
		graceTimeout: gt,
		dc:           dc,
		pool:         pool,
	}
}

// NewWorkflowDefinition ... Workflow should match the WorkflowDefinitionFactory interface (sdk-go/internal/internal_worker.go:463, RegisterWorkflowWithOptions func)
// DO NOT USE THIS FUNCTION DIRECTLY!!!!
func (wp *Workflow) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &Workflow{
		pool:  wp.pool,
		codec: wp.codec,
		log:   wp.log,
		sID:   wp.sID,
	}
}

// Execute implementation must be asynchronous.
func (wp *Workflow) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.log.Debug("workflow execute", zap.String("runID", env.WorkflowInfo().WorkflowExecution.RunID), zap.Any("workflow info", env.WorkflowInfo()))

	wp.mh = env.GetMetricsHandler()
	wp.env = env
	wp.header = header
	wp.seqID = 0
	wp.runID = env.WorkflowInfo().WorkflowExecution.RunID
	wp.canceller = new(canceller.Canceller)

	// sequenceID shared for all pool workflows
	wp.mq = queue.NewMessageQueue(wp.sID)
	wp.ids = new(registry.IDRegistry)

	env.RegisterCancelHandler(wp.handleCancel)
	env.RegisterSignalHandler(wp.handleSignal)
	env.RegisterQueryHandler(wp.handleQuery)

	var lastCompletion = bindings.GetLastCompletionResult(env)
	var lastCompletionOffset = 0

	if lastCompletion != nil && len(lastCompletion.Payloads) != 0 {
		if input == nil {
			input = &commonpb.Payloads{Payloads: []*commonpb.Payload{}}
		}

		input.Payloads = append(input.Payloads, lastCompletion.Payloads...)
		lastCompletionOffset = len(lastCompletion.Payloads)
	}

	_ = wp.mq.PushCommand(
		internal.StartWorkflow{
			Info:           env.WorkflowInfo(),
			LastCompletion: lastCompletionOffset,
		},
		input,
		wp.header,
	)
}

// OnWorkflowTaskStarted is called for each non timed out startWorkflowTask event.
// Executed after all history events since the previous commands are applied to WorkflowDefinition
// Application level code must be executed from this function only.
// Execute call as well as callbacks called from WorkflowEnvironment functions can only schedule callbacks
// which can be executed from OnWorkflowTaskStarted().
// FROM THE TEMPORAL DESCRIPTION
func (wp *Workflow) OnWorkflowTaskStarted(t time.Duration) {
	atomic.StoreUint32(&wp.inLoop, 1)
	defer func() {
		atomic.StoreUint32(&wp.inLoop, 0)
	}()

	wp.log.Debug("workflow task started", zap.Duration("time", t))

	var err error
	// do not copy
	for k := range wp.callbacks {
		err = wp.callbacks[k]()
		if err != nil {
			panic(err)
		}
	}

	wp.callbacks = nil

	err = wp.flushQueue()
	if err != nil {
		panic(err)
	}

	for len(wp.pipeline) > 0 {
		msg := wp.pipeline[0]
		wp.pipeline = wp.pipeline[1:]

		if msg.IsCommand() {
			err = wp.handleMessage(msg)
		}

		if err != nil {
			panic(err)
		}
	}
}

// StackTrace of all coroutines owned by the Dispatcher instance.
func (wp *Workflow) StackTrace() string {
	result, err := wp.runCommand(
		internal.GetStackTrace{
			RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID,
		},
		nil,
		wp.header,
	)

	if err != nil {
		return err.Error()
	}

	var stacktrace string
	if len(result.Payloads.GetPayloads()) == 0 {
		return ""
	}

	err = wp.env.GetDataConverter().FromPayload(result.Payloads.Payloads[0], &stacktrace)
	if err != nil {
		return err.Error()
	}

	return stacktrace
}

func (wp *Workflow) Close() {
	// send destroy command
	_, _ = wp.runCommand(internal.DestroyWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID}, nil, wp.header)
	// flush queue
	wp.mq.Flush()
}

func (wp *Workflow) execute(ctx context.Context, args *commonpb.Payloads) (*commonpb.Payloads, error) {
	const op = errors.Op("activity_pool_execute_activity")

	var info = tActivity.GetInfo(ctx)
	info.TaskToken = []byte(uuid.NewString())
	mh := tActivity.GetMetricsHandler(ctx)
	// if the mh is not nil, record the RR metric
	if mh != nil {
		mh.Gauge(RrMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
		defer mh.Gauge(RrMetricName).Update(float64(wp.pool.(pool.Queuer).QueueSize()))
	}

	var msg = &internal.Message{
		ID: atomic.AddUint64(&wp.seqID, 1),
		Command: internal.InvokeLocalActivity{
			Name: info.ActivityType.Name,
			Info: info,
		},
		Payloads: args,
	}

	pld := &payload.Payload{}
	err := wp.codec.Encode(&internal.Context{TaskQueue: info.TaskQueue}, pld, msg)
	if err != nil {
		return nil, err
	}

	result, err := wp.pool.Exec(pld)
	if err != nil {
		return nil, errors.E(op, err)
	}

	out := make([]*internal.Message, 0, 2)
	err = wp.codec.Decode(result, &out)
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

		return nil, bindings.ConvertFailureToError(retPld.Failure, wp.dc)
	}

	return retPld.Payloads, nil
}
