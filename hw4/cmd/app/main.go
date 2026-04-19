package main

import (
	"context"
	"http-mocker/internal"
	"http-mocker/internal/endpoints"
	"http-mocker/internal/mocker"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const AppLogFilePath = "logs/mocker.log"

func main() {
	if err := os.Mkdir("logs", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(AppLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Failed to create http server log, file=%s, err=%v", AppLogFilePath, err)
	}
	defer appLogFile.Close()

	appLogger := internal.LogInitWithConsole(appLogFile, true)

	m := mocker.NewHttpMocker()
	setTestRoutes(m)

	mockedSrv := endpoints.NewMockedHttpServer(
		m,
		appLogger.With(zap.String("module", "insecure-frontend")),
	)

	//certPath, keyPath := "certs/server.crt", "certs/server.key"
	mockedSrv.ListenAndServe()
	//mockedSrv.ListenAndServeTLS(certPath, keyPath)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	appLogger.Info("Received os signal, starting graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	mockedSrv.Shutdown(ctx)
}

func setTestRoutes(m *mocker.HttpMocker) {
	m.SetReply(
		mocker.NewRequestSpecBuilder("/users", "GET").Build(),
		mocker.NewResponseSpec(
			200,
			[]byte(`["user-1","user-2"]`),
		),
	)

	m.SetReply(
		mocker.NewRequestSpecBuilder("/test", "POST").
			WithBody([]byte("aaa")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	m.SetReply(
		mocker.NewRequestSpecBuilder("/test/empty", "POST").
			WithBody([]byte("")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	m.SetReply(
		mocker.NewRequestSpecBuilder("/test/nil", "POST").Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)
}
