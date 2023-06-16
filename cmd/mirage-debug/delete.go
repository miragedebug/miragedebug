package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func deleteCmd() *cobra.Command {
	serverAddr := ""
	c := &cobra.Command{
		Use:   "delete",
		Short: "Delete a project",
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
			_, err = c.RollbackApp(context.Background(), &app.SingleAppRequest{
				Name: appName,
			})
			if err != nil {
				log.Fatalf("Rollback app failed: %v", err)
			}
			_, err = c.DeleteApp(context.Background(), &app.SingleAppRequest{
				Name: appName,
			})
			if err != nil {
				log.Fatalf("Delete app failed: %v", err)
				return nil
			}
			fmt.Println("Delete app success")
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	return c
}
