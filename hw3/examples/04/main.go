package main

import (
	"context"
	"downloader/internal"
	"downloader/internal/shell/commands"
	"downloader/pkg/downloader"
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

// Shell commands verification
// [when running, see top part of output]
func main() {
	exampleDir := "examples/04/"
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

	d := downloader.NewDefaultDownloader(internal.LogInit(downloadLogFile, true))
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

	var headCmd = commands.NewDownloadCommand(d)
	headCmd.WithNext(commands.NewStatusCommand(d))

	for i := 0; i < 3; i++ {
		filepath := path.Join(outputsDir, uuid.New().String())
		err = headCmd.Handle(fmt.Sprintf("download %s %s", resource, filepath))
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("Suggestions:")
	fmt.Printf("\tprompt contains \"s\":\n")
	fmt.Println(headCmd.CompletePrompt("s"))
	fmt.Printf("\tprompt contains \"status\":\n")
	fmt.Println(headCmd.CompletePrompt("status"))

	fmt.Printf(
		"\n\nAll downloads submitted, shutting down with timeout 5 seconds:\n\tOutputs: %s\n\tLogs: %s\n",
		outputsDir,
		outputsDir+"/logs",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.Shutdown(ctx)
	eventsLoggerDone <- struct{}{}

	fmt.Println("Call handle(\"status\")")
	err = headCmd.Handle("status")
	if err != nil {
		log.Fatal(err)
	}
}
