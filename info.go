package roadrunner_temporal //nolint:revive,stylecheck

import (
	"context"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/sdk/v3/payload"
	"github.com/temporalio/roadrunner-temporal/v3/common"
	"github.com/temporalio/roadrunner-temporal/v3/internal"
)

func WorkerInfo(c common.Codec, p common.Pool, rrVersion string) ([]*internal.WorkerInfo, error) {
	const op = errors.Op("workflow_definition_init")

	pld := &payload.Payload{}
	err := c.Encode(&internal.Context{}, pld, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{RRVersion: rrVersion}})
	if err != nil {
		return nil, errors.E(op, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := p.Exec(ctx, pld)
	if err != nil {
		return nil, errors.E(op, err)
	}

	wi := make([]*internal.WorkerInfo, 0, 2)
	err = c.DecodeWorkerInfo(resp, &wi)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return wi, nil
}

func WorkflowsInfo(wi []*internal.WorkerInfo) map[string]*internal.WorkflowInfo {
	workflowInfo := make(map[string]*internal.WorkflowInfo)

	for i := 0; i < len(wi); i++ {
		for j := 0; j < len(wi[i].Workflows); j++ {
			workflowInfo[wi[i].Workflows[j].Name] = &wi[i].Workflows[j]
		}
	}

	return workflowInfo
}

func ActivitiesInfo(wi []*internal.WorkerInfo) map[string]*internal.ActivityInfo {
	activitiesInfo := make(map[string]*internal.ActivityInfo)

	for i := 0; i < len(wi); i++ {
		for j := 0; j < len(wi[i].Activities); j++ {
			activitiesInfo[wi[i].Activities[j].Name] = &wi[i].Activities[j]
		}
	}

	return activitiesInfo
}
