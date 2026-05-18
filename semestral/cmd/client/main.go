package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	pb "gomessenger/generated"

	"gomessenger/internal"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui/models"

	"gomessenger/internal/auth"

	tea "charm.land/bubbletea/v2"
	"github.com/caarlos0/env/v11"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	AppLogFilePath string `env:"APP_LOG_FILE,required"`

	// Trusted JWT issuer and verification key.
	TokenIssuer      string `env:"JWT_ISSUER,required"`
	JwtPublicKeyPath string `env:"JWT_PUBLIC_KEY,required"`

	// TLS certificate trusted by the client when connecting to servers.
	ServerCertPath string `env:"SERVER_CERT,required"`

	// gRPC endpoints.
	ProfilesApiAddr  string `env:"PROFILES_API_ADDR,required"`
	MessagingApiAddr string `env:"MESSAGING_API_ADDR,required"`

	// Directory for local client state (tokens, cache, etc.).
	LocalDataDir string `env:"LOCAL_DATA_DIR,required"`
}

var cfg Config

func init() {
	if err := env.Parse(&cfg); err != nil {
		panic(fmt.Errorf("failed to load configuration: %w", err))
	}
}

func main() {
	verificationKey, tlsCfg := loadSecurityAssets(
		cfg.JwtPublicKeyPath,
		cfg.ServerCertPath,
	)

	appLogFile := createLogFile()
	defer appLogFile.Close()

	logger := internal.LogInit(appLogFile, true)

	// cancel global context when framework loop exits
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appContext := &state.AppContext{
		Ctx: ctx,

		RootLogger: logger,

		Config: &state.Config{
			ProfilesApiAddr:  cfg.ProfilesApiAddr,
			MessagingApiAddr: cfg.MessagingApiAddr,
			TokenIssuer:      cfg.TokenIssuer,
			LocalDataDir:     cfg.LocalDataDir,
		},

		VerificationKey: verificationKey,
		TlsConfig:       tlsCfg,
	}

	// close context it is everything that was initialized (repo, connection)
	defer appContext.Close()

	setupClients(appContext)

	model := models.NewRootModel(appContext)

	p := tea.NewProgram(model, tea.WithContext(appContext.Ctx))
	if _, err := p.Run(); err != nil {
		logger.Info("error occurred BubbleTea run", zap.Error(err))
		os.Exit(1)
	}
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

func loadSecurityAssets(
	publicKeyPath, certPath string,
) (*rsa.PublicKey, *tls.Config) {
	publicKey, err := auth.LoadPublicKey(publicKeyPath)
	if err != nil {
		log.Fatal("could not load public: %w", err)
	}

	pem, err := os.ReadFile(certPath)
	if err != nil {
		log.Fatalf("not read certificate: %w", err)
	}

	// create trust store
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		log.Fatal("could not parse certificate")
	}

	// configure TLS.
	tlsCfg := &tls.Config{
		RootCAs: pool,

		// Must match the certificate's Common Name (CN) or Subject Alternative Name.
		ServerName: "localhost",
	}

	return publicKey, tlsCfg
}

func setupClients(appContext *state.AppContext) {
	credentialsInterceptor := auth.NewTokenCredentialsInterecptor(
		appContext.Config.TokenIssuer,
		appContext.VerificationKey,
		appContext.RootLogger.With(zap.String("module", "auth-interceptor")),
	)

	// dial both servers at application startup
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(appContext.TlsConfig)),
		grpc.WithPerRPCCredentials(credentialsInterceptor),
	}

	profileConn, err := grpc.NewClient(
		appContext.Config.ProfilesApiAddr, opts...,
	)

	if err != nil {
		appContext.RootLogger.Fatal(
			"failed to connect to profile server", zap.Error(err),
		)
	}
	profileClient := pb.NewProfileServiceClient(profileConn)

	messagingConn, err := grpc.NewClient(
		appContext.Config.MessagingApiAddr, opts...,
	)

	if err != nil {
		appContext.RootLogger.Fatal(
			"failed to connect to messaging server", zap.Error(err),
		)
	}

	messagingClient := pb.NewMessagingServiceClient(messagingConn)

	// use separate client to refresh tokens (no recursion in interceptor)
	refreshConn, err := grpc.NewClient(
		appContext.Config.ProfilesApiAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(appContext.TlsConfig)),
	)

	if err != nil {
		appContext.RootLogger.Fatal(
			"failed to connect to profile server", zap.Error(err),
		)
	}

	refreshClient := pb.NewProfileServiceClient(refreshConn)
	credentialsInterceptor.SetProfileClient(refreshClient)


	appContext.CredentialsInterceptor = credentialsInterceptor

	appContext.ProfileConn = profileConn
	appContext.MessagingConn = messagingConn

	appContext.MessagingClient = messagingClient
	appContext.ProfileClient = profileClient

}
