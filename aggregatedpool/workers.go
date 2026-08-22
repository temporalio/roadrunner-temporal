package aggregatedpool

import (
	"fmt"
	"strings"

	"log/slog"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v6/api"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/sysinfo"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const tq = "taskqueue"

// ResolveInterceptors returns the list of WorkerInterceptors to apply.
// The built-in header context-bridging interceptor is always first.
// When enabledOrder is non-empty, only those named interceptors are used (in the specified order);
// an error is returned if any name is not found in the map.
// When enabledOrder is empty, all collected interceptors are applied.
func ResolveInterceptors(
	interceptors map[string]api.Interceptor,
	enabledOrder []string,
) ([]sdkinterceptor.WorkerInterceptor, error) {
	// +1 for the built-in interceptor at position 0
	result := make([]sdkinterceptor.WorkerInterceptor, 1, max(len(enabledOrder), len(interceptors))+1)
	result[0] = NewWorkerInterceptor()

	if len(enabledOrder) > 0 {
		for _, name := range enabledOrder {
			intcpt, ok := interceptors[name]
			if !ok {
				return nil, errors.E(
					errors.Op("temporal_resolve_interceptors"),
					errors.Errorf("interceptor %q is not registered", name),
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

// ResolveDataConverters returns the list of custom PayloadConverters to apply.
// When both inputs are empty, nil, nil is returned (no custom converters needed).
// When enabledOrder is non-empty, only those converters are used (in the specified order);
// an error is returned if any encoding is not found in the map.
// When enabledOrder is empty, all collected converters are applied.
func ResolveDataConverters(
	converters map[string]converter.PayloadConverter,
	enabledOrder []string,
) ([]converter.PayloadConverter, error) {
	if len(converters) == 0 && len(enabledOrder) == 0 {
		return nil, nil
	}

	if len(enabledOrder) > 0 {
		result := make([]converter.PayloadConverter, 0, len(enabledOrder))
		for _, encoding := range enabledOrder {
			dc, ok := converters[encoding]
			if !ok {
				return nil, errors.E(
					errors.Op("temporal_resolve_data_converters"),
					errors.Errorf("data converter with encoding %q is not registered", encoding),
				)
			}
			result = append(result, dc)
		}
		return result, nil
	}

	result := make([]converter.PayloadConverter, 0, len(converters))
	for _, dc := range converters {
		result = append(result, dc)
	}
	return result, nil
}

// registerWorkflow runs the SDK workflow registration and converts any panic it raises
// into a normal error. The Temporal SDK panics (rather than returning an error) on
// invalid registration config; because RR's config comes from the PHP worker at
// runtime, that must fail worker init cleanly instead of crashing the process. No
// SDK-internal condition is mirrored, so nothing here needs to track SDK changes.
func registerWorkflow(register func(), name, taskQueue string) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		op := errors.Op("temporal_register_workflow")
		// Best-effort friendly hint for the common case (missing versioning behavior).
		// Purely cosmetic: if the SDK reworks this message we still return the generic
		// error below, so correctness never depends on the matched string.
		if msg, ok := r.(string); ok && strings.Contains(msg, "versioning behavior") {
			err = errors.E(op, errors.Errorf("workflow %q on task queue %q has no versioning behavior set while worker versioning is enabled; set a VersioningBehavior on the workflow or a DefaultVersioningBehavior on the worker", name, taskQueue))
			return
		}

		err = errors.E(op, errors.Errorf("failed to register workflow %q on task queue %q: %v", name, taskQueue, r))
	}()

	register()
	return nil
}

func TemporalWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *slog.Logger, tc temporalClient.Client, interceptors map[string]api.Interceptor, configuredInterceptors []string) ([]worker.Worker, error) {
	resolved, err := ResolveInterceptors(interceptors, configuredInterceptors)
	if err != nil {
		return nil, err
	}

	workers := make([]worker.Worker, 0, len(wi))

	for i := range wi {
		workerInfo := wi[i]
		log.Debug("worker info", "worker_info", workerInfo)

		// Override to 0: RoadRunner manages worker lifecycle independently
		workerInfo.Options.WorkerStopTimeout = 0

		if workerInfo.TaskQueue == "" {
			workerInfo.TaskQueue = temporalClient.DefaultNamespace
		}

		if workerInfo.Options.Identity == "" {
			workerInfo.Options.Identity = fmt.Sprintf(
				"roadrunner:%s:%s",
				workerInfo.TaskQueue,
				uuid.NewString(),
			)
		}

		workerInfo.Options.Interceptors = append(workerInfo.Options.Interceptors, resolved...)

		// CPU/memory source for worker heartbeats (Temporal UI "Resource Utilization").
		// The provider is a process-wide singleton shared by all task queues and is only
		// queried when a heartbeat fires.
		workerInfo.Options.SysInfoProvider = sysinfo.SysInfoProvider()

		wrk := worker.New(tc, workerInfo.TaskQueue, workerInfo.Options)
		dynamicWorkflowRegistered := false

		for _, wf := range workerInfo.Workflows {
			// A dynamic workflow is the catch-all: register it via
			// RegisterDynamicWorkflow (not by name) so it handles any workflow
			// type that has no statically registered handler.
			if wf.Dynamic {
				if dynamicWorkflowRegistered {
					return nil, errors.E(
						errors.Op("temporal_workers"),
						errors.Errorf("multiple dynamic workflows configured on task queue %q", workerInfo.TaskQueue),
					)
				}

				err := registerWorkflow(func() {
					wrk.RegisterDynamicWorkflow(wDef, workflow.DynamicRegisterOptions{})
				}, wf.Name, workerInfo.TaskQueue)
				if err != nil {
					return nil, err
				}
				dynamicWorkflowRegistered = true

				log.Debug("dynamic workflow registered", tq, workerInfo.TaskQueue, "workflow name", wf.Name)

				continue
			}

			err := registerWorkflow(func() {
				wrk.RegisterWorkflowWithOptions(wDef, workflow.RegisterOptions{
					Name:                          wf.Name,
					VersioningBehavior:            wf.VersioningBehavior,
					DisableAlreadyRegisteredCheck: false,
				})
			}, wf.Name, workerInfo.TaskQueue)
			if err != nil {
				return nil, err
			}

			log.Debug("workflow registered", tq, workerInfo.TaskQueue, "workflow name", wf.Name, "versioning_behavior", int(wf.VersioningBehavior))
		}

		if actDef.disableActivityWorkers {
			log.Debug("activity workers disabled", tq, workerInfo.TaskQueue)
			// add worker to the pool without activities
			workers = append(workers, wrk)
			continue
		}

		for _, activity := range workerInfo.Activities {
			wrk.RegisterActivityWithOptions(actDef.execute, tActivity.RegisterOptions{
				Name:                          activity.Name,
				DisableAlreadyRegisteredCheck: false,
				SkipInvalidStructFunctions:    false,
			})

			log.Debug("activity registered", tq, workerInfo.TaskQueue, "workflow name", activity.Name)
		}
		// add worker to the pool
		workers = append(workers, wrk)
	}

	log.Debug("workers initialized", "num_workers", len(workers))

	return workers, nil
}
