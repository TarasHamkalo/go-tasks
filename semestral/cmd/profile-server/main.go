package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/caarlos0/env/v11"
	"google.golang.org/grpc"

	pb "gomessenger/generated"
	gomessenger "gomessenger/internal"

	"gomessenger/internal/auth"
	"gomessenger/internal/profiles"
)


// Config holds the application configuration parsed from environment variables
type Config struct {
	AppLogFilePath string `env:"APP_LOG_FILE,required"`

	// Sqlite database file path
	ProfilesDbPath string `env:"PROFILES_DB,required"`

	// JWT issuer and verification, signing keys
	Issuer string `env:"JWT_ISSUER,required"`
	PublicKeyPath  string `env:"JWT_PUBLIC_KEY,required"`
	PrivateKeyPath string `env:"JWT_PRIVATE_KEY,required"`

	// TLS cert and private key
	CertPath string `env:"TLS_CERT,required"`
	KeyPath  string `env:"TLS_KEY,required"`

	Port int `env:"PORT,required"`
}

var cfg Config

func init() {
	if err := env.Parse(&cfg); err != nil {
		panic(fmt.Errorf("failed to load configuration: %w", err))
	}
}

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
	privateKey, err := auth.LoadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		logger.Fatal("could not load private key", zap.Error(err))
	}

	publicKey, err := auth.LoadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		logger.Fatal("could not load public key", zap.Error(err))
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		logger.Fatal("could not load server certs", zap.Error(err))
	}

	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}
	return privateKey, publicKey, &tlsCfg
}

// initDatabase prepares SQLite database
func initDatabase(logger *zap.Logger) *profiles.SqliteRepository {
	repo, err := profiles.NewSqliteRepository(cfg.ProfilesDbPath)
	if err != nil {
		logger.Fatal("could not open sqlite db", zap.Error(err))
	}

	if err = repo.InitializeSchema(context.Background()); err != nil {
		repo.Close()
		logger.Fatal("could not initialize schema", zap.Error(err))
	}

	logger.Info(
		"database and schema initialized",
		zap.String("path", cfg.ProfilesDbPath),
	)
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

	// define unprotected routes
	publicRoutes := map[string]bool{
		pb.ProfileService_RegisterProfile_FullMethodName: true,
		pb.ProfileService_Login_FullMethodName:           true,
		pb.ProfileService_Refresh_FullMethodName:         true,
	}

	grpcServer := gomessenger.NewGrpcServer(
		tlsCfg,
		logger,
		auth.AuthorizationInterceptor(publicKey, cfg.Issuer, publicRoutes),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterProfileServiceServer(
			srv,
			profiles.NewProfileService(
				repo,
				cfg.Issuer,
				publicKey,
				privateKey,
				logger,
			),
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

// createLogFile ensures the log directory exists and opens the application log file
func createLogFile() *os.File {
	if err := os.Mkdir("logs", 0750); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(
		cfg.AppLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0600,
	)
	if err != nil {
		log.Fatalf(
			"Failed to create app server log, file=%s, err=%v",
			cfg.AppLogFilePath,
			err,
		)
	}

	return appLogFile
}
