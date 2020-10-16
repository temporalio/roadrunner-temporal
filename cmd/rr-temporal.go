package main

import (
	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/cmd/subcommands"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

func main() {
	var err error
	subcommands.Container, err = endure.NewContainer(endure.DebugLevel, endure.RetryOnFail(false))
	if err != nil {
		panic(err)
	}

	err = subcommands.Container.Register(&temporal.Plugin{})
	if err != nil {
		panic(err)
	}

	conf := &config.ViperProvider{}
	conf.Path = subcommands.ConfDir
	conf.Prefix = "rr"

	err = subcommands.Container.Register(conf)
	if err != nil {
		panic(err)
	}

	err = subcommands.Container.Register(&factory.WFactory{})
	if err != nil {
		panic(err)
	}

	err = subcommands.Container.Register(&factory.App{})
	if err != nil {
		panic(err)
	}

	err = subcommands.Container.Init()
	if err != nil {
		panic(err)
	}

	// exec
	subcommands.Execute()
}
