package cli

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spiral/endure"
)

var (
	CfgFile, WorkDir string
	Container        endure.Container
	rootCmd          = &cobra.Command{
		Use:           "rr",
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
	rootCmd.PersistentFlags().StringVarP(&CfgFile, "config", "c", ".rr.yaml", "config file (default is .rr.yaml)")
	rootCmd.PersistentFlags().StringVarP(&WorkDir, "WorkDir", "w", "", "work directory")

	cobra.OnInitialize(func() {
		if CfgFile != "" {
			if absPath, err := filepath.Abs(CfgFile); err == nil {
				CfgFile = absPath

				// force working absPath related to config file
				if err := os.Chdir(filepath.Dir(absPath)); err != nil {
					panic(err)
				}
			}
		}

		if WorkDir != "" {
			if err := os.Chdir(WorkDir); err != nil {
				panic(err)
			}
		}
	})
}
