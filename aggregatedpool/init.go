package aggregatedpool

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/api/v2/payload"
	"github.com/roadrunner-server/api/v2/pool"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/internal"
	tActivity "go.temporal.io/sdk/activity"
	temporalClient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func Init(wDef *Workflow, actDef *Activity, p pool.Pool, c Codec, log *zap.Logger, tc temporalClient.Client, graceTimeout time.Duration, version string) ([]worker.Worker, map[string]internal.WorkflowInfo, []string, error) {
	const op = errors.Op("workflow_definition_init")

	// todo(rustatian): to sync.Pool
	pld := &payload.Payload{}
	err := c.Encode(&internal.Context{}, pld, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{RRVersion: version}})
	if err != nil {
		return nil, nil, nil, errors.E(op, err)
	}

	resp, err := p.Exec(pld)
	if err != nil {
		return nil, nil, nil, errors.E(op, err)
	}

	wi := make([]*internal.WorkerInfo, 0, 1)
	err = c.DecodeWorkerInfo(resp, &wi)
	if err != nil {
		return nil, nil, nil, errors.E(op, err)
	}

	workers := make([]worker.Worker, 0, 1)
	workflows := make(map[string]internal.WorkflowInfo, 2)
	activities := make([]string, 0, 2)

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

			workflows[wi[i].Workflows[j].Name] = wi[i].Workflows[j]

			log.Debug("workflow registered", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("workflow name", wi[i].Workflows[j].Name))
		}

		for j := 0; j < len(wi[i].Activities); j++ {
			wrk.RegisterActivityWithOptions(actDef.execute, tActivity.RegisterOptions{
				Name:                          wi[i].Activities[j].Name,
				DisableAlreadyRegisteredCheck: false,
				SkipInvalidStructFunctions:    false,
			})

			log.Debug("activity registered", zap.String("taskqueue", wi[i].TaskQueue), zap.Any("workflow name", wi[i].Activities[j].Name))
			activities = append(activities, wi[i].Activities[j].Name)
		}

		workers = append(workers, wrk)
	}

	log.Debug("workers initialized", zap.Int("num_workers", len(workers)))

	return workers, workflows, activities, nil
}
