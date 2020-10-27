package workflow

import (
	"context"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"sync"
	"sync/atomic"
)

// workflowPool manages workflowProxy executions between worker restarts.
type workflowPool struct {
	seqID           uint64
	numW            uint64
	numS            uint64
	mu              sync.Mutex
	worker          roadrunner.SyncWorker
	temporalWorkers []worker.Worker
}

func newWorkflowPool(worker roadrunner.WorkerBase) (*workflowPool, error) {
	syncWorker, err := roadrunner.NewSyncWorker(worker)
	if err != nil {
		return nil, err
	}

	return &workflowPool{worker: syncWorker}, nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (ss *workflowPool) InitSession(ctx context.Context, temporal temporal.Temporal) error {
	info, err := rrt.GetWorkerInfo(ss.worker)
	if err != nil {
		return err
	}

	ss.temporalWorkers = make([]worker.Worker, 0)
	for _, cfg := range info {
		w, err := temporal.CreateWorker(cfg.TaskQueue, cfg.Options.TemporalOptions())

		if err != nil {
			ss.Destroy(ctx)
			return err
		}

		ss.temporalWorkers = append(ss.temporalWorkers, w)
		for _, wCfg := range cfg.Workflows {
			// todo: store signal information somewhere
			w.RegisterWorkflowWithOptions(ss, workflow.RegisterOptions{
				Name:                          wCfg.Name,
				DisableAlreadyRegisteredCheck: false,
			})
		}
	}

	return nil
}

func (ss *workflowPool) Start() error {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		err := ss.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}

	return nil
}

func (ss *workflowPool) Destroy(ctx context.Context) {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		ss.temporalWorkers[i].Stop()
	}

	// todo: pass via event callback
	ss.worker.Stop(ctx)
}

func (ss *workflowPool) NewWorkflowDefinition() bindings.WorkflowDefinition {
	atomic.AddUint64(&ss.numW, 1)
	return &workflowProxy{
		worker:  ss.worker,
		session: ss,
	}
}

func (ss *workflowPool) Exec(p roadrunner.Payload) (roadrunner.Payload, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	return ss.worker.Exec(p)
}
