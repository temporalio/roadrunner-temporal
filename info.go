package rrtemporal

import (
	"context"
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/roadrunner-server/goridge/v3/pkg/frame"
	"github.com/roadrunner-server/sdk/v4/payload"
	"github.com/temporalio/roadrunner-temporal/v4/common"
	"github.com/temporalio/roadrunner-temporal/v4/internal"
)

func WorkerInfo(c common.Codec, p common.Pool, rrVersion string) ([]*internal.WorkerInfo, error) {
	const op = errors.Op("workflow_definition_init")

	pl := &payload.Payload{}
	err := c.Encode(&internal.Context{}, pl, &internal.Message{ID: 0, Command: internal.GetWorkerInfo{RRVersion: rrVersion}})
	if err != nil {
		return nil, errors.E(op, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ch := make(chan struct{}, 1)
	resp, err := p.Exec(ctx, pl, ch)
	if err != nil {
		return nil, errors.E(op, err)
	}

	var r *payload.Payload
	select {
	case pld := <-resp:
		if pld.Error() != nil {
			return nil, errors.E(op, pld.Error())
		}
		// streaming is not supported
		if pld.Payload().Flags&frame.STREAM != 0 {
			ch <- struct{}{}
			return nil, errors.E(op, errors.Str("streaming is not supported"))
		}

		// assign the payload
		r = pld.Payload()
	default:
		return nil, errors.E(op, errors.Str("worker empty response"))
	}

	wi := make([]*internal.WorkerInfo, 0, 2)
	err = c.DecodeWorkerInfo(r, &wi)
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
