package aggregatedpool

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/temporalio/roadrunner-temporal/v2/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

const tq = "taskqueue"

func TemporalWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *zap.Logger, tc temporalClient.Client) ([]worker.Worker, error) {
	workers := make([]worker.Worker, 0, 1)

	for i := 0; i < len(wi); i++ {
		log.Debug("worker info", zap.String(tq, wi[i].TaskQueue), zap.Any("options", wi[i].Options))

		wi[i].Options.WorkerStopTimeout = time.Minute

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

		// interceptor used here to  the headers
		wi[i].Options.Interceptors = append(wi[i].Options.Interceptors, NewWorkerInterceptor())

		wrk := worker.New(tc, wi[i].TaskQueue, wi[i].Options)

		for j := 0; j < len(wi[i].Workflows); j++ {
			wrk.RegisterWorkflowWithOptions(wDef, workflow.RegisterOptions{
				Name:                          wi[i].Workflows[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			log.Debug("workflow registered", zap.String(tq, wi[i].TaskQueue), zap.Any("workflow name", wi[i].Workflows[j].Name))
		}

		for j := 0; j < len(wi[i].Activities); j++ {
			wrk.RegisterActivityWithOptions(actDef.execute, tActivity.RegisterOptions{
				Name:                          wi[i].Activities[j].Name,
				DisableAlreadyRegisteredCheck: false,
				SkipInvalidStructFunctions:    false,
			})

			log.Debug("activity registered", zap.String(tq, wi[i].TaskQueue), zap.Any("workflow name", wi[i].Activities[j].Name))
		}

		workers = append(workers, wrk)
	}

	log.Debug("workers initialized", zap.Int("num_workers", len(workers)))

	return workers, nil
}
