package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"google.golang.org/grpc"
)

type contextKey string

// traceIdKey specifies name of field storing trace id for requests
const traceIdKey contextKey = "traceId"

// GetTraceId should be used to retrieve trace id for given request.
// Empty string when field does not exist.
func GetTraceId(ctx context.Context) string {
	id, _ := ctx.Value(traceIdKey).(string)
	// TODO: maybe return nil so logger ignores
	return id
}

// HttpTracing create HTTP middleware to add short request id to request context.
func HttpTracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()[:8]
		ctx := context.WithValue(r.Context(), traceIdKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GrpcTracing create GRPC middleware to add short request id to request context.
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
