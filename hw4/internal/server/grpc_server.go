package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// GrpcServer wrapper around grpc.Server to unify startup and shutdown logic.
type GrpcServer struct {
	srv    *grpc.Server
	logger *zap.Logger
}

// NewGrpcServer constructs new GrpcServer object with grpc.Server initialized
// with given set of interceptors and tls config.
func NewGrpcServer(
	tlsCfg *tls.Config,
	logger *zap.Logger,
	interceptor ...grpc.UnaryServerInterceptor,
) *GrpcServer {
	opts := []grpc.ServerOption{
		grpc.MaxSendMsgSize(1024 * 1024 * 10), // max 10 MB
		grpc.Creds(credentials.NewTLS(tlsCfg)),
		grpc.ChainUnaryInterceptor(interceptor...),
	}
	return &GrpcServer{
		srv:    grpc.NewServer(opts...),
		logger: logger,
	}
}

// WithServer helper to decouple service registration from grpc.Server
func (s *GrpcServer) WithServer(handler func(srv *grpc.Server)) {
	handler(s.srv)
}

// Serve attempts to listen to given port on localhost and start grpc.Server.
// IMPORTANT: server is started in new routine.
// NOTE: registers reflection for registered services.
func (s *GrpcServer) Serve(port int) error {
	reflection.Register(s.srv)
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	go (func() {
		if err = s.srv.Serve(lis); err != nil {
			s.logger.Error("failed to serve, server stopped", zap.Error(err))
		}
	})()

	s.logger.Info("server started", zap.Int("port", port))
	return nil
}

// Shutdown attempts to gracefully shutdown grpc.Server instance.
// Start go routine calling grpc.Server graceful shutdown
// and wait till either ctx timeouts or server is stopped.
func (s *GrpcServer) Shutdown(ctx context.Context) {
	serverStopped := make(chan struct{})
	go (func() {
		s.srv.GracefulStop()
		close(serverStopped)
	})()

	select {
	case <-ctx.Done():
		s.logger.Info("ctx timeout, calling server stop")
		s.srv.Stop()
	case <-serverStopped:
		s.logger.Info("server stopped gracefully")
	}
}
