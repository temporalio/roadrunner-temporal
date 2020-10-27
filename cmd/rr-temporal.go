package main

import (
	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"
	"log"

	"github.com/temporalio/roadrunner-temporal/cmd/cli"
)

func main() {
	err := cli.InitApp(
		// todo: move to root
		&logger.ZapLogger{},

		// PHP application init.
		&app.App{},
		&rpc.Service{},

		// Temporal extension.
		&temporal.Server{},
		//&activity.Service{},
		&workflow.Service{},
	)

	if err != nil {
		log.Fatal(err)
		return
	}

	cli.Execute()
}
