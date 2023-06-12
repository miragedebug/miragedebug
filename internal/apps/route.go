package apps

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/kebe7jun/miragedebug/api/app"
)

func RegisterHTTPRoutes() []func(ctx context.Context, serveMux *runtime.ServeMux, clientConn *grpc.ClientConn) error {
	return []func(ctx context.Context, serveMux *runtime.ServeMux, clientConn *grpc.ClientConn) error{
		app.RegisterAppManagementHandler,
	}
}

func RegisterGRPCRoutes(s *grpc.Server) {
	am := &appManagement{}
	am.init()
	app.RegisterAppManagementServer(s, am)
	reflection.Register(s)
}
