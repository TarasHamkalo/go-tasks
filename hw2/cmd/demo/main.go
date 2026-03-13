package main

import (
	. "hw2/pkg/logging"
	"time"
)

func main() {
	log := NewBaseLogger(
		[]Sink{
			NewBaseSink(
				NewStagesFormatterBuilder().
					WithMessage(GREEN).
					WithTimestamp(time.DateOnly, RED).
					WithLogLevel(map[LogLevel]AnsiColor{
						LEVEL_INFO: WHITE,
					}).
					Build(),
				NewConsoleAppender(),
				LEVEL_INFO,
			),
		},
	)

	log.Debug("First message", "version", "1.0.0", "tag", "initial")
	log.Info("First message", "version", "1.0.0", "tag", "initial")
	log.Warning("First message", "version", "1.0.0", "tag", "initial")
	log.Error("First message", "version", "1.0.0", "tag", "initial")

}
