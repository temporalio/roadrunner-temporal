package workflow

import (
	"context"
	"github.com/spiral/roadrunner/v2"
	rrt "github.com/temporalio/roadrunner-temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	bindings "go.temporal.io/sdk/internalbindings"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// session manages workflowProxy executions between worker restarts.
type session struct {
	seqID           uint64
	worker          roadrunner.SyncWorker
	temporalWorkers []worker.Worker
}

func newSession(worker roadrunner.WorkerBase) (*session, error) {
	syncWorker, err := roadrunner.NewSyncWorker(worker)
	if err != nil {
		return nil, err
	}

	// todo: watch worker state and restart session or notify server and dead session

	return &session{worker: syncWorker}, nil
}

// initWorkers request workers info from underlying PHP and configures temporal workers linked to the pool.
func (ss *session) InitSession(ctx context.Context, temporal temporal.Temporal) error {
	info, err := rrt.GetWorkerInfo(ctx, ss.worker)
	if err != nil {
		return err
	}

	ss.temporalWorkers = make([]worker.Worker, 0)
	for _, cfg := range info {
		w, err := temporal.CreateWorker(cfg.TaskQueue, cfg.Options.ToNativeOptions())

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

func (ss *session) Start() error {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		err := ss.temporalWorkers[i].Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ss *session) Destroy(ctx context.Context) {
	for i := 0; i < len(ss.temporalWorkers); i++ {
		ss.temporalWorkers[i].Stop()
	}

	// todo: how to handle error?
	ss.worker.Stop(ctx)
}

func (ss *session) NewWorkflowDefinition() bindings.WorkflowDefinition {
	return &workflowProxy{
		seqID:  &ss.seqID,
		worker: ss.worker,
		// todo: data converter?
	}
}
