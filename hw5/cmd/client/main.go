package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"

	"gomessenger/internal"
	"gomessenger/internal/client/tui"

	"gomessenger/internal/auth"
)

const AppLogFilePath = "logs/tui.log"
const ServerCertPath = "resources/certs/server.crt"
const JwtPublicKeyPath = "resources/jwt-keys/public.key"

func main() {
	verificationKey, tlsCfg := loadSecurityAssets(JwtPublicKeyPath, ServerCertPath)

	appLogFile := createLogFile()
	defer appLogFile.Close()

	logger := internal.LogInit(appLogFile, true)
	model := tui.NewRootModel(
		context.Background(), verificationKey, tlsCfg, logger,
	)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("there's been an error: %v", err)
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
