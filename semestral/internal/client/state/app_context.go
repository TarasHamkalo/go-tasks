package state

import (
	"context"
	"crypto/rsa"
	"crypto/tls"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
	"gomessenger/internal/client/storage"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type AppContext struct {
	Ctx context.Context

	RootLogger *zap.Logger

	Session *Session
	Config  *Config

	VerificationKey *rsa.PublicKey
	TlsConfig       *tls.Config

	CredentialsInterceptor *auth.TokenCredentialsInterecptor

	RefreshConn *grpc.ClientConn
	ProfileConn   *grpc.ClientConn
	MessagingConn *grpc.ClientConn

	RefreshClient pb.ProfileServiceClient
	ProfileClient   pb.ProfileServiceClient
	MessagingClient pb.MessagingServiceClient

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
