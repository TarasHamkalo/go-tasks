package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HttpServer wrapper around http.Server to unify startup and shutdown logic.
// Allows to run both HTTP and HTTPS servers at the same time.
type HttpServer struct {
	httpSrv  *http.Server
	httpsSrv *http.Server
	logger   *zap.Logger
}

// NewHttpServer constructs new instance of HttpServer with handler
// registered as root path handler for both HTTP and HTTPS servers.
func NewHttpServer(handler http.Handler, logger *zap.Logger) *HttpServer {
	h := &HttpServer{
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	h.httpSrv = &http.Server{
		// no address
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // max. 1 MB
	}

	h.httpsSrv = &http.Server{
		// no address
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // max. 1 MB
	}

	return h
}

// Run starts both HTTP and HTTPS servers in their own goroutines.
func (h *HttpServer) Run(
	addr string,
	addrTls string,
	certFile string,
	keyFile string,
) {
	h.ListenAndServe(addr)
	h.ListenAndServeTLS(addrTls, certFile, keyFile)
}

// ListenAndServe starts HTTP server, listening to addr.
// NOTE: server is started in separate routine.
func (h *HttpServer) ListenAndServe(addr string) {
	h.httpSrv.Addr = addr
	go (func() {
		err := h.httpSrv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			h.logger.Debug("http server closed with error", zap.Error(err))
		}
	})()
	// not quite true, but leave there
	h.logger.Info("http server listening", zap.String("addr", h.httpSrv.Addr))
}

// ListenAndServeTLS starts HTTPS server, listening to addr.
// NOTE: server is started in separate routine.
func (h *HttpServer) ListenAndServeTLS(
	addr string,
	certFile string,
	keyFile string,
) {
	h.httpsSrv.Addr = addr
	go func() {
		err := h.httpsSrv.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			h.logger.Error("https server error", zap.Error(err))
		}
	}()

	h.logger.Info("https server listening", zap.String("addr", h.httpsSrv.Addr))
}

// Shutdown handles graceful shutdown of both HTTP and HTTPS servers.
// Starts two go routines waiting for servers to shut down under given ctx.
func (h *HttpServer) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	shutdown := func(srv *http.Server, isTLS bool) {
		defer wg.Done()
		err := srv.Shutdown(ctx)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error("server shutdown error",
				zap.Bool("tls", isTLS),
				zap.Error(err),
			)
		} else {
			h.logger.Info("server shutdown success",
				zap.Bool("tls", isTLS),
			)
		}
	}

	go shutdown(h.httpSrv, false)
	go shutdown(h.httpsSrv, true)

	wg.Wait()
}
