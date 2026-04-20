package middleware

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"google.golang.org/grpc"
)

func HttpLogging(logger *zap.Logger, next http.Handler) http.Handler {
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

func GrpcLogging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		logger.Info(
			"received request",
			zap.String("trace", GetTraceId(ctx)),
			zap.String("procedure", info.FullMethod),
		)
		res, err := handler(ctx, req)

		logger.Info(
			"request handled",
			zap.String("trace", GetTraceId(ctx)),
			zap.Error(err),
		)
		return res, err
	}
}
