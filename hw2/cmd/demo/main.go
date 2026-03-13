package main

import "hw2/pkg/logging"

func main() {
	log := logging.NewBaseLogger(
		[]logging.Sink{
			logging.NewBaseSink(
				logging.NewBaseFormatter(),
				logging.NewConsoleAppender(),
				logging.LEVEL_INFO,
			),
		},
	)

	log.Debug("First message", "version", "1.0.0", "tag", "initial")
	log.Info("First message", "version", "1.0.0", "tag", "initial")
	log.Warning("First message", "version", "1.0.0", "tag", "initial")
	log.Error("First message", "version", "1.0.0", "tag", "initial")

}
