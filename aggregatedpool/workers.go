package aggregatedpool

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

const tq = "taskqueue"

func TemporalWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *zap.Logger, tc temporalClient.Client, interceptors map[string]api.Interceptor) ([]worker.Worker, error) {
	workers := make([]worker.Worker, 0, 1)

	for i := range wi {
		log.Debug("worker info", zap.Any("worker_info", wi[i]))

		// just to be sure
		wi[i].Options.WorkerStopTimeout = 0

		if wi[i].TaskQueue == "" {
			wi[i].TaskQueue = temporalClient.DefaultNamespace
		}

		if wi[i].Options.Identity == "" {
			wi[i].Options.Identity = fmt.Sprintf(
				"roadrunner:%s:%s",
				wi[i].TaskQueue,
				uuid.NewString(),
			)
		}

		// interceptor used here to  the headers
		wi[i].Options.Interceptors = append(wi[i].Options.Interceptors, NewWorkerInterceptor())
		for _, interceptor := range interceptors {
			wi[i].Options.Interceptors = append(wi[i].Options.Interceptors, interceptor.WorkerInterceptor())
		}

		wrk := worker.New(tc, wi[i].TaskQueue, wi[i].Options)

		for j := 0; j < len(wi[i].Workflows); j++ {
			wrk.RegisterWorkflowWithOptions(wDef, workflow.RegisterOptions{
				Name:                          wi[i].Workflows[j].Name,
				VersioningBehavior:            wi[i].Workflows[j].VersioningBehavior,
				DisableAlreadyRegisteredCheck: false,
			})

			log.Debug("workflow registered", zap.String(tq, wi[i].TaskQueue), zap.Any("workflow name", wi[i].Workflows[j].Name), zap.Int("versioning_behavior", int(wi[i].Workflows[j].VersioningBehavior)))
		}

		if actDef.disableActivityWorkers {
			log.Debug("activity workers disabled", zap.String(tq, wi[i].TaskQueue))
			// add worker to the pool without activities
			workers = append(workers, wrk)
			continue
		}

		for j := 0; j < len(wi[i].Activities); j++ {
			wrk.RegisterActivityWithOptions(actDef.execute, tActivity.RegisterOptions{
				Name:                          wi[i].Activities[j].Name,
				DisableAlreadyRegisteredCheck: false,
				SkipInvalidStructFunctions:    false,
			})

			log.Debug("activity registered", zap.String(tq, wi[i].TaskQueue), zap.Any("workflow name", wi[i].Activities[j].Name))
		}
		// add worker to the pool
		workers = append(workers, wrk)
	}

	log.Debug("workers initialized", zap.Int("num_workers", len(workers)))

	return workers, nil
}
