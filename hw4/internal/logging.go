package internal

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogInitWithConsole is helper function to init logger to my preferences,
// not really a part of pkg
func LogInitWithConsole(file *os.File, debug bool) *zap.Logger {
	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	fileEncoder := zapcore.NewJSONEncoder(pe)

	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	fileCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(file),
		level,
	)

	core := zapcore.NewTee(consoleCore, fileCore)

	return zap.New(core)
}

// LogInit is helper function to init logger to my preferences
func LogInit(debug bool) *zap.Logger {
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level),
	)

	l := zap.New(core)

	return l
}
