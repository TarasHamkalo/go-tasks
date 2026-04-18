package main

import (
	"context"
	"errors"
	"http-mocker/internal"
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
	if err := os.Mkdir("logs", 0755); !errors.Is(err, os.ErrExist) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(AppLogFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Failed to create http server log, file=%s, err=%v", AppLogFilePath, err)
	}
	defer appLogFile.Close()

	appLogger := internal.LogInit(appLogFile, true)

	httpSrv := mocker.NewInsecureMockerFrontend(appLogger.With(zap.String("module", "frontend")))
	httpSrv.ListenAndServe()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); !errors.Is(err, http.ErrServerClosed) {
		appLogger.Error("Failed to shutdown http server", zap.Error(err))
	}
}
