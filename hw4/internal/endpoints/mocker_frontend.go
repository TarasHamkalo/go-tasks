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

type contextKey string

const traceIdKey contextKey = "traceId"

type MockerFrontend struct {
	srv    *http.Server
	mocker *mocker.HttpMocker
	logger *zap.Logger
}

func NewInsecureMockerFrontend(mocker *mocker.HttpMocker, logger *zap.Logger) *MockerFrontend {
	m := &MockerFrontend{
		mocker: mocker,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/", m.tracing(m.logging(http.HandlerFunc(m.handle))))

	m.srv = &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // max. 1 MB
	}

	return m
}

func (m *MockerFrontend) tracing(next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()[:8] // demo, trim for readability
		ctx := context.WithValue(r.Context(), traceIdKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(f)
}

func (m *MockerFrontend) logging(next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		m.logger.Info(
			"received request",
			zap.String("trace", r.Context().Value(traceIdKey).(string)),
			zap.String("addr", r.RemoteAddr),
			zap.String("method", r.Method),
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
	m.logger.Info("http server listening", zap.String("addr", m.srv.Addr))
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
	m.logger.Debug(
		"request body received",
		zap.String("trace", r.Context().Value(traceIdKey).(string)),
	)

	requestSpec := mocker.NewRequestSpec(
		r.URL.EscapedPath(), r.Method, r.URL.Query(), bodyBytes,
	)

	responseSpec, err := m.mocker.Serve(requestSpec)
	if err != nil {
		statusCode := http.StatusInternalServerError
		switch err {
		case mocker.ErrNoConfigurationExists:
			statusCode = http.StatusNotImplemented
		case mocker.ErrMethodNotSupported:
			statusCode = http.StatusMethodNotAllowed
		case
			mocker.ErrSpecificationDiffers,
			mocker.ErrMethodNotRegistered,
			mocker.ErrPathNotRegistered:
			statusCode = http.StatusNotFound
		default:
		}

		m.logger.Error(
			"failed to serve request",
			zap.String("trace", r.Context().Value(traceIdKey).(string)),
			zap.Int("statusCode", statusCode),
			zap.Error(err),
		)

		w.WriteHeader(statusCode)
	} else {
		m.logger.Info(
			"serve request",
			zap.String("trace", r.Context().Value(traceIdKey).(string)),
			zap.Int("statusCode", responseSpec.StatusCode()),
			// TODO: remove, should not be used normally, but for testing with small bodies, it is nice
			zap.ByteString("body", responseSpec.Body()),
			zap.Error(err),
		)

		w.WriteHeader(responseSpec.StatusCode())
		_, err := w.Write(responseSpec.Body())
		if err != nil {
			m.logger.Error("failed to write response", zap.Error(err))
		}
	}
}

func (m *MockerFrontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.srv.Handler.ServeHTTP(w, r)
}
