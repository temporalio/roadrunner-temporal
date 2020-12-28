package main

import (
	"log"

	"github.com/spiral/endure"
	"github.com/spiral/roadrunner/v2/plugins/informer"
	"github.com/spiral/roadrunner/v2/plugins/resetter"

	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/spiral/roadrunner/v2/plugins/server"
	"github.com/temporalio/roadrunner-temporal/plugins/activity"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"

	"github.com/temporalio/roadrunner-temporal/cmd/cli"
)

func main() {
	var err error
	cli.Container, err = endure.NewContainer(cli.InitLogger(), endure.RetryOnFail(false))
	if err != nil {
		log.Fatal(err)
	}

	err = cli.Container.RegisterAll(
		// todo: move to root
		&logger.ZapLogger{},

		// Helpers
		&resetter.Plugin{},
		&informer.Plugin{},

		// PHP application init.
		&server.Plugin{},
		&rpc.Plugin{},

		// Temporal extension.
		&temporal.Plugin{},
		&activity.Plugin{},
		&workflow.Plugin{},
	)

	if err != nil {
		log.Fatal(err)
		return
	}

	cli.Execute()
}
