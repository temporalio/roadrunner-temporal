package rrtemporal

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v6/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"go.temporal.io/sdk/worker"
)

const (
	lambdaRuntimeAPIEnv = "AWS_LAMBDA_RUNTIME_API"
	minimumRunTime      = time.Second
)

func (p *Plugin) serveLambda() {
	p.log.Info("serving the lambda runtime api",
		"shutdown_buffer", p.config.Lambda.ShutdownBuffer.String(),
		"graceful_timeout", p.config.Lambda.GracefulTimeout.String(),
	)

	lambda.Start(p.handleInvocation)
}

func (p *Plugin) handleInvocation(ctx context.Context) error {
	const op = errors.Op("temporal_lambda_invocation")

	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.E(op, errors.Str("the Runtime API did not provide an invocation deadline"))
	}

	stopAt := deadline.Add(-p.config.Lambda.ShutdownBuffer)

	runFor := time.Until(stopAt)
	if runFor < minimumRunTime {
		return errors.E(op, errors.Errorf(
			"insufficient invocation time: %s left after reserving a %s shutdown buffer",
			runFor, p.config.Lambda.ShutdownBuffer,
		))
	}

	if err := p.startInvocation(); err != nil {
		return errors.E(op, err)
	}

	p.log.Info("invocation started", "polling_for", runFor.String())

	waitUntil(ctx, stopAt)
	p.stopInvocation()

	p.log.Info("invocation finished", "remaining", time.Until(deadline).String())

	return nil
}

func waitUntil(ctx context.Context, stopAt time.Time) {
	timer := time.NewTimer(time.Until(stopAt))
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func (p *Plugin) startInvocation() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.startTemporalWorkersLocked(p.config.Lambda.GracefulTimeout)
}

func (p *Plugin) stopInvocation() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopTemporalWorkersLocked()
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

// TemporalWorkers appends the resolved interceptors into Options, so the stored
// value must not be reused across invocations. WorkerStopTimeout at zero makes
// Stop() wait for nothing, which acknowledges the invocation mid-activity.
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
