package main

import (
	"context"
	"downloader/internal"
	"downloader/internal/core"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const resource string = "https://nbg1-speed.hetzner.com/100MB.bin"

// This example submits 3 download jobs to the downloader, but ignores completion channels.
// Once all submitted, main threads initiates downloader shutdown with 5 seconds timeout.
// Expected behavior is that downloader spends half of a time, giving opportunity for tasks to finish,
// then cancels their contexts and wait other half for event system to process
// their exits and exit itself.

// Downloader internal events are logged to file "examples/02/logs/downloader.log".
// Events open to any subscriber are logged into "examples/02/logs/external-events.log".
// They contain only crucial events (complete, start, fail) and don't contain
// e.g. task ids

// [when running, see top part of output]
func main() {
	exampleDir := "examples/02/"
	downloaderLogFilePath := path.Join(exampleDir, "logs/downloader.log")
	externalEventsLogFilePath := path.Join(exampleDir, "logs/external-events.log")
	outputsDir := path.Join(exampleDir, "outputs")

	downloadLogFile, err := os.OpenFile(
		downloaderLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)

	externalEventsLogFile, err := os.OpenFile(
		externalEventsLogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)

	if err != nil {
		log.Fatal(err)
	}

	d := core.NewDefaultDownloader(internal.LogInit(downloadLogFile, true))
	d.Start()

	eventsLoggerDone := make(chan struct{})
	go (func() {
		externalEventsChan := d.Subscribe()
		logger := internal.LogInit(externalEventsLogFile, true)
		for {
			select {
			case event := <-externalEventsChan:
				// contains id, type, content (tasks not exposed)
				logger.Info(
					"External event",
					zap.String("downloadId", event.DownloadId()),
					zap.String("type", string(event.Type())),
				)
			case <-eventsLoggerDone:
				break
			}
		}

		logger.Sync()
	})()

	for i := 0; i < 3; i++ {
		filepath := path.Join(outputsDir, uuid.New().String())
		downloadId, err := d.SubmitDownload(
			context.TODO(), resource, filepath,
		)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Download [%s] to destination [%s]\n", downloadId, filepath)
	}

	fmt.Println(
		"All downloads submitted, initiating shutdown with 5 seconds timeout",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.Shutdown(ctx)
	eventsLoggerDone <- struct{}{}
	fmt.Printf(
		"Shutdown completed:\n\tOutputs: %s\n\tLogs: %s\n",
		outputsDir,
		outputsDir+"/logs",
	)

	fmt.Printf("Below is dump of all downloads stored in Downloader:\n\n\n")
	downloads := d.GetAllDownloads()
	for _, view := range downloads {
		fmt.Printf("%s\n\n", view.String())
	}
}
