package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/internal/ide-adapotors/jetbrains"
	"github.com/kebe7jun/miragedebug/pkg/log"
)

func initCmd() *cobra.Command {
	ideType := ""
	language := ""
	serverAddr := ""
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new project",
		Long:  "Initialize a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				log.Fatalf("please specify the project name")
				return nil
			}
			appName := args[0]
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("did not connect: %v", err)
			}
			defer conn.Close()
			c := app.NewAppManagementClient(conn)
			if err := initLocalConfig(c, appName); err != nil {
				log.Fatalf("init local config failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&ideType, "ide", "", "", "IDE type, currently only support golang")
	c.PersistentFlags().StringVarP(&language, "language", "l", "go", "Programming language, currently only support go")
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	return c
}

func initLocalConfig(client app.AppManagementClient, appName string) error {
	app_, err := client.GetApp(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return err
	}
	if app_.LocalConfig == nil {
		return fmt.Errorf("app %s local config not inited", appName)
	}
	// init remote
	s, err := client.InitAppRemote(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return err
	}
	switch app_.LocalConfig.IdeType {
	case app.IDEType_GOLAND:
		j := jetbrains.NewJetbrainsAdaptor()
		if err := j.PrepareLaunch(app_); err != nil {
			return err
		}
	default:
		return fmt.Errorf("ide type %s not supported", app_.LocalConfig.IdeType)
	}
	log.Debugf("remote init result: %s", s)
	return nil
}
