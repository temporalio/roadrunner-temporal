package cli

import (
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spiral/roadrunner/v2/plugins/informer"
	"github.com/temporalio/roadrunner-temporal/cmd/util"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	interactive bool
	stopSignal  = make(chan os.Signal, 1)
)

func init() {
	workersCommand := &cobra.Command{
		Use:   "workers",
		Short: "Show information about active roadrunner workers",
		RunE:  workersHandler,
	}

	workersCommand.Flags().BoolVarP(
		&interactive,
		"interactive",
		"i",
		false,
		"render interactive workers table",
	)

	root.AddCommand(workersCommand)

	signal.Notify(stopSignal, syscall.SIGTERM)
	signal.Notify(stopSignal, syscall.SIGINT)
}

func workersHandler(cmd *cobra.Command, args []string) error {
	client, err := RPCClient()
	if err != nil {
		return err
	}
	defer func() {
		err := client.Close()
		if err != nil {
			Logger.Error(err.Error())
		}
	}()

	var plugins []string
	if len(args) != 0 {
		plugins = args
	} else {
		err = client.Call("informer.List", true, &plugins)
		if err != nil {
			return err
		}
	}

	if !interactive {
		return showWorkers(plugins, client)
	}

	tm.Clear()
	for {
		select {
		case <-stopSignal:
			return nil
		case <-time.NewTicker(time.Second).C:
			tm.MoveCursor(1, 1)
			showWorkers(plugins, client)
			tm.Flush()
		}
	}
}

func showWorkers(plugins []string, client *rpc.Client) error {
	for _, plugin := range plugins {
		list := &informer.WorkerList{}
		err := client.Call("informer.Workers", plugin, &list)
		if err != nil {
			return err
		}

		fmt.Printf("Workers of [%s]:\n", color.HiYellowString(plugin))
		util.WorkerTable(os.Stdout, list.Workers).Render()
	}

	return nil
}
