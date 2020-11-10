package cli

import (
	"github.com/spf13/cobra"
	"log"
)

func init() {
	root.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset workers of all or specific RoadRunner service",
		RunE:  reloadHandler,
	})
}

func reloadHandler(cmd *cobra.Command, args []string) error {
	client, err := RPCClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var r []string
	err = client.Call("resetter.List", true, &r)
	if err != nil {
		return err
	}

	log.Print(r)

	return nil
}
