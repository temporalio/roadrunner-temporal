package workflow

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/pkg/events"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	rrPool "github.com/spiral/roadrunner/v2/pkg/pool"
	rrWorker "github.com/spiral/roadrunner/v2/pkg/worker"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/server"
	roadrunner_temporal "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/client"
	rrt "github.com/temporalio/roadrunner-temporal/protocol"
	bindings "go.temporal.io/sdk/internalbindings"
	tWorker "go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// RR_MODE env variable key
const RR_MODE = "RR_MODE" //nolint:revive,stylecheck

// RR_CODEC env variable key
const RR_CODEC = "RR_CODEC" //nolint:revive,stylecheck

// Workflow pool
type pool interface {
	SeqID() uint64
	Exec(p payload.Payload) (payload.Payload, error)
	Start(ctx context.Context, temporal client.Temporal) error
	Destroy(ctx context.Context) error
	Workers() []rrWorker.BaseProcess
	Pool() rrPool.Pool
	WorkflowNames() []string
}

// workerImpl manages workflowProcess executions between pool restarts.
type workerImpl struct {
	sync.Mutex
	codec     rrt.Codec
	seqID     uint64
	workflows map[string]rrt.WorkflowInfo
	tWorkers  []tWorker.Worker
	pool      rrPool.Pool
	//
	// logger
	//
	log logger.Logger
	//
	// graceful stop timeout for the worker
	//
	graceTimeout time.Duration
}

// newPool creates new workflow pool.
func newPool(codec rrt.Codec, factory server.Server, graceTimeout time.Duration, log logger.Logger, listener ...events.Listener) (pool, error) {
	const op = errors.Op("new_workflow_pool")
	env := map[string]string{RR_MODE: roadrunner_temporal.RRMode, RR_CODEC: codec.GetName()}

	cfg := rrPool.Config{
		Debug:           false,
		NumWorkers:      1,
		MaxJobs:         0,
		AllocateTimeout: time.Hour * 240,
		DestroyTimeout:  time.Second * 30,
		// no supervisor for the workflow worker
		Supervisor:      nil,
	}

	p, err := factory.NewWorkerPool(
		context.Background(),
		cfg,
		env,
		listener...,
	)
	if err != nil {
		return nil, errors.E(op, err)
	}

	wrk := &workerImpl{
		codec:        codec,
		pool:         p,
		log:          log,
		graceTimeout: graceTimeout,
	}

	return wrk, nil
}

// Start the pool in non blocking mode.
func (w *workerImpl) Start(ctx context.Context, temporal client.Temporal) error {
	const op = errors.Op("workflow_pool_start")

	err := w.initPool(ctx, temporal)
	if err != nil {
		return errors.E(op, err)
	}

	for i := 0; i < len(w.tWorkers); i++ {
		err = w.tWorkers[i].Start()
		if err != nil {
			return errors.E(op, err)
		}
	}

	return nil
}

// Destroy stops all temporal workers and application pool.
func (w *workerImpl) Destroy(ctx context.Context) error {
	w.Lock()
	defer w.Unlock()

	for i := 0; i < len(w.tWorkers); i++ {
		w.tWorkers[i].Stop()
	}

	tWorker.PurgeStickyWorkflowCache()
	// destroy pool
	w.pool.Destroy(ctx)

	return nil
}

// Pool returns rr Pool
func (w *workerImpl) Pool() rrPool.Pool {
	w.Lock()
	defer w.Unlock()
	return w.pool
}

// NewWorkflowDefinition initiates new workflow process.
func (w *workerImpl) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &workflowProcess{
		codec: w.codec,
		pool:  w,
	}
}

// NewWorkflowDefinition initiates new workflow process.
func (w *workerImpl) SeqID() uint64 {
	return atomic.AddUint64(&w.seqID, 1)
}

// Exec set of commands in thread safe move.
func (w *workerImpl) Exec(p payload.Payload) (payload.Payload, error) {
	w.Lock()
	defer w.Unlock()

	return w.pool.Exec(p)
}

func (w *workerImpl) Workers() []rrWorker.BaseProcess {
	return w.pool.Workers()
}

func (w *workerImpl) WorkflowNames() []string {
	names := make([]string, 0, len(w.workflows))
	for name := range w.workflows {
		names = append(names, name)
	}

	return names
}

// initPool request workers workflows from underlying PHP and configures temporal workers linked to the pool.
func (w *workerImpl) initPool(ctx context.Context, temporal client.Temporal) error {
	const op = errors.Op("workflow_pool_init_workers")
	workerInfo, err := rrt.FetchWorkerInfo(w.codec, w, temporal.GetDataConverter())
	if err != nil {
		return errors.E(op, err)
	}

	w.workflows = make(map[string]rrt.WorkflowInfo)
	w.tWorkers = make([]tWorker.Worker, 0, len(workerInfo))

	for i := range workerInfo {
		w.log.Debug("worker info", "taskqueue", workerInfo[i].TaskQueue, "options", workerInfo[i].Options)
		// set the graceful timeout for the worker
		workerInfo[i].Options.WorkerStopTimeout = w.graceTimeout
		wrk, err := temporal.CreateWorker(workerInfo[i].TaskQueue, workerInfo[i].Options)
		if err != nil {
			return errors.E(op, err, w.Destroy(ctx))
		}

		w.tWorkers = append(w.tWorkers, wrk)
		for j := range workerInfo[i].Workflows {
			w.log.Debug("workflows loop", "taskqueue", workerInfo[i].TaskQueue, "workflow name", workerInfo[i].Workflows[j].Name)
			wrk.RegisterWorkflowWithOptions(w, workflow.RegisterOptions{
				Name:                          workerInfo[i].Workflows[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			w.workflows[workerInfo[i].Workflows[j].Name] = workerInfo[i].Workflows[j]
		}
	}

	return nil
}
