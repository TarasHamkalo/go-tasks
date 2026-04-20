package main

import (
	"context"
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"http-mocker/internal"
	"http-mocker/internal/handler"
	"http-mocker/internal/middleware"
	"http-mocker/internal/server"
	"http-mocker/pkg/mocker"

	pb "http-mocker/generated"
)

const AppLogFilePath = "logs/mocker.log"

func main() {
	certPath, keyPath := "certs/server.crt", "certs/server.key"
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}

	if err != nil {
		log.Fatalf("Failed to load server certificate and key: %v", err)
	}

	appLogFile := createLogFile()
	defer appLogFile.Close()

	appLogger := internal.LogInitWithConsole(appLogFile, true)

	httpServer, grpcServer := setupServers(appLogger, &tlsCfg)
	if err = grpcServer.Serve(8081); err != nil {
		log.Fatalf("Failed to start GRPC server: %v", err)
	}

	httpServer.Run(":8080", ":8443", certPath, keyPath)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	appLogger.Info("Received os signal, starting graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go (func() {
		defer wg.Done()
		httpServer.Shutdown(ctx)
	})()
	go (func() {
		defer wg.Done()
		grpcServer.Shutdown(ctx)
	})()

	wg.Wait()
}

func createLogFile() *os.File {
	if err := os.Mkdir("logs", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(
		AppLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)

	if err != nil {
		log.Fatalf(
			"Failed to create app server log, file=%s, err=%v",
			AppLogFilePath,
			err,
		)
	}

	return appLogFile
}

func setupServers(
	appLogger *zap.Logger,
	tlsCfg *tls.Config,
) (*server.HttpServer, *server.GrpcServer) {
	m := mocker.NewHttpMocker()
	httpServerLogger := appLogger.With(zap.String("module", "http-server"))
	httpServer := server.NewHttpServer(
		middleware.HttpTracing(
			middleware.HttpLogging(httpServerLogger,
				handler.NewMockHttpHandler(m, httpServerLogger))),
		httpServerLogger,
	)

	grpcServerLogger := appLogger.With(zap.String("module", "grpc-server"))
	grpcServer := server.NewGrpcServer(
		tlsCfg,
		grpcServerLogger,
		middleware.GrpcTracing(),
		middleware.GrpcLogging(grpcServerLogger),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterManagementServiceServer(srv, handler.NewManagementService(m))
	})

	return httpServer, grpcServer
}
