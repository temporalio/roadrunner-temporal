package rrtemporal

import (
	"time"

	"github.com/roadrunner-server/errors"
	"github.com/temporalio/roadrunner-temporal/v6/aggregatedpool"
	"github.com/temporalio/roadrunner-temporal/v6/internal"
	"github.com/temporalio/roadrunner-temporal/v6/internal/lambda"
	"go.temporal.io/sdk/worker"
)

const (
	lambdaRuntimeAPIEnv = "AWS_LAMBDA_RUNTIME_API"
	lambdaPollInterval  = time.Millisecond * 100
	lambdaMinimumRun    = time.Second
)

// serveLambda replaces the long-lived worker loop with one burst of polling per
// Lambda invocation. The PHP pools stay up for the whole lifetime of the
// execution environment; only the Temporal workers are cycled, so no PHP
// process is restarted between invocations.
func (p *Plugin) serveLambda(errCh chan error, host string) {
	api := lambda.NewRuntimeAPI(host)

	go func() {
		for {
			select {
			case <-p.stopCh:
				return
			default:
			}

			invocation, err := api.NextInvocation()
			if err != nil {
				errCh <- errors.E(errors.Op("temporal_lambda_serve"), err)
				return
			}

			invocationErr := p.runInvocation(invocation)
			if invocationErr != nil {
				p.log.Error("invocation failed", "requestID", invocation.RequestID, "error", invocationErr)
			}

			p.acknowledge(api, invocation, invocationErr)
		}
	}()
}

func (p *Plugin) runInvocation(invocation *lambda.Invocation) error {
	stopAt := invocation.Deadline.Add(-p.config.Lambda.ShutdownBuffer)

	runFor := time.Until(stopAt)
	if runFor < lambdaMinimumRun {
		return errors.Errorf(
			"insufficient invocation time: %s left to poll after reserving a %s shutdown buffer",
			runFor, p.config.Lambda.ShutdownBuffer,
		)
	}

	if err := p.startTemporalWorkers(); err != nil {
		return err
	}

	p.log.Info("workers started", "requestID", invocation.RequestID, "polling_for", runFor.String())

	p.pollUntil(stopAt)
	p.stopTemporalWorkers()

	p.log.Info("workers stopped", "requestID", invocation.RequestID, "remaining", time.Until(invocation.Deadline).String())

	return nil
}

func (p *Plugin) pollUntil(stopAt time.Time) {
	for time.Now().Before(stopAt) {
		select {
		case <-p.stopCh:
			return
		case <-time.After(lambdaPollInterval):
		}
	}
}

func (p *Plugin) startTemporalWorkers() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.startTemporalWorkersLocked()
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

func (p *Plugin) stopTemporalWorkers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopTemporalWorkersLocked()
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

func (p *Plugin) acknowledge(api *lambda.RuntimeAPI, invocation *lambda.Invocation, invocationErr error) {
	var err error
	if invocationErr == nil {
		err = api.Respond(invocation.RequestID)
	} else {
		err = api.ReportInvocationError(invocation.RequestID, invocationErr)
	}

	if err != nil {
		p.log.Error("acknowledgement failed", "requestID", invocation.RequestID, "error", err)
	}
}
