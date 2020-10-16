package subcommands

import (
	"github.com/spf13/cobra"
	"github.com/spiral/endure"
)

var (
	ConfDir string

	Container *endure.Endure

	rootCmd = &cobra.Command{
		Use:           "rrt",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
)
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// exit with error
		panic(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfDir, "config", "c", "plugins/temporal/.rr.yaml", "-c <path to .rr.yaml>")

	cobra.OnInitialize(func() {

	})
}
