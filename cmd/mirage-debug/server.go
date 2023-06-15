package main

import (
	"github.com/spf13/cobra"

	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/internal/apps"
	"github.com/miragedebug/miragedebug/internal/servers"
)

func serverCmd() *cobra.Command {
	httpAddr := ""
	grpcAddr := ""
	kubeconfig := ""
	root := &cobra.Command{
		Use: "server",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			config.SetKubeconfig(kubeconfig)
			grpcServer := servers.NewGRPCServer(grpcAddr, apps.RegisterGRPCRoutes)
			go grpcServer.Run()
			gwServer := servers.NewGatewayServer("mirage debug server", httpAddr, grpcAddr, apps.RegisterHTTPRoutes())
			return gwServer.Run()
		},
	}
	root.PersistentFlags().StringVarP(&httpAddr, "http-addr", "", ":38080", "HTTP listen address.")
	root.PersistentFlags().StringVarP(&grpcAddr, "grpc-addr", "", ":38081", "GRPC listen address.")
	root.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", "~/.kube/config", "Kubeconfig file path.")
	return root
}
