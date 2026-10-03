package rrtemporal

import (
	"context"
	"time"

	"github.com/temporalio/roadrunner-temporal/v6/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"go.temporal.io/sdk/worker"
)

const lambdaRuntimeAPIEnv = "AWS_LAMBDA_RUNTIME_API"

// StartInvocation and StopInvocation let the lambda plugin cycle the Temporal
// workers per invocation while the PHP pools stay up for the whole lifetime of
// the execution environment.
func (p *Plugin) StartInvocation(_ context.Context, graceful time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.startTemporalWorkersLocked(graceful)
}

func (p *Plugin) StopInvocation(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopTemporalWorkersLocked()

	return nil
}

// startTemporalWorkersLocked requires p.mu to be held.
func (p *Plugin) startTemporalWorkersLocked(graceful time.Duration) error {
	workers, err := aggregatedpool.TemporalWorkers(
		p.getWfDef(),
		p.getActDef(),
		cloneWorkerInfo(p.temporal.workerInfo, graceful),
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

	// Purge before dropping the references: the cache refcount falls on a GC
	// finalizer, and at zero the purge silently becomes a no-op, so the cached
	// workflows would never be destroyed on the PHP side.
	worker.PurgeStickyWorkflowCache()
	p.temporal.workers = nil
}

// cloneWorkerInfo keeps the stored worker info pristine: TemporalWorkers appends
// the resolved interceptors into Options, so reusing the same value every
// invocation would stack them up. It also supplies WorkerStopTimeout when the
// PHP worker left it unset, because at zero Stop() waits for nothing and the
// invocation would be acknowledged while an activity is suspended mid-call.
func cloneWorkerInfo(source []*internal.WorkerInfo, graceful time.Duration) []*internal.WorkerInfo {
	cloned := make([]*internal.WorkerInfo, 0, len(source))
	for i := range source {
		copied := *source[i]
		copied.Options.Interceptors = nil

		if copied.Options.WorkerStopTimeout == 0 {
			copied.Options.WorkerStopTimeout = graceful
		}

		cloned = append(cloned, &copied)
	}

	return cloned
}
