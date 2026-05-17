package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gomessenger/generated"
	gomessenger "gomessenger/internal"
	"gomessenger/internal/auth"
	"gomessenger/internal/messaging"
)

type Config struct {
	AppLogFilePath  string `env:"APP_LOG_FILE,required"`
	MessagingDbPath string `env:"MESSAGING_DB,required"`

	// Address of the profile service used for cross-service calls
	// (e.g. updating user presence).
	ProfilesApiAddr string `env:"PROFILES_API_ADDR,required"`

	PublicKeyPath string `env:"JWT_PUBLIC_KEY,required"`

	CertPath string `env:"TLS_CERT,required"`
	KeyPath  string `env:"TLS_KEY,required"`

	Issuer string `env:"JWT_ISSUER,required"`

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

	publicKey, tlsCfg := loadSecurityAssets(appLogger)

	// build client connection to profile service
	profileServiceConn, profileServiceClient := buildProfileClient(appLogger)
	defer func() {
		appLogger.Info("closing profile service client connection")
		if err := profileServiceConn.Close(); err != nil {
			appLogger.Error("failed to close profile service connection cleanly", zap.Error(err))
		}
	}()

	repo := initDatabase(appLogger)
	defer func() {
		appLogger.Info("closing database repository")
		if err := repo.Close(); err != nil {
			appLogger.Error("failed to close database repository cleanly", zap.Error(err))
		}
	}()

	grpcServer := initGrpcServer(
		appLogger,
		tlsCfg,
		publicKey,
		repo,
		profileServiceClient,
	)
	if err := grpcServer.Serve(cfg.Port); err != nil {
		appLogger.Fatal("failed to start gRPC server", zap.Error(err))
	}

	waitForShutdown(appLogger, grpcServer)
}

// loadSecurityAssets handles loading JWT keys and SSL certificates
func loadSecurityAssets(logger *zap.Logger) (*rsa.PublicKey, *tls.Config) {
	publicKey, err := auth.LoadPublicKey(cfg.PublicKeyPath)
	if err != nil {
		logger.Fatal("could not load public key", zap.Error(err))
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		logger.Fatal("could not load server certs", zap.Error(err))
	}

	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}
	return publicKey, &tlsCfg
}

func initDatabase(logger *zap.Logger) messaging.Repository {
	repo, err := messaging.NewSqliteRepository(cfg.MessagingDbPath)
	if err != nil {
		logger.Fatal("could not open sqlite db", zap.Error(err))
	}

	if err = repo.InitializeSchema(context.Background()); err != nil {
		repo.Close()
		logger.Fatal("could not initialize schema", zap.Error(err))
	}

	logger.Info(
		"database and schema initialized",
		zap.String("path", cfg.MessagingDbPath),
	)
	return repo
}

// initGrpcServer configures interceptors and registers services
func initGrpcServer(
	logger *zap.Logger,
	tlsCfg *tls.Config,
	publicKey *rsa.PublicKey,
	repo messaging.Repository,
	profileServiceClient pb.ProfileServiceClient,
) *gomessenger.GrpcServer {
	publicRoutes := map[string]bool{}
	grpcServer := gomessenger.NewGrpcServerWithStreams(
		tlsCfg,
		logger,
		auth.AuthorizationInterceptor(publicKey, cfg.Issuer, publicRoutes),
		auth.AuthorizationStreamInterceptor(publicKey, cfg.Issuer, publicRoutes),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterMessagingServiceServer(
			srv,
			messaging.NewMessagingService(repo, profileServiceClient, logger),
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

// buildProfileClient creates a TLS-secured gRPC client for the ProfileService.
func buildProfileClient(logger *zap.Logger) (*grpc.ClientConn, pb.ProfileServiceClient) {
	pem, err := os.ReadFile(cfg.CertPath)
	if err != nil {
		logger.Fatal("could not read CA certificate", zap.Error(err))
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		logger.Fatal("could not parse CA certificate")
	}

	clientTLSConfig := &tls.Config{
		RootCAs:    pool,
		ServerName: "localhost", // must match certificate SAN/CN
	}

	conn, err := grpc.NewClient(
		cfg.ProfilesApiAddr,
		grpc.WithTransportCredentials(
			credentials.NewTLS(clientTLSConfig),
		),
	)
	if err != nil {
		logger.Fatal("failed to connect to profile service", zap.Error(err))
	}

	return conn, pb.NewProfileServiceClient(conn)
}
