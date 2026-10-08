package rrtemporal

import (
	"net/http"
	"slices"

	"github.com/roadrunner-server/pool/v2/fsm"
	"github.com/roadrunner-server/pool/v2/worker"

	"github.com/roadrunner-server/api-plugins/v6/status"
)

// Status return status of the particular plugin
func (p *Plugin) Status() (*status.Status, error) {
	return p.workersStatus(func(w *worker.Process) bool { return w.State().IsActive() }), nil
}

// Ready return readiness status of the particular plugin
func (p *Plugin) Ready() (*status.Status, error) {
	return p.workersStatus(func(w *worker.Process) bool { return w.State().Compare(fsm.StateReady) }), nil
}

// workersStatus returns 200 when at least one worker matches ok, otherwise 503.
func (p *Plugin) workersStatus(ok func(*worker.Process) bool) *status.Status {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config.DisableActivityWorkers {
		if wf := p.wfP.Workers(); len(wf) > 0 && ok(wf[0]) {
			return &status.Status{Code: http.StatusOK}
		}
	}

	if slices.ContainsFunc(p.actP.Workers(), ok) {
		return &status.Status{Code: http.StatusOK}
	}

	return &status.Status{Code: http.StatusServiceUnavailable}
}
