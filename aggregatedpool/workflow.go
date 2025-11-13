package aggregatedpool

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/pool/payload"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/canceller"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	"github.com/temporalio/roadrunner-temporal/v5/queue"
	"github.com/temporalio/roadrunner-temporal/v5/registry"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
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
type LaFn func(ctx context.Context, hdr *commonpb.Header, args *commonpb.Payloads) (*commonpb.Payloads, error)

// seqID is global sequence ID
var seqID uint64 //nolint:gochecknoglobals

func seq() uint64 {
	return atomic.AddUint64(&seqID, 1)
}

type Workflow struct {
	codec api.Codec
	pool  api.Pool
	rrID  string

	// LocalActivityFn
	la LaFn

	env          bindings.WorkflowEnvironment
	header       *commonpb.Header
	mq           *queue.MessageQueue
	ids          *registry.IDRegistry
	seqID        uint64
	pipeline     []*internal.Message
	updatesQueue map[string]struct{}
	callbacks    []Callback
	canceller    *canceller.Canceller
	inLoop       uint32

	// updates
	updateCompleteCb map[string]func(res *internal.Message)
	updateValidateCb map[string]func(res *internal.Message)

	log *zap.Logger
	mh  temporalClient.MetricsHandler

	// objects pool
	pldPool *sync.Pool
}

// NewWorkflowDefinition ... WorkflowDefinition Constructor
func NewWorkflowDefinition(codec api.Codec, la LaFn, pool api.Pool, log *zap.Logger) *Workflow {
	return &Workflow{
		rrID:  uuid.NewString(),
		log:   log,
		la:    la,
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
// This function called after the constructor above, it is safe to assign fields like that
func (wp *Workflow) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &Workflow{
		rrID: uuid.NewString(),
		// LocalActivity
		la: wp.la,
		// updates logic
		updateCompleteCb: make(map[string]func(res *internal.Message)),
		updateValidateCb: make(map[string]func(res *internal.Message)),
		updatesQueue:     map[string]struct{}{},
		// -- updates
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
	wp.canceller = new(canceller.Canceller)

	// sequenceID shared for all pool workflows
	wp.mq = queue.NewMessageQueue(seq)
	wp.ids = new(registry.IDRegistry)

	env.RegisterCancelHandler(wp.handleCancel)
	env.RegisterSignalHandler(wp.handleSignal)
	env.RegisterQueryHandler(wp.handleQuery)
	env.RegisterUpdateHandler(wp.handleUpdate)

	// check if we have some TSA
	tsa := env.TypedSearchAttributes()
	// start workflow command
	stwfcmd := internal.StartWorkflow{
		Info: env.WorkflowInfo(),
	}

	// search attributes types are:
	/*
		INDEXED_VALUE_TYPE_TEXT         IndexedValueType = 1
		INDEXED_VALUE_TYPE_KEYWORD      IndexedValueType = 2
		INDEXED_VALUE_TYPE_INT          IndexedValueType = 3
		INDEXED_VALUE_TYPE_DOUBLE       IndexedValueType = 4
		INDEXED_VALUE_TYPE_BOOL         IndexedValueType = 5
		INDEXED_VALUE_TYPE_DATETIME     IndexedValueType = 6
		INDEXED_VALUE_TYPE_KEYWORD_LIST IndexedValueType = 7

	*/
	// only process if there're values, obviously
	if tsa.Size() > 0 {
		untuped := tsa.GetUntypedValues()
		tsaParsed := make(map[string]*internal.TypedSearchAttribute, tsa.Size())
		for k, v := range untuped {
			vt := k.GetValueType()
			switch vt {
			// just for the linters, should be never reached
			case enumspb.INDEXED_VALUE_TYPE_UNSPECIFIED:
				continue
			case enumspb.INDEXED_VALUE_TYPE_TEXT:
				str, ok := v.(string)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a string", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.StringType,
					Value: str,
				}
			case enumspb.INDEXED_VALUE_TYPE_KEYWORD:
				str, ok := v.(string)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a string[keyword]", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.KeywordType,
					Value: str,
				}
			case enumspb.INDEXED_VALUE_TYPE_INT:
				switch tt := v.(type) {
				case int:
					tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
						Type:  internal.IntType,
						Value: tt,
					}
				case int64:
					tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
						Type:  internal.IntType,
						Value: tt,
					}
				case string:
					res, err := strconv.Atoi(tt)
					if err != nil {
						wp.log.Warn("typed search attribute found, but it is not an int", zap.Error(err), zap.String("key", k.GetName()))
						continue
					}
					tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
						Type:  internal.IntType,
						Value: res,
					}
				default:
					wp.log.Warn("typed search attribute found, but it is not an int", zap.String("key", k.GetName()))
					continue
				}
			case enumspb.INDEXED_VALUE_TYPE_DOUBLE:
				str, ok := v.(float64)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a float64", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.FloatType,
					Value: str,
				}
			case enumspb.INDEXED_VALUE_TYPE_BOOL:
				str, ok := v.(bool)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a bool", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.BoolType,
					Value: str,
				}
			case enumspb.INDEXED_VALUE_TYPE_DATETIME:
				str, ok := v.(time.Time)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a datetime", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.DatetimeType,
					Value: str.Format(time.RFC3339),
				}
			case enumspb.INDEXED_VALUE_TYPE_KEYWORD_LIST:
				str, ok := v.([]string)
				if !ok {
					wp.log.Warn("typed search attribute found, but it is not a []string", zap.String("key", k.GetName()))
					continue
				}
				tsaParsed[k.GetName()] = &internal.TypedSearchAttribute{
					Type:  internal.KeywordListType,
					Value: str,
				}
			}
		}

		// set typed search attributes
		stwfcmd.SearchAttributes = tsaParsed
	}

	var lastCompletion = bindings.GetLastCompletionResult(env)
	if lastCompletion != nil && len(lastCompletion.Payloads) != 0 {
		if input == nil {
			input = &commonpb.Payloads{Payloads: []*commonpb.Payload{}}
		}

		input.Payloads = append(input.Payloads, lastCompletion.Payloads...)
		stwfcmd.LastCompletion = len(lastCompletion.Payloads)
	}

	wp.mq.PushCommand(
		stwfcmd,
		input,
		wp.header,
		wp.getWorkflowWorkerPid(),
	)
}

// OnWorkflowTaskStarted is called for each non-timed out startWorkflowTask event.
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
		for k := range wp.updatesQueue {
			wp.env.HandleQueuedUpdates(k)
			delete(wp.updatesQueue, k)
		}
	}
	// clean
	wp.updatesQueue = map[string]struct{}{}

	// at first, we should flush our queue with command, e.g.: startWorkflow
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
	// when closing the workflow, we should drain(execute) unhandled updates
	if wp.env.DrainUnhandledUpdates() {
		wp.log.Info("drained unhandled updates")
	}

	// clean the map
	for k := range wp.updatesQueue {
		delete(wp.updatesQueue, k)
	}

	for k := range wp.updateCompleteCb {
		delete(wp.updateCompleteCb, k)
	}

	// send destroy command
	_, _ = wp.runCommand(internal.DestroyWorkflow{RunID: wp.env.WorkflowInfo().WorkflowExecution.RunID}, nil, wp.header)
	// flush queue
	wp.mq.Flush()
}
