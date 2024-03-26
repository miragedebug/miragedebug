package main

import (
	"github.com/spf13/cobra"

	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func main() {
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
	root.PersistentFlags().StringVarP(&httpAddr, "http-addr", "", ":38080", "HTTP listen address.")
	root.PersistentFlags().StringVarP(&grpcAddr, "grpc-addr", "", ":38081", "GRPC listen address.")
	root.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", "~/.kube/config", "Kubeconfig file path.")
	root.AddCommand(configCmd())
	root.AddCommand(debugCmd())
	root.AddCommand(serverCmd())
	root.AddCommand(initCmd())
	root.AddCommand(editCmd())
	root.AddCommand(getCmd())
	root.AddCommand(deleteCmd())
	if err := root.Execute(); err != nil {
		panic(err)
	}
}
