package workflow

import (
	"context"
	"github.com/fatih/color"
	"github.com/spiral/endure/errors"
	"github.com/spiral/roadrunner/v2"
	"github.com/spiral/roadrunner/v2/plugins/app"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"log"
	"sync"
)

// workflowPool manages workflowProcess executions between worker restarts.
type workflowPool struct {
	seqID     uint64
	workflows map[string]rrt.WorkflowInfo
	tWorkers  []worker.Worker
	mu        sync.Mutex
	worker    roadrunner.SyncWorker
}

// NewWorkflowPool creates new workflow pool.
func NewWorkflowPool(ctx context.Context, factory app.WorkerFactory) (*workflowPool, error) {
	w, err := factory.NewWorker(
		context.Background(),
		map[string]string{"RR_MODE": RRMode},
	)
	if err != nil {
		return nil, err
	}

	w.AddListener(func(event interface{}) {
		if event.(roadrunner.WorkerEvent).Event == roadrunner.EventWorkerLog {
			// todo: recreate pool
			log.Print(color.RedString(string(event.(roadrunner.WorkerEvent).Payload.([]byte))))
		}
	})

	go func() {
		// todo: move into pool start
		err := w.Wait(ctx)
		// todo: must be supervised
		log.Print(err)

		// todo: report to parent supervisor
	}()

	sw, err := roadrunner.NewSyncWorker(w)
	if err != nil {
		return nil, err
	}

	return &workflowPool{worker: sw}, nil
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
	pool.worker.Stop(ctx)
}

// NewWorkflowDefinition initiates new workflow process.
func (pool *workflowPool) NewWorkflowDefinition() bindings.WorkflowDefinition {
	// todo: add logging or event listener?
	return &workflowProcess{worker: pool.worker, pool: pool}
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

	pool.tWorkers = make([]worker.Worker, 0)
	for _, info := range workerInfo {
		w, err := temporal.CreateWorker(info.TaskQueue, info.Options.TemporalOptions())
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
