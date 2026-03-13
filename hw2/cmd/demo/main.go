package main

import (
	"fmt"
	. "hw2/pkg/logging"
	"os"
)

func main() {
	fileAppender, err := NewFileAppender("logs/app-log.jsonl")
	if err != nil {
		panic(err)
	}

	defer func(fileAppender *FileAppender) {
		err = fileAppender.Close()
		if err != nil {
			fmt.Println(err.Error())
		}
	}(fileAppender)

	log := NewBaseLogger(
		[]Sink{
			NewBaseSink(
				NewDefaultStagesFormatter(),
				NewConsoleAppender(),
				LevelInfo,
			),
			NewBaseSink(
				NewJsonFormatter(),
				fileAppender,
				LevelDebug,
			),
		},
		os.Stderr,
	)

	log.Debug("Debug message", "version", "1.0.0", "tag", os.Stderr)
	log.Info("Info message", "version", "1.0.0", "tag", "initial")
	log.Warning("Warning message", "version", "1.0.0", "tag", "initial")
	log.Error("Error message", "version", "1.0.0", "tag", "initial")

}
