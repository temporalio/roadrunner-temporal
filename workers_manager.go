package rrtemporal

import (
	"context"
)

func (p *Plugin) AddWorker() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actP.AddWorker()
}

func (p *Plugin) RemoveWorker(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actP.RemoveWorker(ctx)
}
