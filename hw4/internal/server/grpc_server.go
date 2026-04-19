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

type GrpcServer struct {
	srv    *grpc.Server
	logger *zap.Logger
}

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

func (s *GrpcServer) WithServer(handler func(srv *grpc.Server)) {
	handler(s.srv)
}

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
