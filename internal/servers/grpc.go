package servers

import (
	"net"

	"google.golang.org/grpc"

	"github.com/miragedebug/miragedebug/internal/servers/route"
)

type GrpcServer struct {
	LogLevel   string
	ListenAddr string
	routes     route.GRPCRouterRegister
	server     *grpc.Server
}

func NewGRPCServer(listenAddr string, routes route.GRPCRouterRegister) *GrpcServer {
	ser := GrpcServer{
		ListenAddr: listenAddr,
		routes:     routes,
		server:     grpc.NewServer(),
	}
	return &ser
}

func (m *GrpcServer) Run() error {
	m.routes(m.server)
	l, err := net.Listen("tcp", m.ListenAddr)
	if err != nil {
		return err
	}
	return m.server.Serve(l)
}

func (m *GrpcServer) Stop() error {
	m.server.GracefulStop()
	return nil
}
