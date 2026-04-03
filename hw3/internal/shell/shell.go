package shell

import (
	"context"
	"downloader/internal"
	"downloader/internal/core"
	"downloader/internal/shell/commands"
	"fmt"
	"os"
	"time"

	"github.com/c-bata/go-prompt"
	"go.uber.org/zap"
)

type Shell struct {
	commandsChain commands.Command

	downloader *core.Downloader

	downloaderLogFile *os.File

	shellLogFile *os.File

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
	chain := commands.NewDownloadCommand(d).
		WithNext(commands.NewStatusCommand(d)).
		WithNext(commands.NewCancelCommand(d))

	return &Shell{
		commandsChain:     chain,
		downloader:        d,
		downloaderLogFile: downloadLogFile,
		shellLogFile:      shellLogFile,
		logger:            shellLogger,
	}
}

func (s *Shell) execute(input string) {
	if input == "" {
		return
	}

	if input == "exit" {
		fmt.Println("Shutting down downloader (timeout 5s)...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		s.downloader.Shutdown(ctx)

		fmt.Println("Shutdown complete.")
		os.Exit(0)
		return
	}

	err := s.commandsChain.Handle(input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func (s *Shell) completer(d prompt.Document) []prompt.Suggest {
	return s.commandsChain.CompletePrompt(d.Text)
}
func (s *Shell) Run() error {
	defer s.downloaderLogFile.Close()
	defer s.shellLogFile.Close()
	msg := make(chan string)

	go func() {
		events := s.downloader.Subscribe()
		for e := range events {
			msg <- fmt.Sprintf(
				"[event] %s %s\n",
				e.DownloadId(),
				e.Type(),
			)
		}
	}()

	p := prompt.New(
		s.execute,
		s.completer,
		prompt.OptionPrefix(">>> "),
		prompt.OptionTitle("downloader-shell"),
		prompt.OptionWithAsyncMessageChan(msg),
	)

	p.Run()

	return nil
}
