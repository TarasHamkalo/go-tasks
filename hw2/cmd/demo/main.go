package main

import (
	. "hw2/pkg/logging"
)

func main() {
	log := NewBaseLogger(
		[]Sink{
			NewBaseSink(
				NewDefaultStagesFormatter(),
				NewConsoleAppender(),
				LevelInfo,
			),
			NewBaseSink(
				NewJsonFormatter(),
				NewConsoleAppender(),
				LevelDebug,
			),
		},
	)

	log.Debug("First message", "version", "1.0.0", "tag", "initial")
	log.Info("First message", "version", "1.0.0", "tag", "initial")
	log.Warning("First message", "version", "1.0.0", "tag", "initial")
	log.Error("First message", "version", "1.0.0", "tag", "initial")

}
