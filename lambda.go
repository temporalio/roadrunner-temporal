package rrtemporal

import (
	"context"

	"github.com/temporalio/roadrunner-temporal/v6/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"go.temporal.io/sdk/worker"
)

const lambdaRuntimeAPIEnv = "AWS_LAMBDA_RUNTIME_API"

// StartInvocation and StopInvocation let the lambda plugin drive this one per
// AWS Lambda invocation: only the Temporal workers are cycled, the PHP pools
// stay up for the whole lifetime of the execution environment.
func (p *Plugin) StartInvocation(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.startTemporalWorkersLocked()
}

func (p *Plugin) StopInvocation(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopTemporalWorkersLocked()

	return nil
}

// startTemporalWorkersLocked requires p.mu to be held.
func (p *Plugin) startTemporalWorkersLocked() error {
	workers, err := aggregatedpool.TemporalWorkers(
		p.getWfDef(),
		p.getActDef(),
		cloneWorkerInfo(p.temporal.workerInfo),
		p.log,
		p.temporal.client,
		p.temporal.interceptors,
		p.config.Interceptors,
	)
	if err != nil {
		return err
	}

	for i := range workers {
		if err := workers[i].Start(); err != nil {
			return err
		}
	}

	p.temporal.workers = workers

	return nil
}

// stopTemporalWorkersLocked requires p.mu to be held.
func (p *Plugin) stopTemporalWorkersLocked() {
	for i := range p.temporal.workers {
		p.temporal.workers[i].Stop()
	}

	p.temporal.workers = nil
	worker.PurgeStickyWorkflowCache()
}

// cloneWorkerInfo keeps the stored worker info pristine: TemporalWorkers appends
// the resolved interceptors into Options, so reusing the same value for every
// invocation would stack them up.
func cloneWorkerInfo(source []*internal.WorkerInfo) []*internal.WorkerInfo {
	cloned := make([]*internal.WorkerInfo, 0, len(source))
	for i := range source {
		copied := *source[i]
		copied.Options.Interceptors = nil
		cloned = append(cloned, &copied)
	}

	return cloned
}
