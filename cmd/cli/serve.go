package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spiral/roadrunner/v2/plugins/config"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Serve RoadRunner Temporal service(s)",
		RunE:  handler,
	})
}

func handler(cmd *cobra.Command, args []string) error {
	/*
		We need to have path to the config at the Register stage
		But after cobra.Execute, because cobra fills up cli variables on this stage
	*/
	conf := &config.ViperProvider{}
	conf.Path = CfgFile
	conf.Prefix = "rr"

	err := Container.Register(conf)
	if err != nil {
		return err
	}

	err = Container.Init()
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
			fmt.Println(e.Error)
			fmt.Println(e.VertexID)
		case <-c:
			err = Container.Stop()
			if err != nil {
				return err
			}
			return nil
		}
	}
}
