package aggregatedpool

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/payload"
	"github.com/temporalio/roadrunner-temporal/v2/common"
	"github.com/temporalio/roadrunner-temporal/v2/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func GetWorkerInfo(c common.Codec, p common.Pool, rrVersion string, wi *[]*internal.WorkerInfo) error {
	const op = errors.Op("workflow_definition_init")

	// todo(rustatian): to sync.Pool
	pld := &payload.Payload{}
	err := c.Encode(&internal.Context{}, pld, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{RRVersion: rrVersion}})
	if err != nil {
		return errors.E(op, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := p.Exec(ctx, pld)
	if err != nil {
		return errors.E(op, err)
	}

	err = c.DecodeWorkerInfo(resp, wi)
	if err != nil {
		return errors.E(op, err)
	}

	return nil
}

func GrabWorkflows(wi []*internal.WorkerInfo) map[string]*internal.WorkflowInfo {
	workflowInfo := make(map[string]*internal.WorkflowInfo)

	for i := 0; i < len(wi); i++ {
		for j := 0; j < len(wi[i].Workflows); j++ {
			workflowInfo[wi[i].Workflows[j].Name] = &wi[i].Workflows[j]
		}
	}

	return workflowInfo
}

func GrabActivities(wi []*internal.WorkerInfo) map[string]*internal.ActivityInfo {
	activitiesInfo := make(map[string]*internal.ActivityInfo)

	for i := 0; i < len(wi); i++ {
		for j := 0; j < len(wi[i].Activities); j++ {
			activitiesInfo[wi[i].Activities[j].Name] = &wi[i].Activities[j]
		}
	}

	return activitiesInfo
}

func InitWorkers(wDef *Workflow, actDef *Activity, wi []*internal.WorkerInfo, log *zap.Logger, tc temporalClient.Client, graceTimeout time.Duration) ([]worker.Worker, error) {
	workers := make([]worker.Worker, 0, 1)

	for i := 0; i < len(wi); i++ {
		log.Debug("worker info", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("options", wi[i].Options))

		wi[i].Options.WorkerStopTimeout = graceTimeout

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

		wrk := worker.New(tc, wi[i].TaskQueue, wi[i].Options)

		for j := 0; j < len(wi[i].Workflows); j++ {
			wrk.RegisterWorkflowWithOptions(wDef, workflow.RegisterOptions{
				Name:                          wi[i].Workflows[j].Name,
				DisableAlreadyRegisteredCheck: false,
			})

			log.Debug("workflow registered", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("workflow name", wi[i].Workflows[j].Name))
		}

		for j := 0; j < len(wi[i].Activities); j++ {
			wrk.RegisterActivityWithOptions(actDef.execute, tActivity.RegisterOptions{
				Name:                          wi[i].Activities[j].Name,
				DisableAlreadyRegisteredCheck: false,
				SkipInvalidStructFunctions:    false,
			})

			log.Debug("activity registered", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("workflow name", wi[i].Activities[j].Name))
		}

		workers = append(workers, wrk)
	}

	log.Debug("workers initialized", zap.Int("num_workers", len(workers)))

	return workers, nil
}
