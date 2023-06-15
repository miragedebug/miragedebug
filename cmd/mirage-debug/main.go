package main

import (
	"github.com/spf13/cobra"

	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func main() {
	configRoot := ""
	debug := false
	root := &cobra.Command{
		Use: "mirage-debug",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			config.SetConfigRootPath(configRoot)
			if debug {
				log.SetDebug()
			}
			return nil
		},
	}
	root.PersistentFlags().StringVarP(&configRoot, "config", "c", "~/.mirage", "Config root path")
	root.PersistentFlags().BoolVarP(&debug, "debug", "", true, "Enable debug config")
	root.AddCommand(configCmd())
	root.AddCommand(debugCmd())
	root.AddCommand(serverCmd())
	if err := root.Execute(); err != nil {
		panic(err)
	}
}
