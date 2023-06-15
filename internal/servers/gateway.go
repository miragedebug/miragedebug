package servers

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/slok/go-http-metrics/middleware/std"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/miragedebug/miragedebug/pkg/log"
)

type GatewayServer struct {
	service    string
	ctx        context.Context
	listenAddr string
	grpcAddr   string
	handlers   []func(ctx context.Context, serveMux *runtime.ServeMux, clientConn *grpc.ClientConn) error
}

func (g *GatewayServer) Run() error {
	ctx, cancel := context.WithCancel(g.ctx)
	defer cancel()

	conn, err := grpc.DialContext(ctx, g.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			log.Errorf("Failed to close a client connection to the gPRC server: %v", err)
		}
	}()

	gw, err := NewGateway(ctx, conn, g.handlers...)
	if err != nil {
		return err
	}

	router := mux.NewRouter()
	router.PathPrefix("/").Handler(gw)
	router.Use(otelmux.Middleware(g.service))

	// Create http metrics  middleware.
	// ref: https://github.com/slok/go-http-metrics
	httpMetricMiddleware := middleware.New(middleware.Config{
		Recorder: prometheus.NewRecorder(prometheus.Config{
			Prefix: "mirage_",
		}),
		Service:                g.service,
		GroupedStatus:          false,
		DisableMeasureSize:     false,
		DisableMeasureInflight: false,
	})
	// Wrap our main handler, we pass empty handler ID so the middleware inferes
	// the handler label from the URL.
	h := std.Handler("", httpMetricMiddleware, router)

	s := &http.Server{
		Addr:    g.listenAddr,
		Handler: h,
	}
	go func() {
		<-ctx.Done()
		log.Infof("Shutting down the http server")
		if err := s.Shutdown(context.Background()); err != nil {
			log.Errorf("Failed to shutdown http server: %v", err)
		}
	}()
	log.Infof("Starting listening at %s", g.listenAddr)
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Errorf("Failed to listen and serve: %v", err)
		return err
	}
	return nil
}

func NewGateway(ctx context.Context, conn *grpc.ClientConn,
	registerFuncs ...func(ctx context.Context, serveMux *runtime.ServeMux, clientConn *grpc.ClientConn) error) (http.Handler, error) {
	gw := runtime.NewServeMux(
		runtime.WithErrorHandler(runtime.DefaultHTTPErrorHandler),
	)
	for _, f := range registerFuncs {
		if err := f(ctx, gw, conn); err != nil {
			return nil, err
		}
	}
	return gw, nil
}

func NewGatewayServer(service string, listenAddr, grpcAddr string,
	handlers []func(ctx context.Context, serveMux *runtime.ServeMux, clientConn *grpc.ClientConn) error) *GatewayServer {
	gw := &GatewayServer{
		service:    service,
		ctx:        context.Background(),
		listenAddr: listenAddr,
		grpcAddr:   grpcAddr,
		handlers:   handlers,
	}
	return gw
}
