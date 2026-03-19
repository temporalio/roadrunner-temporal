package aggregatedpool

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v5/api"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

const tq = "taskqueue"

// ResolveInterceptors returns the ordered list of WorkerInterceptors to apply.
// The built-in header-propagation interceptor is always first.
// When configuredOrder is non-empty, only those named interceptors are used (in order);
// an error is returned if any name is not found in the map.
// When configuredOrder is empty, all collected interceptors are applied.
func ResolveInterceptors(
	interceptors map[string]api.Interceptor,
	configuredOrder []string,
) ([]sdkinterceptor.WorkerInterceptor, error) {
	n := max(len(configuredOrder), len(interceptors))
	result := make([]sdkinterceptor.WorkerInterceptor, 1, 1+n)
	result[0] = NewWorkerInterceptor()

	if len(configuredOrder) > 0 {
		for _, name := range configuredOrder {
			intcpt, ok := interceptors[name]
			if !ok {
				return nil, errors.E(
					errors.Op("temporal_resolve_interceptors"),
					errors.Str(fmt.Sprintf("interceptor %q is not registered", name)),
				)
			}

			result = append(result, intcpt.WorkerInterceptor())
		}
	} else {
		for _, intcpt := range interceptors {
			result = append(result, intcpt.WorkerInterceptor())
		}
	}

	return result, nil
}

func TemporalWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *zap.Logger, tc temporalClient.Client, interceptors map[string]api.Interceptor, configuredInterceptors []string) ([]worker.Worker, error) {
	resolved, err := ResolveInterceptors(interceptors, configuredInterceptors)
	if err != nil {
		return nil, err
	}

	workers := make([]worker.Worker, 0, len(wi))

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

		wi[i].Options.Interceptors = append(wi[i].Options.Interceptors, resolved...)

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
