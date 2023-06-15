package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/miragedebug/miragedebug/api/app"
	langadaptors "github.com/miragedebug/miragedebug/internal/lang-adaptors"
	"github.com/miragedebug/miragedebug/internal/lang-adaptors/golang"
	"github.com/miragedebug/miragedebug/internal/lang-adaptors/rust"
	"github.com/miragedebug/miragedebug/pkg/log"
	"github.com/miragedebug/miragedebug/pkg/shell"
)

func debugCmd() *cobra.Command {
	serverAddr := ""
	c := &cobra.Command{
		Use:   "debug",
		Short: "start debug",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetDebug()
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
			if err := startDebug(c, appName); err != nil {
				log.Fatalf("start debug failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	return c
}

func buildBinary(app_ *app.App) error {
	var langAdaptor langadaptors.LanguageAdaptor
	switch app_.ProgramType {
	case app.ProgramType_GO:
		langAdaptor = golang.NewGolangAdaptor()
	case app.ProgramType_RUST:
		langAdaptor = rust.NewRustAdaptor()
	default:
		return fmt.Errorf("program type %s not supported", app_.ProgramType)
	}
	cmd, err := langAdaptor.BuildCommand(app_)
	if err != nil {
		return err
	}
	commands := []string{
		fmt.Sprintf("cd %s", app_.LocalConfig.WorkingDir),
		cmd,
	}
	log.Debugf("build command: %s", strings.Join(commands, "\n"))
	out, errOut, err := shell.ExecuteCommands(context.Background(), commands)
	if len(out) > 0 {
		fmt.Fprintf(os.Stdout, "build out: %s\n", out)
	}
	if len(errOut) > 0 {
		fmt.Fprintf(os.Stderr, "build error: %s\n", errOut)
	}
	if err != nil {
		log.Errorf("build failed: %v", err)
		return err
	}
	log.Infof("build %s success", app_.LocalConfig.BuildOutput)
	return nil
}

func startDebug(client app.AppManagementClient, appName string) error {
	app_, err := client.GetApp(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return err
	}
	if app_.LocalConfig == nil {
		return fmt.Errorf("app %s local config not inited", appName)
	}
	s, err := client.InitAppRemote(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return fmt.Errorf("get app %s status failed: %v", appName, err)
	}
	if !s.Connected {
		return fmt.Errorf("app %s not connected", appName)
	}
	if err := buildBinary(app_); err != nil {
		return err
	}
	_, err = client.StartDebugging(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	return err
}
