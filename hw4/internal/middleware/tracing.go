package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"google.golang.org/grpc"
)

type contextKey string

const traceIdKey contextKey = "traceId"

func HttpTracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()[:8]
		ctx := context.WithValue(r.Context(), traceIdKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTraceId(ctx context.Context) string {
	id, _ := ctx.Value(traceIdKey).(string)
	// empty string :)
	return id
}

func GrpcTracing() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {

		id := uuid.New().String()[:8]
		ctx = context.WithValue(ctx, traceIdKey, id)
		return handler(ctx, req)
	}
}
