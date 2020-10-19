package rpc

import (
	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
)

type Plugin struct {
	configProvider config.Provider
}


func (p *Plugin) Init() error {
	return nil
}


type RpcPlugin interface {
	endure.Named
	RpcService() interface{}
}