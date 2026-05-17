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
// for all methods other than publicMethods.
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
		if isPublicMethod(info.FullMethod, publicMethods) {
			return handler(ctx, req)
		}

		claims, err := extractAndVerifyClaims(ctx, verificationKey, issuer)
		if err != nil {
			return nil, err
		}

		newCtx := ContextWithClaims(ctx, claims)
		return handler(newCtx, req)
	}
}

// wrappedServerStream is used to update context of gRPC stream
// https://stackoverflow.com/questions/74148348/modifying-metadata-on-go-grpc-server-streaming-interceptor
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// AuthorizationStreamInterceptor protects streaming connections by checking metadata
// and wrapping the stream context with the parsed token claims.
func AuthorizationStreamInterceptor(
	verificationKey *rsa.PublicKey,
	issuer string,
	publicMethods map[string]bool,
) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if isPublicMethod(info.FullMethod, publicMethods) {
			return handler(srv, ss)
		}

		claims, err := extractAndVerifyClaims(ss.Context(), verificationKey, issuer)
		if err != nil {
			return err
		}

		newCtx := ContextWithClaims(ss.Context(), claims)
		// wrap the original server stream with our context override
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          newCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// extractAndVerifyClaims is a that pulls metadata from a context,
// strips the authorization prefix, and validates the incoming token.
func extractAndVerifyClaims(
	ctx context.Context,
	verificationKey *rsa.PublicKey,
	issuer string,
) (*MessengerClaims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "access token missing")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "access token missing")
	}

	// remove "Bearer " prefix
	token := strings.TrimPrefix(values[0], "Bearer ")
	claims, err := ValidateToken(
		token, issuer, verificationKey, AccessTokenType,
	)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid access token")
	}

	return claims, nil
}

func ContextWithClaims(
	ctx context.Context, messengerClaims *MessengerClaims,
) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, messengerClaims)
}

func ClaimsFromContext(ctx context.Context) (*MessengerClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*MessengerClaims)
	return claims, ok
}

func isPublicMethod(fullMethod string, publicMethods map[string]bool) bool {
	if publicMethods[fullMethod] {
		return true
	}

	// Allow grpcurl / reflection clients.
	if strings.HasPrefix(
		fullMethod,
		"/grpc.reflection.v1alpha.ServerReflection/",
	) {
		return true
	}

	if strings.HasPrefix(
		fullMethod,
		"/grpc.reflection.v1.ServerReflection/",
	) {
		return true
	}

	return false
}
