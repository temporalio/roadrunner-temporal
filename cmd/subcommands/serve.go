package subcommands

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Serve RoadRunner Temporal service(s)",
		RunE:  handler,
	})
}

func handler(cmd *cobra.Command, args []string) error {
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
