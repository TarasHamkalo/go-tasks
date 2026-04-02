package internal

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func LogInit(file *os.File, debug bool) *zap.Logger {
	pe := zap.NewProductionEncoderConfig()
	//fileEncoder := zapcore.NewJSONEncoder(pe)
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	pe.EncodeTime = zapcore.ISO8601TimeEncoder

	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(file), level),
	)

	l := zap.New(core)

	return l
}
