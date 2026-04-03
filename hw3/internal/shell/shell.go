package shell

import (
	"context"
	"downloader/internal"
	"downloader/internal/core"
	"downloader/internal/shell/commands"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/c-bata/go-prompt"
	"go.uber.org/zap"
)

type Shell struct {
	commandsChain commands.Command

	downloader *core.Downloader

	downloaderLogFile *os.File

	shellLogFile *os.File

	shutdown atomic.Bool

	shutdownDoneCh chan struct{}

	logger *zap.Logger
}

func NewShell() *Shell {
	downloadLogFile, err := os.OpenFile(
		"logs/downloader.log",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		panic(err)
	}

	shellLogFile, err := os.OpenFile(
		"logs/shell.log",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		panic(err)
	}

	downloaderLogger := internal.LogInit(downloadLogFile, true)
	shellLogger := internal.LogInit(shellLogFile, true)

	d := core.NewDefaultDownloader(downloaderLogger)
	chain := commands.NewDownloadCommand(d)
	chain.
		WithNext(commands.NewStatusCommand(d)).
		WithNext(commands.NewCancelCommand(d))

	return &Shell{
		commandsChain:     chain,
		downloader:        d,
		downloaderLogFile: downloadLogFile,
		shellLogFile:      shellLogFile,
		logger:            shellLogger,
		shutdownDoneCh:    make(chan struct{}),
	}
}

func (s *Shell) execute(input string) {
	if input == "" {
		return
	}

	if input == "exit" {
		s.handleExit()
		return
	}

	err := s.commandsChain.Handle(input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

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

func (s *Shell) Run() {
	defer s.downloaderLogFile.Close()
	defer s.shellLogFile.Close()

	promptAsyncMsgChan := make(chan string)
	go s.startEventsProcessing(promptAsyncMsgChan)
	p := prompt.New(
		s.execute,
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
		s.execute(in)
	}

	fmt.Println("Main routine await shutdown.")
	<-s.shutdownDoneCh
}

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

			case core.EDownloadEventStart:
				msg = fmt.Sprintf(
					"[start]\t%s -> downloading (%s)",
					id,
					commands.FormatBytes(e.Bytes()),
				)

			case core.EDownloadEventCancel:
				msg = fmt.Sprintf(
					"[cancel]\t%s -> cancelled",
					id,
				)

			case core.EDownloadEventComplete:
				msg = fmt.Sprintf(
					"[done]\t%s -> completed (%s)",
					id,
					commands.FormatBytes(e.Bytes()),
				)

			case core.EDownloadEventError:
				errStr := "unknown error"
				if e.Error() != nil {
					errStr = e.Error().Error()
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
