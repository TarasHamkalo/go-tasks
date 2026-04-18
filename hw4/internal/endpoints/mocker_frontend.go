package endpoints

import (
	"context"
	"errors"
	"http-mocker/internal/mocker"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type MockerFrontend struct {
	srv    *http.Server
	mocker *mocker.HttpMocker
	logger *zap.Logger
}

func NewInsecureMockerFrontend(mocker *mocker.HttpMocker, logger *zap.Logger) *MockerFrontend {
	return &MockerFrontend{
		srv: &http.Server{
			Addr:           ":8080",
			Handler:        nil,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1 << 20, // max. 1 MB
		},
		mocker: mocker,
		logger: logger,
	}
}

func (m *MockerFrontend) tracing(next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		r.WithContext(context.WithValue(r.Context(), "traceId", uuid.New().String()))
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(f)
}

func (m *MockerFrontend) logging(next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		m.logger.Info(
			"received request",
			zap.String("trace", r.Context().Value("traceId").(string)),
			zap.String("addr", r.RemoteAddr),
			zap.String("method", r.Method),
			zap.String("path", r.URL.EscapedPath()), // # TODO remove
			zap.String("url", r.URL.String()),
		)
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(f)
}

func (m *MockerFrontend) ListenAndServe() {
	go (func() {
		if err := m.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			m.logger.Debug("http server closed with error", zap.Error(err))
		}
	})()
}

func (m *MockerFrontend) Shutdown(ctx context.Context) error {
	return m.srv.Shutdown(ctx)
}

func (m *MockerFrontend) handle(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		m.logger.Error("failed to read request body", zap.Error(err))
		return
	}

	requestSpec := mocker.NewRequestSpec(
		r.URL.EscapedPath(), r.Method, r.URL.Query(), bodyBytes,
	)

	responseSpec, err := m.mocker.Serve(requestSpec)

	if err != nil {
		switch err {
		case mocker.ErrNoConfigurationExists:
			w.WriteHeader(http.StatusNotImplemented)
		case mocker.ErrMethodNotSupported:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case
			mocker.ErrSpecificationDiffers,
			mocker.ErrMethodNotRegistered,
			mocker.ErrPathNotRegistered:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	w.Write(responseSpec.Body())
	w.WriteHeader(responseSpec.StatusCode())
}
