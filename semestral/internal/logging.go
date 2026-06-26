package internal

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)



// LogInit is helper function to init logger to my preferences
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

// LogInitWithConsole is helper function to init logger to my preferences
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
