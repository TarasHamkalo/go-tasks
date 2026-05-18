// Package state defines client configuration and runtime state passed
// to every model. All types defined in this package are not stored, for 
// those see storage package.
package state

import (
	"context"
	"crypto/rsa"
	"crypto/tls"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
	"gomessenger/internal/client/storage"
)

// AppContext, super large and nice for UI :) object containing everything
// possibly need - runtime user session data, static config and other.
type AppContext struct {
	// Ctx used to derive smaller contexts per request, should not be used directly
	// but as parent. Is nice to stop whole client operation at once.
	Ctx context.Context

	// RootLogger similarly as context, should be used to derive smaller loggers
	RootLogger *zap.Logger

	// Session stores user runtime data as e.g. profile, chats
	// Should act as cache for remote operations
	Session *Session

	// Config is readonly config of client
	Config *Config

	// Public key used to verify JWT signatures.
	VerificationKey *rsa.PublicKey

	// TLS configuration shared by all gRPC clients.
	TlsConfig *tls.Config

	// Per-RPC credentials interceptor handling access token refresh.
	CredentialsInterceptor *auth.TokenCredentialsInterecptor

	// Connection to profile service.
	ProfileConn   *grpc.ClientConn
	ProfileClient pb.ProfileServiceClient

	// Connection to messaging service.
	MessagingConn   *grpc.ClientConn
	MessagingClient pb.MessagingServiceClient

	// Dedicated connection used for token refresh requests.
	RefreshConn   *grpc.ClientConn
	RefreshClient pb.ProfileServiceClient

	// Local client-side persistence repository.
	LocalRepo storage.Repository
}

func (a *AppContext) Close() {
	if a.LocalRepo != nil {
		_ = a.LocalRepo.Close()
	}

	if a.RefreshConn != nil {
		_ = a.RefreshConn.Close()
	}

	if a.ProfileConn != nil {
		_ = a.ProfileConn.Close()
	}

	if a.MessagingConn != nil {
		_ = a.MessagingConn.Close()
	}
}
