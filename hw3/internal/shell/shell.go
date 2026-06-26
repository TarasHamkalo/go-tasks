package shell

import (
	"context"
	"downloader/internal"
	"downloader/internal/shell/commands"
	"downloader/pkg/downloader"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/c-bata/go-prompt"
)

// Shell represents downloader shell, containing command handlers chain and
// being responsible for logs file, downloader instance creation/shutdown.
//
// NOTE: shell should not be used after Run exits (file creation/truncation)
type Shell struct {
	commandsChain commands.Command

	downloader *downloader.Downloader

	downloaderLogFile *os.File

	// shutdown indicates that shell should exit.
	// Initially bound to routine waiting for OS signal to occur through,
	// due to go-prompt events handling, all modifications to it are done
	// in single routine (atomicity not used anymore).
	shutdown atomic.Bool

	// shutdownDoneCh main routine await closing of this channel after main
	// loop exits. Left for similar reason as above atomic shutdown.
	shutdownDoneCh chan struct{}
}

// NewShell constructs shell with default command set (download, status, cancel).
// NOTE: on creation gonna create logs dir and truncate existing logs
func NewShell() *Shell {
	err := os.Mkdir("logs", 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	downloadLogFile, err := os.OpenFile(
		"logs/downloader.log",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		panic(err)
	}

	downloaderLogger := internal.LogInit(downloadLogFile, true)

	d := downloader.NewDefaultDownloader(downloaderLogger)
	chain := commands.NewDownloadCommand(d)
	chain.
		WithNext(commands.NewStatusCommand(d)).
		WithNext(commands.NewCancelCommand(d))

	return &Shell{
		commandsChain:     chain,
		downloader:        d,
		downloaderLogFile: downloadLogFile,
		shutdownDoneCh:    make(chan struct{}),
	}
}

// handle trims input and passes down commandsChain
func (s *Shell) handle(input string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return
	}

	if trimmed == "exit" {
		s.handleExit()
		return
	}

	err := s.commandsChain.Handle(trimmed)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// handleExit handles graceful shutdown of Downloader by providing context
// with 5 seconds timeout
func (s *Shell) handleExit() {
	if s.shutdown.Load() {
		return
	}

	fmt.Println("Shutting down downloader (timeout 5s)...")

	s.shutdown.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.downloader.Shutdown(ctx)

	fmt.Println("Shutdown complete.")

	close(s.shutdownDoneCh)
}

// completer takes currently present input and passed down to commands chain,
// gathering possible completions.
func (s *Shell) completer(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	suggestions := s.commandsChain.CompletePrompt(text)
	if strings.HasPrefix("exit", text) {
		suggestions = append(
			suggestions, prompt.Suggest{Text: "exit", Description: "Exit"},
		)
	}

	return suggestions
}

// Run starts input handling loop
func (s *Shell) Run() {
	defer s.downloaderLogFile.Close()

	promptAsyncMsgChan := make(chan string)
	go s.startEventsProcessing(promptAsyncMsgChan)
	p := prompt.New(
		s.handle,
		s.completer,
		prompt.OptionPrefix(">>> "),
		prompt.OptionTitle("downloader-shell"),
		prompt.OptionWithAsyncMessageChan(promptAsyncMsgChan),
	)

	s.downloader.Start()
	for !s.shutdown.Load() {
		in, shouldExit := p.Input()
		if shouldExit {
			s.handleExit()
			break
		}
		s.handle(in)
	}

	fmt.Println("Main routine await shutdown.")
	<-s.shutdownDoneCh
}

// startEventsProcessing subscribes to downloader.Downloader events
// and prints them to user.
// Go-prompt initially didn't allow to print async messages while user
// was providing input, I could not fix this issue with provided api, so
// forked go-prompt :)
func (s *Shell) startEventsProcessing(outputChan chan<- string) {
	events := s.downloader.Subscribe()
	for {
		select {
		case e, ok := <-events:
			if !ok {
				return
			}

			id := e.DownloadId()[:8]
			var msg string

			switch e.Type() {

			case downloader.ExternalEventStart:
				msg = fmt.Sprintf(
					"[start]\t%s -> downloading (%s)",
					id,
					commands.FormatBytes(e.Bytes()),
				)

			case downloader.ExternalEventCancel:
				msg = fmt.Sprintf(
					"[cancel]\t%s -> cancelled",
					id,
				)

			case downloader.ExternalEventComplete:
				msg = fmt.Sprintf(
					"[done]\t%s -> completed (%s)",
					id,
					commands.FormatBytes(e.Bytes()),
				)

			case downloader.ExternalEventError:
				errStr := "unknown error"
				if e.Err() != nil {
					errStr = e.Err().Error()
				}

				msg = fmt.Sprintf(
					"[error]\t%s -> %s",
					id,
					errStr,
				)
			}

			outputChan <- msg

		case <-s.shutdownDoneCh:
			return
		}
	}
}
