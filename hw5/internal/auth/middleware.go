package auth

import (
	"context"
	"crypto/rsa"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type claimsContextKey struct{}

// AuthorizationInterceptor verifies that incoming request has bearer token
// for all method other than publicMethods.
// Parsed claims from JWT are added to context @see ClaimFromContext
func AuthorizationInterceptor(
	verificationKey *rsa.PublicKey,
	issuer string,
	publicMethods map[string]bool,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {

		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return "", status.Error(codes.Unauthenticated, "access token missing")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return "", status.Error(codes.Unauthenticated, "access token missing")
		}

		// remove "Bearer " prefix
		token := strings.TrimPrefix(values[0], "Bearer ")
		claims, err := ValidateToken(
			token, issuer, verificationKey, AccessTokenType,
		)

		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		ctx = context.WithValue(ctx, claimsContextKey{}, claims)

		return handler(ctx, req)
	}
}

func ClaimsFromContext(ctx context.Context) (*MessengerClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*MessengerClaims)
	return claims, ok
}
