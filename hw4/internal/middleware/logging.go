package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

func Logging(logger *zap.Logger, next http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		logger.Info(
			"received request",
			zap.String("trace", GetTraceId(r.Context())),
			zap.String("addr", r.RemoteAddr),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(f)
}
