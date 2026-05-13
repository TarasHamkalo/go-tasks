package main

import (
	"context"
	"fmt"
	gomessenger "gomessenger/internal"
	"gomessenger/internal/profiles"
	"log"
	"os"

	"go.uber.org/zap"
	sqlite "modernc.org/sqlite"
)


const AppLogFilePath = "logs/profile-server.log"
const ProfilesDbPath = "data/profiles.db"

func main() {
	appLogFile := createLogFile()
	defer appLogFile.Close()

	appLogger := gomessenger.LogInitWithConsole(appLogFile, true)

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

	p := profiles.Profile{
		UserId: "1234",
		Username: "Taras",
		Password: "Taras",
	}

	err = repo.InsertProfile(context.TODO(), p)

	if err != nil {
		appLogger.Error(
			"could not insert user", zap.Error(err), zap.String("error", sqlite.ErrorCodeString[err.]
		)
	}
	
	userP, err := repo.GetProfileByUserId(context.TODO(), p.UserId)

	if err != nil {
		appLogger.Error(
			"could not get user", zap.Error(err),
		)
	}
	
	fmt.Println(userP)
}

func createLogFile() *os.File {
	if err := os.Mkdir("logs", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	appLogFile, err := os.OpenFile(
		AppLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
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
