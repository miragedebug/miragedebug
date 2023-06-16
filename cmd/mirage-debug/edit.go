package main

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func editCmd() *cobra.Command {
	serverAddr := ""
	c := &cobra.Command{
		Use:   "edit",
		Short: "Edit project config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				log.Fatalf("please specify the project name")
				return nil
			}
			appName := args[0]
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("did not connect: %v", err)
				return nil
			}
			defer conn.Close()
			c := app.NewAppManagementClient(conn)
			if err := editConfig(c, appName); err != nil {
				log.Fatalf("edit app config failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	return c
}

func editConfig(client app.AppManagementClient, appName string) error {
	app_, err := client.GetApp(context.Background(), &app.SingleAppRequest{
		Name: appName,
	})
	if err != nil {
		return err
	}
	bs, err := yaml.Marshal(app_)
	if err != nil {
		return err
	}
	qs := []*survey.Question{
		{
			Name: "config",
			Prompt: &survey.Editor{
				Message:       "Edit config:",
				Default:       string(bs),
				AppendDefault: true,
			},
		},
	}
	answers := struct {
		Config string `survey:"config"`
	}{}
	err = survey.Ask(qs, &answers)
	if err != nil {
		return err
	}
	app_ = &app.App{}
	err = yaml.Unmarshal([]byte(answers.Config), app_)
	if app_.Name != appName {
		return fmt.Errorf("app name can not be changed")
	}
	_, err = client.UpdateApp(context.Background(), app_)
	return err
}
