package main

import (
	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/config"
	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/cmd/subcommands"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
)

func main() {
	/*
		1. -c flag - path to the config (default .rr.yaml
		2. change working dir to the path with .rr.yaml
		3. init endure here, path should be -c
		4. command to serve endure container
		5. plugins to use - temporal, config, factory
		6. USE COBRA!!!!!11111oneoneone
	*/

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
