package aggregatedpool

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/sdk/v4/payload"
	"github.com/temporalio/roadrunner-temporal/v4/canceller"
	"github.com/temporalio/roadrunner-temporal/v4/common"
	"github.com/temporalio/roadrunner-temporal/v4/internal"
	"github.com/temporalio/roadrunner-temporal/v4/queue"
	"github.com/temporalio/roadrunner-temporal/v4/registry"
	commonpb "go.temporal.io/api/common/v1"
	temporalClient "go.temporal.io/sdk/client"
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

// seqID is global sequence ID
var seqID uint64 //nolint:gochecknoglobals

func seq() uint64 {
	return atomic.AddUint64(&seqID, 1)
}

type Workflow struct {
	codec common.Codec
	pool  common.Pool
	rrID  string

	env          bindings.WorkflowEnvironment
	header       *commonpb.Header
	mq           *queue.MessageQueue
	ids          *registry.IDRegistry
	seqID        uint64
	pipeline     []*internal.Message
	updatesQueue []string
	callbacks    []Callback
	canceller    *canceller.Canceller
	inLoop       uint32

	log *zap.Logger
	mh  temporalClient.MetricsHandler

	// objects pool
	pldPool *sync.Pool
}

func NewWorkflowDefinition(codec common.Codec, pool common.Pool, log *zap.Logger) *Workflow {
	return &Workflow{
		rrID:  uuid.NewString(),
		log:   log,
		codec: codec,
		pool:  pool,
		pldPool: &sync.Pool{
			New: func() any {
				return new(payload.Payload)
			},
		},
	}
}

// NewWorkflowDefinition ... Workflow should match the WorkflowDefinitionFactory interface (sdk-go/internal/internal_worker.go:463, RegisterWorkflowWithOptions func)
// DO NOT USE THIS FUNCTION DIRECTLY!!!!
func (wp *Workflow) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &Workflow{
		rrID:  uuid.NewString(),
		pool:  wp.pool,
		codec: wp.codec,
		log:   wp.log,
		pldPool: &sync.Pool{
			New: func() any {
				return new(payload.Payload)
			},
		},
	}
}

// Execute implementation must be asynchronous.
func (wp *Workflow) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.log.Debug("workflow execute", zap.String("runID", env.WorkflowInfo().WorkflowExecution.RunID), zap.Any("workflow info", env.WorkflowInfo()))

	wp.mh = env.GetMetricsHandler()
	wp.env = env
	wp.header = header
	wp.seqID = 0
	wp.updatesQueue = make([]string, 0, 1)
	wp.canceller = new(canceller.Canceller)

	// sequenceID shared for all pool workflows
	wp.mq = queue.NewMessageQueue(seq)
	wp.ids = new(registry.IDRegistry)

	env.RegisterCancelHandler(wp.handleCancel)
	env.RegisterSignalHandler(wp.handleSignal)
	env.RegisterQueryHandler(wp.handleQuery)
	env.RegisterUpdateHandler(wp.handleUpdate)

	var lastCompletion = bindings.GetLastCompletionResult(env)
	var lastCompletionOffset = 0

	if lastCompletion != nil && len(lastCompletion.Payloads) != 0 {
		if input == nil {
			input = &commonpb.Payloads{Payloads: []*commonpb.Payload{}}
		}

		input.Payloads = append(input.Payloads, lastCompletion.Payloads...)
		lastCompletionOffset = len(lastCompletion.Payloads)
	}

	wp.mq.PushCommand(
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
	for i := 0; i < len(wp.callbacks); i++ {
		err = wp.callbacks[i]()
		if err != nil {
			panic(err)
		}
	}

	wp.callbacks = nil

	// handle updates
	if len(wp.updatesQueue) > 0 {
		for i := 0; i < len(wp.updatesQueue); i++ {
			wp.env.HandleQueuedUpdates(wp.updatesQueue[i])
		}
	}
	// clean
	wp.updatesQueue = make([]string, 0, 1)

	err = wp.flushQueue()
	if err != nil {
		panic(err)
	}

	for len(wp.pipeline) > 0 {
		msg := wp.pipeline[0]
		wp.pipeline = wp.pipeline[1:]

		if msg.IsCommand() {
			if msg.UndefinedResponse() {
				wp.pipeline = nil
				panic(fmt.Sprintf("undefined response: %s", msg.Command.(*internal.UndefinedResponse).Message))
			}

			err = wp.handleMessage(msg)
		}

		if err != nil {
			wp.pipeline = nil
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
	wp.log.Debug("close workflow", zap.String("RunID", wp.env.WorkflowInfo().WorkflowExecution.RunID))
	if wp.env.DrainUnhandledUpdates() {
		wp.log.Info("drained unhandled updates")
	}
	// send destroy command
	_, _ = wp.runCommand(internal.DestroyWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID}, nil, wp.header)
	// flush queue
	wp.mq.Flush()
}
