package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type HttpServer struct {
	handler  http.Handler
	httpSrv  *http.Server
	httpsSrv *http.Server
	logger   *zap.Logger
}

func NewHttpServer(handler http.Handler, logger *zap.Logger) *HttpServer {
	h := &HttpServer{
		handler: handler,
		logger:  logger,
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

func (h *HttpServer) Run(
	addr string,
	addrTls string,
	certFile string,
	keyFile string,
) {
	h.ListenAndServe(addr)
	h.ListenAndServeTLS(addrTls, certFile, keyFile)
}

func (h *HttpServer) ListenAndServe(addr string) {
	h.httpSrv.Addr = addr
	go (func() {
		if err := h.httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			h.logger.Debug("http server closed with error", zap.Error(err))
		}
	})()
	// not quite true, but leave there
	h.logger.Info("http server listening", zap.String("addr", h.httpSrv.Addr))
}

func (h *HttpServer) ListenAndServeTLS(addr string, certFile string, keyFile string) {
	h.httpsSrv.Addr = addr
	go func() {
		err := h.httpsSrv.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			h.logger.Error("https server error", zap.Error(err))
		}
	}()

	h.logger.Info("https server listening", zap.String("addr", h.httpsSrv.Addr))
}

func (h *HttpServer) Shutdown(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	shutdown := func(srv *http.Server, isTLS bool) {
		defer wg.Done()
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
