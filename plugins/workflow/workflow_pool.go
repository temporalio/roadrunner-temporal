package workflow

import (
	"context"
	"log"
	"sync"

	"github.com/spiral/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/interfaces/server"
	"github.com/spiral/roadrunner/v2/util"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// workflowPool manages workflowProcess executions between worker restarts.
type workflowPool struct {
	events    util.EventsHandler
	seqID     uint64
	workflows map[string]rrt.WorkflowInfo
	tWorkers  []worker.Worker
	mu        sync.Mutex
	worker    roadrunner.SyncWorker
}

// NewWorkflowPool creates new workflow pool.
func NewWorkflowPool(ctx context.Context, factory server.WorkerFactory) (*workflowPool, error) {
	w, err := factory.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)

	if err != nil {
		return nil, err
	}

	go func() {
		// todo: move into pool start
		// todo: must be supervised
		// todo: report to parent supervisor

		err := w.Wait(ctx)
		log.Print(err)
	}()

	sw, err := roadrunner.NewSyncWorker(w)
	if err != nil {
		return nil, err
	}

	return &workflowPool{worker: sw}, nil
}

// AddListener adds event listeners to the workflow pool.
func (pool *workflowPool) AddListener(listener util.EventListener) {
	pool.events.AddListener(listener)
}

// Start the pool in non blocking mode. TODO: capture worker errors.
func (pool *workflowPool) Start(ctx context.Context, temporal temporal.Temporal) error {
	err := pool.initWorkers(ctx, temporal)
	if err != nil {
		return err
	}

	for i := 0; i < len(pool.tWorkers); i++ {
		err := pool.tWorkers[i].Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// Destroy stops all temporal workers and application worker.
func (pool *workflowPool) Destroy(ctx context.Context) {
	for i := 0; i < len(pool.tWorkers); i++ {
		pool.tWorkers[i].Stop()
	}

	// todo: pass via event callback
	//if err := pool.worker.Stop(ctx); err != nil {
	//	pool.events.Push()
	//}
}

// NewWorkflowDefinition initiates new workflow process.
func (pool *workflowPool) NewWorkflowDefinition() bindings.WorkflowDefinition {
	// todo: add logging or event listener?
	return &workflowProcess{pool: pool}
}

// Exec set of commands in thread safe move.
func (pool *workflowPool) Exec(p roadrunner.Payload) (roadrunner.Payload, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.worker.Exec(p)
}

// initWorkers request workers workflows from underlying PHP and configures temporal workers linked to the pool.
func (pool *workflowPool) initWorkers(ctx context.Context, temporal temporal.Temporal) error {
	workerInfo, err := rrt.GetWorkerInfo(pool)
	if err != nil {
		return err
	}

	pool.workflows = make(map[string]rrt.WorkflowInfo)
	pool.tWorkers = make([]worker.Worker, 0)

	for _, info := range workerInfo {
		w, err := temporal.CreateWorker(info.TaskQueue, info.Options.TemporalOptions())
		//worker.SetStickyWorkflowCacheSize(1)
		if err != nil {
			pool.Destroy(ctx)
			return errors.E(errors.Op("createTemporalWorker"), err)
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
