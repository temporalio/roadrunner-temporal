package workflow

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2/interfaces/events"
	"github.com/spiral/roadrunner/v2/interfaces/server"
	rrWorker "github.com/spiral/roadrunner/v2/interfaces/worker"
	"github.com/spiral/roadrunner/v2/pkg/payload"
	syncWorker "github.com/spiral/roadrunner/v2/pkg/worker"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	EventWorkerExit = iota + 8390
	EventNewWorkflowProcess
)

type (
	workflowPool interface {
		SeqID() uint64
		Exec(p payload.Payload) (payload.Payload, error)
		Start(ctx context.Context, temporal temporal.Temporal) error
		Destroy(ctx context.Context) error
		Workers() []rrWorker.BaseProcess
		WorkflowNames() []string
		Active() bool
	}

	PoolEvent struct {
		Event   int
		Context interface{}
		Caused  error
	}

	// workflowPoolImpl manages workflowProcess executions between worker restarts.
	workflowPoolImpl struct {
		events    events.Handler
		seqID     uint64
		workflows map[string]rrt.WorkflowInfo
		tWorkers  []worker.Worker
		mu        sync.Mutex
		worker    rrWorker.SyncWorker
	}
)

// newWorkflowPool creates new workflow pool.
func newWorkflowPool(listener events.EventListener, factory server.Server) (workflowPool, error) {
	w, err := factory.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, errors.E(errors.Op("newWorker"), err)
	}

	w.AddListener(listener)

	go func() {
		err := w.Wait()
		listener(PoolEvent{Event: EventWorkerExit, Caused: err})
	}()

	sw, err := syncWorker.From(w)
	if err != nil {
		return nil, errors.E(errors.Op("newSyncWorker"), err)
	}

	return &workflowPoolImpl{worker: sw}, nil
}

// Start the pool in non blocking mode.
func (pool *workflowPoolImpl) Start(ctx context.Context, temporal temporal.Temporal) error {
	err := pool.initWorkers(ctx, temporal)
	if err != nil {
		return errors.E(errors.Op("initWorkers"), err)
	}

	for i := 0; i < len(pool.tWorkers); i++ {
		err := pool.tWorkers[i].Start()
		if err != nil {
			return errors.E(errors.Op("startTemporalWorker"), err)
		}
	}

	pool.active = true

	return nil
}

// Active.
func (pool *workflowPoolImpl) Active() bool {
	return pool.active
}

// Destroy stops all temporal workers and application worker.
func (pool *workflowPoolImpl) Destroy(ctx context.Context) error {
	pool.active = false
	for i := 0; i < len(pool.tWorkers); i++ {
		pool.tWorkers[i].Stop()
	}

	if err := pool.worker.Stop(ctx); err != nil {
		return errors.E(errors.Op("stopWorkflowWorker"), err)
	}

	return nil
}

// NewWorkflowDefinition initiates new workflow process.
func (pool *workflowPoolImpl) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &workflowProcess{pool: pool}
}

// NewWorkflowDefinition initiates new workflow process.
func (pool *workflowPoolImpl) SeqID() uint64 {
	return atomic.AddUint64(&pool.seqID, 1)
}

// Exec set of commands in thread safe move.
func (pool *workflowPoolImpl) Exec(p payload.Payload) (payload.Payload, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.worker.Exec(p)
}

func (pool *workflowPoolImpl) Workers() []rrWorker.BaseProcess {
	return []rrWorker.BaseProcess{pool.worker}
}

func (pool *workflowPoolImpl) WorkflowNames() []string {
	names := make([]string, 0, len(pool.workflows))
	for name, _ := range pool.workflows {
		names = append(names, name)
	}

	return names
}

// initWorkers request workers workflows from underlying PHP and configures temporal workers linked to the pool.
func (pool *workflowPoolImpl) initWorkers(ctx context.Context, temporal temporal.Temporal) error {
	workerInfo, err := rrt.GetWorkerInfo(pool)
	if err != nil {
		return err
	}

	pool.workflows = make(map[string]rrt.WorkflowInfo)
	pool.tWorkers = make([]worker.Worker, 0)

	for _, info := range workerInfo {
		w, err := temporal.CreateWorker(info.TaskQueue, info.Options)
		// worker.SetStickyWorkflowCacheSize(1)
		if err != nil {
			return errors.E(errors.Op("createTemporalWorker"), err, pool.Destroy(ctx))
		}

		pool.tWorkers = append(pool.tWorkers, w)
		for _, workflowInfo := range info.Workflows {
			w.RegisterWorkflowWithOptions(pool, workflow.RegisterOptions{
				Name:                          workflowInfo.Name,
				DisableAlreadyRegisteredCheck: false,
			})

			pool.workflows[workflowInfo.Name] = workflowInfo
		}
	}

	return nil
}
