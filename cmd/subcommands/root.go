package subcommands

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spiral/endure"
)

var (
	ConfDir string

	Container endure.Container

	rootCmd = &cobra.Command{
		Use:           "rrt",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// exit with error, fatal invoke os.Exit(1)
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfDir, "config", "c", "", "config file (default is .rr.yaml)")

	cobra.OnInitialize(func() {
		// chdir
	})
}
