package workflow

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/internal"
	"github.com/temporalio/roadrunner-temporal/internal/codec"
	"github.com/temporalio/roadrunner-temporal/workflow/canceller"
	"github.com/temporalio/roadrunner-temporal/workflow/queue"
	"github.com/temporalio/roadrunner-temporal/workflow/registry"
	commonpb "go.temporal.io/api/common/v1"
	temporalClient "go.temporal.io/sdk/client"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
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
type process struct {
	codec  codec.Codec
	client temporalClient.Client

	env       bindings.WorkflowEnvironment
	header    *commonpb.Header
	mq        *queue.MessageQueue
	ids       *registry.IDRegistry
	seqID     uint64
	sID       func() uint64
	runID     string
	pipeline  []*internal.Message
	callbacks []func() error
	canceller *canceller.Canceller
	inLoop    uint32

	workflows map[string]internal.WorkflowInfo
	tWorkers  []worker.Worker

	log          *zap.Logger
	graceTimeout time.Duration
}

func NewWorkflowDefinition(codec codec.Codec, log *zap.Logger, seqID func() uint64, client temporalClient.Client, gt time.Duration) internal.Workflow {
	return &process{
		client:       client,
		log:          log,
		sID:          seqID,
		codec:        codec,
		graceTimeout: gt,

		workflows: make(map[string]internal.WorkflowInfo),
		tWorkers:  make([]worker.Worker, 0, 2),
	}
}

// NewWorkflowDefinition ... process should match the WorkflowDefinitionFactory interface (sdk-go/internal/internal_worker.go:463, RegisterWorkflowWithOptions func)
// DO NOT USE THIS FUNCTION DIRECTLY!!!!
func (wp *process) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &process{
		codec: wp.codec,
		log:   wp.log,
		sID:   wp.sID,
	}
}

// Execute implementation must be asynchronous.
func (wp *process) Execute(env bindings.WorkflowEnvironment, header *commonpb.Header, input *commonpb.Payloads) {
	wp.log.Debug("workflow execute", zap.String("runID", env.WorkflowInfo().WorkflowExecution.RunID), zap.Any("workflow info", env.WorkflowInfo()))

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
func (wp *process) OnWorkflowTaskStarted(t time.Duration) {
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
func (wp *process) StackTrace() string {
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

func (wp *process) Close() {
	// send destroy command
	_, _ = wp.runCommand(internal.DestroyWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID}, nil, wp.header)
	// flush queue
	wp.mq.Flush()
}

func (wp *process) Init() ([]worker.Worker, error) {
	const op = errors.Op("workflow_definition_init")

	wi := make([]*internal.WorkerInfo, 0, 2)
	err := wp.codec.FetchWorkerInfo(&wi)
	if err != nil {
		return nil, errors.E(op, err)
	}

	for i := 0; i < len(wi); i++ {
		wp.log.Debug("worker info", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("options", wi[i].Options))

		wi[i].Options.WorkerStopTimeout = wp.graceTimeout

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

		wrk := worker.New(wp.client, wi[i].TaskQueue, wi[i].Options)

		for j := 0; j < len(wi[i].Workflows); j++ {
			wp.log.Debug("register workflow with options", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("workflow name", wi[i].Workflows[j].Name))

			wrk.RegisterWorkflowWithOptions(wp, workflow.RegisterOptions{
				Name:                          wi[i].Workflows[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			wp.workflows[wi[i].Workflows[j].Name] = wi[i].Workflows[j]
		}

		wp.tWorkers = append(wp.tWorkers, wrk)
	}

	return wp.tWorkers, nil
}

func (wp *process) WorkflowNames() []string {
	names := make([]string, 0, len(wp.workflows))
	for name := range wp.workflows {
		names = append(names, name)
	}

	return names
}
