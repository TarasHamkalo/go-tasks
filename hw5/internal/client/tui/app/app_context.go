package app

import (
	"context"
	"crypto/rsa"
	"crypto/tls"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"

	"go.uber.org/zap"
)

type AppContext struct {
	Ctx context.Context
	
	RootLogger *zap.Logger

	Config *Config

	VerificationKey *rsa.PublicKey
	TlsConfig       *tls.Config

	CredentialsInterceptor *auth.TokenCredentialsInterecptor

	ProfileClient   pb.ProfileServiceClient
	MessagingClient pb.MessagingServiceClient
}
