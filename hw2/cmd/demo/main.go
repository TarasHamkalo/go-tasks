package main

import (
	"fmt"
	. "hw2/pkg/logging"
	"os"
)

func main() {
	log := NewCleanLogger().
		WithConsoleSink(LevelWarning, false).
		WithFileSink("logs/plaintext.log", LevelError, true).
		WithJsonFileSink("logs/jlogs.jsonl", LevelDebug)

	defer func(log *BaseLogger) {
		err := log.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not close logger: %v\n", err)
		}
	}(log)

	log.Debug("Debug message", "version", "1.0.0", "tag", "initial")
	log.Info("Info message", "version", "1.0.0", "tag", "initial")
	log.Warning("Warning message", "version", "1.0.0", "tag", "initial")
	log.Error("Error message", "version", "1.0.0", "tag", "initial")

}
