package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func getCmd() *cobra.Command {
	serverAddr := ""
	format := ""
	all := false
	c := &cobra.Command{
		Use:   "get",
		Short: "Get a project config",
		Example: `
	mirage-debug get --all
	mirage-debug get a b c
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (all && len(args) > 0) || (!all && len(args) == 0) {
				log.Fatalf("please specify either --all or app names")
				return nil
			}
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("did not connect: %v", err)
				return nil
			}
			defer conn.Close()
			c := app.NewAppManagementClient(conn)
			var apps []*app.App
			if !all {
				for _, a := range args {
					app_, err := c.GetApp(context.Background(), &app.SingleAppRequest{
						Name: a,
					})
					if err != nil {
						log.Fatalf("get app failed: %v", err)
						return nil
					}
					apps = append(apps, app_)
				}
			} else {
				apps_, err := c.ListApps(context.Background(), &app.Empty{})
				if err != nil {
					log.Fatalf("list apps failed: %v", err)
					return nil
				}
				apps = apps_.Apps
			}
			if err := printApps(apps, format); err != nil {
				log.Fatalf("print apps failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")
	c.PersistentFlags().StringVarP(&format, "format", "o", "table", "Output format, table or yaml")
	c.PersistentFlags().BoolVarP(&all, "all", "a", false, "List all apps")

	return c
}

func printApps(apps []*app.App, format string) error {
	switch format {
	case "table":
		tableWriter := "%-20s%-10s%-10s%-50s\n"
		fmt.Printf(tableWriter, "NAME", "LANGUAGE", "IDE", "WORKDIR")
		for _, a := range apps {
			fmt.Printf(tableWriter, a.Name, a.ProgramType, a.LocalConfig.IdeType, a.LocalConfig.WorkingDir)
		}
	case "yaml":
		for _, a := range apps {
			bs, _ := yaml.Marshal(a)
			fmt.Println(string(bs))
			fmt.Println("---")
		}
	default:
		return fmt.Errorf("format %s not supported", format)
	}
	return nil
}
