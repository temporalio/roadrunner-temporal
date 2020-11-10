package main

import (
	"github.com/temporalio/roadrunner-temporal/plugins/resetter"
	"log"

	"github.com/spiral/roadrunner/v2/plugins/app"
	"github.com/spiral/roadrunner/v2/plugins/logger"
	"github.com/spiral/roadrunner/v2/plugins/rpc"
	"github.com/temporalio/roadrunner-temporal/plugins/activity"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"

	"github.com/temporalio/roadrunner-temporal/cmd/cli"
)

func main() {
	err := cli.InitApp(
		// todo: move to root
		&logger.ZapLogger{},

		// Helpers
		&resetter.Plugin{},

		// PHP application init.
		&app.Plugin{},
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
