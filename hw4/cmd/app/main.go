package main

import (
	"context"
	"errors"
	"http-mocker/internal"
	"http-mocker/internal/endpoints"
	"http-mocker/internal/mocker"
	"log"
	"net/http"
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

	httpSrv := endpoints.NewInsecureMockerFrontend(
		m,
		appLogger.With(zap.String("module", "insecure-frontend")),
	)

	httpSrv.ListenAndServe()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	appLogger.Info("Received os signal, starting graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); !errors.Is(err, http.ErrServerClosed) {
		appLogger.Error("Failed to shutdown http server", zap.Error(err))
	}
}

func setTestRoutes(m *mocker.HttpMocker) {
	m.SetRoute(
		mocker.NewRequestSpecBuilder("/users", "GET").Build(),
		mocker.NewResponseSpec(
			200,
			[]byte(`["user-1","user-2"]`),
		),
	)

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test", "POST").
			WithBody([]byte("aaa")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test/empty", "POST").
			WithBody([]byte("")).
			Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)

	m.SetRoute(
		mocker.NewRequestSpecBuilder("/test/nil", "POST").Build(),
		mocker.NewResponseSpec(200, []byte("ok")),
	)
}
