package main

import (
	"context"
	"downloader/internal"
	"downloader/internal/core"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const resource string = "https://nbg1-speed.hetzner.com/100MB.bin"

func mergeDone(chans ...<-chan struct{}) <-chan struct{} {
	var wg sync.WaitGroup
	out := make(chan struct{})

	wg.Add(len(chans))

	for _, ch := range chans {
		go func(c <-chan struct{}) {
			defer wg.Done()
			<-c
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// This example submits 3 download jobs to the downloader.
// For each submitted download, a completion channel is retrieved.
// All completion channels are merged and awaited, ensuring all downloads finish.
// Once all downloads are completed, a graceful shutdown of the downloader is triggered.

// Downloader internal events are logged to file "examples/01/logs/downloader.log".
// Events open to any subscriber are logged into "examples/01/logs/external-events.log".
// They contain only crucial events (complete, start, fail) and don't contain
// e.g. task ids
func main() {
	exampleDir := "examples/01/"
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

	completionChans := make([]<-chan struct{}, 0)
	for i := 0; i < 3; i++ {
		filepath := path.Join(outputsDir, uuid.New().String())
		downloadId, err := d.SubmitDownload(
			context.TODO(), resource, filepath,
		)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Download [%s] to destination [%s]\n", downloadId, filepath)
		compChan, err := d.CompletionChan(downloadId)
		if err != nil {
			log.Fatal(err)
		}

		completionChans = append(completionChans, compChan)
	}

	fmt.Println("All downloads submitted, awaiting completion")
	<-mergeDone(completionChans...)

	fmt.Printf(
		"All downloads completed:\n\tOutputs: %s\n\tLogs: %s\n",
		outputsDir,
		downloaderLogFilePath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.Shutdown(ctx)
	eventsLoggerDone <- struct{}{}
	statues := d.GetAllDownloads()
	for _, view := range statues {
		fmt.Printf("%s\n\n", view.String())
	}
}
