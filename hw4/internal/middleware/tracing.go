package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const traceIdKey contextKey = "traceId"

func Tracing(next http.Handler) http.Handler {
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
