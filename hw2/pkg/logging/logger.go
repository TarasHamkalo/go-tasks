package logging

import (
	"fmt"
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

	keysCount := len(structuredData) / 2
	properties := make(map[string]string, keysCount)
	for i := 0; i < keysCount; i++ {
		key := fmt.Sprint(structuredData[2*i])
		properties[key] = fmt.Sprint(structuredData[2*i+1])
	}

	record := NewLogRecord(msg, time.Now(), level, properties)
	for _, sink := range b.sinks {
		sink.Flush(record)
	}
}
