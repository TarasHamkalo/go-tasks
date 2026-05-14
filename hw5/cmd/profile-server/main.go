package main

import (
	"context"
	"crypto/tls"
	pb "gomessenger/generated"
	gomessenger "gomessenger/internal"
	"gomessenger/internal/auth"
	"gomessenger/internal/profiles"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	privateKey, err := gomessenger.LoadPrivateKey(PrivateKeyPath)
	if err != nil {
		appLogger.Fatal("could not load private key", zap.Error(err))
		os.Exit(1)
	}

	publicKey, err := gomessenger.LoadPublicKey(PublicKeyPath)
	if err != nil {
		appLogger.Fatal("could not load public key", zap.Error(err))
		os.Exit(1)
	}

	cert, err := tls.LoadX509KeyPair(CertPath, KeyPath)
	if err != nil {
		appLogger.Fatal("could not load server certs", zap.Error(err))
		os.Exit(1)
	}

	tlsCfg := tls.Config{Certificates: []tls.Certificate{cert}}

	repo, err := profiles.NewSqliteRepository(ProfilesDbPath)
	if err != nil {
		appLogger.Fatal("could not open sqlite db", zap.Error(err))
		os.Exit(1)
	}

	defer repo.Close()

	err = repo.InitializeSchema(context.TODO())
	if err != nil {
		appLogger.Fatal("could not initialize schema", zap.Error(err))
		os.Exit(1)
	}

	appLogger.Info(
		"database and schema initialized", zap.String("path", ProfilesDbPath),
	)

	grpcServerLogger := appLogger.With(zap.String("module", "grpc-server"))
	grpcServer := gomessenger.NewGrpcServer(
		&tlsCfg,
		grpcServerLogger,
		auth.AuthorizationInterceptor(
			publicKey,
			Issuer,
			map[string]bool{
				"/profile.ProfileService/RegisterProfile": true,
				"/profile.ProfileService/Login":           true,
				"/profile.ProfileService/Refresh":         true,
			},
		),
	)

	grpcServer.WithServer(func(srv *grpc.Server) {
		pb.RegisterProfileServiceServer(
			srv,
			profiles.NewProfileService(repo, Issuer, publicKey, privateKey, appLogger),
		)
	})

	if err = grpcServer.Serve(8081); err != nil {
		appLogger.Error("failed to start gRPC server", zap.Error(err))
		os.Exit(1)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	appLogger.Info(
		"initiate shutdown",
		zap.Duration("timeout", time.Duration(time.Second*5)),
	)

	ctx, cancel := context.WithTimeout(
		context.Background(), time.Duration(time.Second*5),
	)

	defer cancel()
	grpcServer.Shutdown(ctx)

	appLogger.Info("main routine exits")
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
			"Failed to create app server log, file=%s, err=%v",
			AppLogFilePath,
			err,
		)
	}

	return appLogFile
}

// p := profiles.Profile{
// 	UserId:   "1234",
// 	Username: "Taras",
// 	Password: []byte("Taras"),
// }
//
// err = repo.InsertProfile(context.TODO(), p)
// if err == profiles.ErrorUniqueConstraintViolated {
// 	appLogger.Error(
// 		"could not insert user because user id not unique", zap.Error(err),
// 	)
// }
//
// userP, err := repo.GetProfileByUserId(context.TODO(), p.UserId)
//
// if err != nil {
// 	appLogger.Error(
// 		"could not get user", zap.Error(err),
// 	)
// }
// fmt.Println(userP)
//
// service.Login(context.TODO(), &pb.LoginRequest{})
