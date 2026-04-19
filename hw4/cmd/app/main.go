package main

import (
	"context"
	"crypto/tls"
	pb "http-mocker/generated"
	"http-mocker/internal"
	"http-mocker/internal/handler"
	"http-mocker/internal/middleware"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"http-mocker/internal/mocker"
	"http-mocker/internal/server"
)

const AppLogFilePath = "logs/mocker.log"

func main() {
	if err := os.Mkdir("logs", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(AppLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Failed to create app server log, file=%s, err=%v", AppLogFilePath, err)
	}
	defer appLogFile.Close()
	appLogger := internal.LogInitWithConsole(appLogFile, true)

	certPath, keyPath := "certs/server.crt", "certs/server.key"
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}

	if err != nil {
		log.Fatalf("Failed to load server certificate and key: %v", err)
	}

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
		&tlsCfg,
		grpcServerLogger,
		middleware.GrpcTracing(),
		middleware.GrpcLogging(grpcServerLogger),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterManagementServiceServer(srv, handler.NewManagementService(m))
	})

	if err = grpcServer.Serve(8081); err != nil {
		log.Fatalf("Failed to start grpc server: %v", err)
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
		go grpcServer.Shutdown(ctx)
	})()

	wg.Wait()
}

func setTestRoutes(m *mocker.HttpMocker) {
	//m.SetReply(
	//	mocker.NewRequestSpecBuilder("/users", "GET").Build(),
	//	mocker.NewResponseSpec(
	//		200,
	//		[]byte(`["user-1","user-2"]`),
	//	),
	//)
	//
	//m.SetReply(
	//	mocker.NewRequestSpecBuilder("/test", "POST").
	//		WithBody([]byte("aaa")).
	//		Build(),
	//	mocker.NewResponseSpec(200, []byte("ok")),
	//)
	//
	//m.SetReply(
	//	mocker.NewRequestSpecBuilder("/test/empty", "POST").
	//		WithBody([]byte("")).
	//		Build(),
	//	mocker.NewResponseSpec(200, []byte("ok")),
	//)
	//
	//m.SetReply(
	//	mocker.NewRequestSpecBuilder("/test/nil", "POST").Build(),
	//	mocker.NewResponseSpec(200, []byte("ok")),
	//)
}
