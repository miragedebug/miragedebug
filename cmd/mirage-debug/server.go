package main

import (
	"context"
	"fmt"
	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/internal/apps"
	"github.com/miragedebug/miragedebug/internal/servers"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func serverInfoCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "info",
		Short: "Print server info",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			checkOrInitServerCommand()
			conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				panic(err)
			}
			c := app.NewAppManagementClient(conn)
			si, err := c.GetServerInfo(context.Background(), &app.Empty{})
			if err != nil {
				panic(err)
			}
			bs, _ := si.MarshalJSON()
			fmt.Printf("%s\n", bs)
			return nil
		},
	}
	return root
}

func serverStopCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "stop",
		Short: "Print server info",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				fmt.Printf("server maybe stopped\n")
				return nil
			}
			c := app.NewAppManagementClient(conn)
			si, err := c.GetServerInfo(context.Background(), &app.Empty{})
			if err != nil {
				fmt.Printf("server maybe stopped\n")
				return nil
			}
			syscall.Kill(int(si.Pid), 9)
			return nil
		},
	}
	return root
}

func serverCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "server",
		Short: "Start mirage-debug server",
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
	root.AddCommand(serverInfoCmd())
	root.AddCommand(serverStopCmd())
	return root
}

func checkOrInitServerCommand() {
	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err == nil {
		defer conn.Close()
		c := app.NewAppManagementClient(conn)
		_, err = c.GetServerInfo(context.Background(), &app.Empty{})
		if err == nil {
			return
		}
	}
	// not exists
	args := []string{os.Args[0], "server",
		"--config", configRoot,
		"--kubeconfig", kubeconfig,
		"--http-addr", httpAddr,
		"--grpc-addr", grpcAddr,
		fmt.Sprintf("--debug=%v", debug)}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("exec %s 2>&1 >/tmp/mirage-debug.logs", strings.Join(args, " ")))
	//cmd := exec.Command(os.Args[0], "server",
	//	"--config", configRoot,
	//	"--kubeconfig", kubeconfig,
	//	"--http-addr", httpAddr,
	//	"--grpc-addr", grpcAddr,
	//	fmt.Sprintf("--debug=%v", debug))
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 2)
}
