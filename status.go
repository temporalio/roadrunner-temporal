package rrtemporal

import (
	"net/http"
	"slices"

	"github.com/roadrunner-server/pool/v2/worker"

	"github.com/roadrunner-server/api-plugins/v6/status"
)

// Status returns 200 when at least one worker is active, otherwise 503.
func (p *Plugin) Status() (*status.Status, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	isActive := func(w *worker.Process) bool { return w.State().IsActive() }

	if p.config != nil && p.config.DisableActivityWorkers && p.wfP != nil {
		if wf := p.wfP.Workers(); len(wf) > 0 && isActive(wf[0]) {
			return &status.Status{Code: http.StatusOK}, nil
		}
	}

	if p.actP != nil && slices.ContainsFunc(p.actP.Workers(), isActive) {
		return &status.Status{Code: http.StatusOK}, nil
	}

	return &status.Status{Code: http.StatusServiceUnavailable}, nil
}

// Ready returns 200 when at least one worker is active, otherwise 503.
func (p *Plugin) Ready() (*status.Status, error) {
	return p.Status()
}
