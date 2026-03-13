package logging

import (
	"time"
)

type Logger interface {
	Info(msg string, structuredData ...interface{})

	Debug(msg string, structuredData ...interface{})

	Warning(msg string, structuredData ...interface{})

	Error(msg string, structuredData ...interface{})
}

type BaseLogger struct {
	sinks []Sink
}

func NewBaseLogger(sinks []Sink) *BaseLogger {
	return &BaseLogger{sinks: sinks}
}

func (b *BaseLogger) Info(msg string, structuredData ...interface{}) {
	b.log(LEVEL_INFO, msg, structuredData)
}

func (b *BaseLogger) Debug(msg string, structuredData ...interface{}) {
	b.log(LEVEL_DEBUG, msg, structuredData)
}

func (b *BaseLogger) Warning(msg string, structuredData ...interface{}) {
	b.log(LEVEL_WARNING, msg, structuredData)
}

func (b *BaseLogger) Error(msg string, structuredData ...interface{}) {
	b.log(LEVEL_ERROR, msg, structuredData)
}

func (b *BaseLogger) log(
	level LogLevel,
	msg string,
	structuredData []interface{},
) {
	if len(structuredData)%2 == 1 {
		panic("Either key or value missing in log record")
	}

	record := NewLogRecord(msg, time.Now(), level, structuredData)
	for _, sink := range b.sinks {
		sink.Flush(record)
	}
}
