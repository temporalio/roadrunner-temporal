package aggregatedpool

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/temporalio/roadrunner-temporal/v5/common"
	"github.com/temporalio/roadrunner-temporal/v5/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

const tq = "taskqueue"

func TemporalWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *zap.Logger, tc temporalClient.Client, interceptors map[string]common.Interceptor) ([]worker.Worker, error) {
	workers := make([]worker.Worker, 0, 1)

	for i := 0; i < len(wi); i++ {
		log.Debug("worker info",
			zap.String(tq, wi[i].TaskQueue),
			zap.Int("num_workflows", len(wi[i].Workflows)),
			zap.Int("num_activities", len(wi[i].Activities)),
			zap.Int("max_concurrent_activity_execution_size", wi[i].Options.MaxConcurrentActivityExecutionSize),
			zap.Float64("worker_activities_per_second", wi[i].Options.WorkerActivitiesPerSecond),
			zap.Int("max_concurrent_local_activity_execution_size", wi[i].Options.MaxConcurrentLocalActivityExecutionSize),
			zap.Float64("worker_local_activities_per_second", wi[i].Options.WorkerLocalActivitiesPerSecond),
			zap.Float64("task_queue_activities_per_second", wi[i].Options.TaskQueueActivitiesPerSecond),
			zap.Int("max_concurrent_activity_task_pollers", wi[i].Options.MaxConcurrentActivityTaskPollers),
			zap.Int("max_concurrent_workflow_task_pollers", wi[i].Options.MaxConcurrentWorkflowTaskPollers),
			zap.Bool("enable_logging_in_replay", wi[i].Options.EnableLoggingInReplay),
			zap.String("workflow_panic_policy", policyToString(wi[i].Options.WorkflowPanicPolicy)),
			zap.Duration("worker_stop_timeout", wi[i].Options.WorkerStopTimeout),
			zap.Bool("enable_session_worker", wi[i].Options.EnableSessionWorker),
			zap.Int("max_concurrent_session_execution_size", wi[i].Options.MaxConcurrentSessionExecutionSize),
			zap.Bool("disable_workflow_worker", wi[i].Options.DisableWorkflowWorker),
			zap.Bool("local_activity_worker_only", wi[i].Options.LocalActivityWorkerOnly),
			zap.String("identity", wi[i].Options.Identity),
			zap.Duration("deadlock_detection_timeout", wi[i].Options.DeadlockDetectionTimeout),
			zap.Duration("max_heartbeat_throttle_interval", wi[i].Options.MaxHeartbeatThrottleInterval),
			zap.Duration("default_heartbeat_throttle_interval", wi[i].Options.DefaultHeartbeatThrottleInterval),
			zap.Bool("disable_eager_activities", wi[i].Options.DisableEagerActivities),
			zap.Int("max_concurrent_eager_activity_execution_size", wi[i].Options.MaxConcurrentEagerActivityExecutionSize),
			zap.Bool("disable_registration_aliasing", wi[i].Options.DisableRegistrationAliasing),
			zap.String("build_id", wi[i].Options.BuildID),
			zap.Bool("use_buildID_for_versioning", wi[i].Options.UseBuildIDForVersioning),
		)

		// just to be sure
		wi[i].Options.WorkerStopTimeout = 0

		if wi[i].TaskQueue == "" {
			wi[i].TaskQueue = temporalClient.DefaultNamespace
		}

		if wi[i].Options.BuildID != "" {
			wi[i].Options.UseBuildIDForVersioning = true
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
		for _, interceptor := range interceptors {
			wi[i].Options.Interceptors = append(wi[i].Options.Interceptors, interceptor.WorkerInterceptor())
		}

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

func policyToString(enum worker.WorkflowPanicPolicy) string {
	switch enum {
	case worker.BlockWorkflow:
		return "block"
	case worker.FailWorkflow:
		return "fail"
	default:
		return "unknown"
	}
}
