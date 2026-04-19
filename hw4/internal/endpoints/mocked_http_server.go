package endpoints

import (
	"context"
	"errors"
	"http-mocker/internal/mocker"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const traceIdKey contextKey = "traceId"

type MockedHttpServer struct {
	httpSrv  *http.Server
	httpsSrv *http.Server
	mocker   *mocker.HttpMocker
	logger   *zap.Logger
}

func NewMockedHttpServer(mocker *mocker.HttpMocker, logger *zap.Logger) *MockedHttpServer {
	m := &MockedHttpServer{
		mocker: mocker,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/", m.tracing(m.logging(http.HandlerFunc(m.handle))))
	m.httpsSrv = &http.Server{
		Addr:           ":8443",
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // max. 1 MB
	}

	m.httpSrv = &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // max. 1 MB
	}

	return m
}

func (m *MockedHttpServer) tracing(next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()[:8] // demo, trim for readability
		ctx := context.WithValue(r.Context(), traceIdKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(f)
}

func (m *MockedHttpServer) logging(next http.Handler) http.Handler {
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

func (m *MockedHttpServer) ListenAndServe() {
	go (func() {
		if err := m.httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			m.logger.Debug("http server closed with error", zap.Error(err))
		}
	})()
	m.logger.Info("http server listening", zap.String("addr", m.httpSrv.Addr))
}

func (m *MockedHttpServer) ListenAndServeTLS(certFile string, keyFile string) {
	go func() {
		err := m.httpsSrv.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			m.logger.Error("https server error", zap.Error(err))
		}
	}()
	m.logger.Info("https server listening", zap.String("addr", m.httpsSrv.Addr))
}

func (m *MockedHttpServer) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	shutdown := func(srv *http.Server, isTLS bool) {
		defer wg.Done()
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.logger.Error("server shutdown error",
				zap.Bool("tls", isTLS),
				zap.Error(err),
			)
		} else {
			m.logger.Info("server shutdown success",
				zap.Bool("tls", isTLS),
			)
		}
	}

	go shutdown(m.httpSrv, false)
	go shutdown(m.httpsSrv, true)

	wg.Wait()
}

func (m *MockedHttpServer) handle(w http.ResponseWriter, r *http.Request) {
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
