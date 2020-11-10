package resetter

import (
	"github.com/spiral/endure"
	"log"
)

const ServiceName = "resetter"

type Resetter interface {
	endure.Named
	Reset() error
}

type Plugin struct {
	registry map[string]Resetter
}

func (p *Plugin) Init() error {
	return nil
}

// Register resettable service.
func (p *Plugin) Register(r Resetter) error {
	log.Print(r.Name())
	return nil
}

// Collects declares services to be collected.
func (p *Plugin) Collects() []interface{} {
	return []interface{}{
		p.Register,
	}
}

// Name of the service.
func (p *Plugin) Name() string {
	return ServiceName
}

// RPCService returns associated rpc service.
func (p *Plugin) RPCService() (interface{}, error) {
	return &rpc{srv: p}, nil
}

// todo: rpc
