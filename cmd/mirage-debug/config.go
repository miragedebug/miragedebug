package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/internal/ide-adapotors/jetbrains"
	"github.com/miragedebug/miragedebug/internal/ide-adapotors/vscode"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func configCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Initialize project ide config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				log.Fatalf("please specify the project name")
				return nil
			}
			checkOrInitServerCommand()
			appName := args[0]
			conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	app_, err = client.GetApp(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return err
	}
	switch app_.LocalConfig.IdeType {
	case app.IDEType_GOLAND, app.IDEType_CLION:
		j := jetbrains.NewJetbrainsAdaptor()
		if err := j.PrepareLaunch(app_); err != nil {
			return err
		}
	case app.IDEType_VS_CODE:
		vs := vscode.NewVSCodeAdaptor()
		if err := vs.PrepareLaunch(app_); err != nil {
			return err
		}
	default:
		return fmt.Errorf("ide type %s not supported", app_.LocalConfig.IdeType)
	}
	log.Debugf("remote init result: %s", s)
	return nil
}
