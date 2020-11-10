package cli

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	root.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset workers of all or specific RoadRunner service",
		RunE:  resetHandler,
	})
}

func resetHandler(cmd *cobra.Command, args []string) error {
	client, err := RPCClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var list []string
	err = client.Call("resetter.List", true, &list)
	if err != nil {
		return err
	}

	fmt.Printf("Resetting [%s]... ", color.YellowString(strings.Join(list, " ")))

	err = client.Call("resetter.ResetAll", true, &list)
	if err != nil {
		fmt.Println(color.RedString(err.Error()))
		return err
	}

	fmt.Println(color.GreenString("done"))

	return nil
}
