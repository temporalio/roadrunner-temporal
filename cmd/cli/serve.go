package cli

import (
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start RoadRunner Temporal service(s)",
		RunE:  handler,
	})
}

func handler(cmd *cobra.Command, args []string) error {
	/*
		We need to have path to the config at the Register stage
		But after cobra.Execute, because cobra fills up cli variables on this stage
	*/
	err := Container.Init()
	if err != nil {
		return err
	}

	errCh, err := Container.Serve()
	if err != nil {
		return err
	}

	// https://golang.org/pkg/os/signal/#Notify
	// should be of buffer size at least 1
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case e := <-errCh:
			Logger.Error(e.Error.Error(), zap.String("service", e.VertexID))
			os.Exit(1)
			er := Container.Stop()
			if er != nil {
				Logger.Error(e.Error.Error(), zap.String("service", e.VertexID))
				if er != nil {
					panic(er)
				}
			}
		case <-c:
			os.Exit(1)
			err = Container.Stop()
			if err != nil {
				return err
			}
			return nil
		}
	}
}
