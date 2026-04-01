package downloader

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func LogInit(debug bool, file *os.File) *zap.SugaredLogger {
	pe := zap.NewProductionEncoderConfig()
	fileEncoder := zapcore.NewJSONEncoder(pe)
	pe.EncodeTime = zapcore.ISO8601TimeEncoder

	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, zapcore.AddSync(file), level),
	)

	l := zap.New(core)

	return l.Sugar()
}
