package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"

	pb "gomessenger/generated"

	"gomessenger/internal"
	"gomessenger/internal/client/tui/app"
	"gomessenger/internal/client/tui/models"

	"gomessenger/internal/auth"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const AppLogFilePath = "logs/tui.log"
const Issuer = "hamkatar-gommessenger"
const ServerCertPath = "resources/certs/server.crt"
const JwtPublicKeyPath = "resources/jwt-keys/public.key"
const	ProfilesApiAddr = "localhost:8081"
const	MessagingApiAddr = "localhost:8082"
const LocalDataDir = "data"

func main() {
	verificationKey, tlsCfg := loadSecurityAssets(
		JwtPublicKeyPath, ServerCertPath,
	)

 	appLogFile := createLogFile()
 	defer appLogFile.Close()

 	logger := internal.LogInit(appLogFile, true)
	
	appContext := &app.AppContext{
		Ctx: context.Background(),

		RootLogger: logger,

		Config: &app.Config{
			ProfilesApiAddr:  ProfilesApiAddr,
			MessagingApiAddr: MessagingApiAddr,
			TokenIssuer:      Issuer,
			LocalDataDir:     LocalDataDir,
		},

		VerificationKey: verificationKey,
		TlsConfig:       tlsCfg,
	}

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

func setupClients(appContext  *app.AppContext) {
	credentialsInterceptor := auth.NewTokenCredentialsInterecptor(
		appContext.Config.TokenIssuer,
		appContext.VerificationKey,
		map[string]bool{
			pb.ProfileService_RegisterProfile_FullMethodName: true,
			pb.ProfileService_Login_FullMethodName:           true,
			pb.ProfileService_Refresh_FullMethodName:         true,
		},
		appContext.RootLogger.With(zap.String("module", "auth-interceptor")),
	)

	// dial both servers at application startup
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(appContext.TlsConfig)),
		grpc.WithPerRPCCredentials(credentialsInterceptor),
	}

	profileConn, err := grpc.NewClient(
		appContext.Config.ProfilesApiAddr, opts...
	)
	if err != nil {
		appContext.RootLogger.Fatal(
			"failed to connect to profile server", zap.Error(err),
		)
	}
	profileClient := pb.NewProfileServiceClient(profileConn)

	messagingConn, err := grpc.NewClient(
		appContext.Config.MessagingApiAddr, opts...
	)
	if err != nil {
		appContext.RootLogger.Fatal(
			"failed to connect to messaging server", zap.Error(err),
		)
	}

	messagingClient := pb.NewMessagingServiceClient(messagingConn)
	credentialsInterceptor.SetProfileClient(profileClient)

	appContext.CredentialsInterceptor = credentialsInterceptor
	appContext.MessagingClient = messagingClient
	appContext.ProfileClient = profileClient
	
}
