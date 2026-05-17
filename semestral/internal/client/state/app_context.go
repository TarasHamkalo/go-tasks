package state

import (
	"context"
	"crypto/rsa"
	"crypto/tls"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
	"gomessenger/internal/client/storage"

	"go.uber.org/zap"
)

type AppContext struct {
	Ctx context.Context

	RootLogger *zap.Logger

	Session *Session
	Config  *Config

	VerificationKey *rsa.PublicKey
	TlsConfig       *tls.Config

	CredentialsInterceptor *auth.TokenCredentialsInterecptor

	ProfileClient   pb.ProfileServiceClient
	MessagingClient pb.MessagingServiceClient

	LocalRepo storage.Repository
}
