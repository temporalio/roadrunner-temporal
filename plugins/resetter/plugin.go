package resetter

import (
	"github.com/spiral/endure"
	"github.com/spiral/errors"
)

const PluginName = "resetter"

type Resetter interface {
	Reset() error
}

type Plugin struct {
	registry map[string]Resetter
}

func (p *Plugin) Init() error {
	p.registry = make(map[string]Resetter)
	return nil
}

// Reset named service.
func (p *Plugin) Reset(name string) error {
	svc, ok := p.registry[name]
	if !ok {
		return errors.E("no such service", errors.Str(name))
	}

	return svc.Reset()
}

// RegisterTarget resettable service.
func (p *Plugin) RegisterTarget(name endure.Named, r Resetter) error {
	p.registry[name.Name()] = r
	return nil
}

// Collects declares services to be collected.
func (p *Plugin) Collects() []interface{} {
	return []interface{}{
		p.RegisterTarget,
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return PluginName
}

// RPCService returns associated rpc service.
func (p *Plugin) RPC() interface{} {
	return &rpc{srv: p}
}
