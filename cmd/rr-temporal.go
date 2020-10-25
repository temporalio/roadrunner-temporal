package main

import (
	"github.com/temporalio/roadrunner-temporal/plugins/activity"
	"github.com/temporalio/roadrunner-temporal/plugins/temporal"
	"github.com/temporalio/roadrunner-temporal/plugins/workflow"
	"log"

	"github.com/spiral/roadrunner/v2/plugins/factory"
	"github.com/temporalio/roadrunner-temporal/cmd/cli"
)

func main() {
	err := cli.InitApp(
		&factory.App{},
		&factory.WFactory{},
		&temporal.Server{},
		&activity.Server{},
		&workflow.Server{},
	)

	if err != nil {
		log.Fatal(err)
		return
	}

	cli.Execute()
}
