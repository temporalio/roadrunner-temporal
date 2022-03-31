package aggregatedpool

import (
	"sync/atomic"
	"time"

	"github.com/roadrunner-server/api/v2/pool"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/canceller"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/queue"
	"github.com/temporalio/roadrunner-temporal/aggregatedpool/registry"
	"github.com/temporalio/roadrunner-temporal/internal"
	commonpb "go.temporal.io/api/common/v1"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
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
	actDef    *Activity

	dc        converter.DataConverter
	workflows map[string]internal.WorkflowInfo
	tWorkers  []worker.Worker

	log          *zap.Logger
	graceTimeout time.Duration
	mh           temporalClient.MetricsHandler
}

func NewWorkflowDefinition(codec Codec, actDef *Activity, dc converter.DataConverter, pool pool.Pool, log *zap.Logger, seqID func() uint64, client temporalClient.Client, gt time.Duration) *Workflow {
	return &Workflow{
		client:       client,
		log:          log,
		sID:          seqID,
		codec:        codec,
		graceTimeout: gt,
		dc:           dc,
		pool:         pool,
		actDef:       actDef,

		workflows: make(map[string]internal.WorkflowInfo),
		tWorkers:  make([]worker.Worker, 0, 2),
	}
}

// NewWorkflowDefinition ... Workflow should match the WorkflowDefinitionFactory interface (sdk-go/internal/internal_worker.go:463, RegisterWorkflowWithOptions func)
// DO NOT USE THIS FUNCTION DIRECTLY!!!!
func (wp *Workflow) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &Workflow{
		actDef: wp.actDef,
		pool:   wp.pool,
		codec:  wp.codec,
		log:    wp.log,
		sID:    wp.sID,
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
	wp.canceller = &canceller.Canceller{}

	// sequenceID shared for all pool workflows
	wp.mq = queue.NewMessageQueue(wp.sID)
	wp.ids = &registry.IDRegistry{}

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
			err = wp.handleMessage(msg, wp.actDef, wp.dc)
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

func (wp *Workflow) WorkflowNames() []string {
	names := make([]string, 0, len(wp.workflows))
	for name := range wp.workflows {
		names = append(names, name)
	}

	return names
}
