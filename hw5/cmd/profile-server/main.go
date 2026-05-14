package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"google.golang.org/grpc"

	pb "gomessenger/generated"
	gomessenger "gomessenger/internal"

	"gomessenger/internal/auth"
	"gomessenger/internal/profiles"
)

const AppLogFilePath = "logs/profile-server.log"
const ProfilesDbPath = "data/profiles.db"

const PublicKeyPath = "resources/jwt-keys/public.key"
const PrivateKeyPath = "resources/jwt-keys/private.key"

const CertPath = "resources/certs/server.crt"
const KeyPath = "resources/certs/server.key"

const Issuer = "hamkatar-gommessenger"

func main() {
	appLogFile := createLogFile()
	defer appLogFile.Close()

	appLogger := gomessenger.LogInitWithConsole(appLogFile, true)

	privateKey, publicKey, tlsCfg := loadSecurityAssets(appLogger)

	repo := initDatabase(appLogger)
	defer repo.Close()

	grpcServer := initGrpcServer(appLogger, tlsCfg, publicKey, privateKey, repo)

	if err := grpcServer.Serve(8081); err != nil {
		appLogger.Fatal("failed to start gRPC server", zap.Error(err))
	}

	waitForShutdown(appLogger, grpcServer)
}

// loadSecurityAssets handles loading JWT keys and SSL certificates
func loadSecurityAssets(
	logger *zap.Logger,
) (*rsa.PrivateKey, *rsa.PublicKey, *tls.Config) {
	privateKey, err := auth.LoadPrivateKey(PrivateKeyPath)
	if err != nil {
		logger.Fatal("could not load private key", zap.Error(err))
	}

	publicKey, err := auth.LoadPublicKey(PublicKeyPath)
	if err != nil {
		logger.Fatal("could not load public key", zap.Error(err))
	}

	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		logger.Fatal("could not load server certs", zap.Error(err))
	}

	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}
	return privateKey, publicKey, &tlsCfg
}

// initDatabase prepares SQLite database
func initDatabase(logger *zap.Logger) *profiles.SqliteRepository {
	repo, err := profiles.NewSqliteRepository(ProfilesDbPath)
	if err != nil {
		logger.Fatal("could not open sqlite db", zap.Error(err))
	}

	if err = repo.InitializeSchema(context.Background()); err != nil {
		repo.Close()
		logger.Fatal("could not initialize schema", zap.Error(err))
	}

	logger.Info("database and schema initialized", zap.String("path", ProfilesDbPath))
	return repo
}

// initGrpcServer configures interceptors and registers services
func initGrpcServer(
	logger *zap.Logger,
	tlsCfg *tls.Config,
	publicKey *rsa.PublicKey,
	privateKey *rsa.PrivateKey,
	repo *profiles.SqliteRepository,
) *gomessenger.GrpcServer {

	// TODO: maybe separate but loggers by modules

	// define unprotected routes
	publicRoutes := map[string]bool{
		"/profile.ProfileService/RegisterProfile": true,
		"/profile.ProfileService/Login":           true,
		"/profile.ProfileService/Refresh":         true,
	}

	grpcServer := gomessenger.NewGrpcServer(
		tlsCfg,
		logger,
		auth.AuthorizationInterceptor(publicKey, Issuer, publicRoutes),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterProfileServiceServer(
			srv,
			profiles.NewProfileService(repo, Issuer, publicKey, privateKey, logger),
		)
	})

	return grpcServer
}

// waitForShutdown blocks until an OS signal is intercepted,
// then gracefully stops the server
func waitForShutdown(logger *zap.Logger, server *gomessenger.GrpcServer) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownTimeout := 5 * time.Second
	logger.Info("initiate shutdown", zap.Duration("timeout", shutdownTimeout))

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	server.Shutdown(ctx)
	logger.Info("main routine exits")
}

func createLogFile() *os.File {
	if err := os.Mkdir("logs", 0750); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(
		AppLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0600,
	)
	if err != nil {
		log.Fatalf(
			"Failed to create app server log, file=%s, err=%v", AppLogFilePath, err,
		)
	}

	return appLogFile
}
