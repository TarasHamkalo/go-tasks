package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Logger handles LogRecord constructions and
// unifies logic of LogRecord serialization as well as write/store.
//
// Should be closed after use.
type Logger interface {
	Info(msg string, structuredData ...interface{})

	Debug(msg string, structuredData ...interface{})

	Warning(msg string, structuredData ...interface{})

	Error(msg string, structuredData ...interface{})

	Close() error
}

// BaseLogger implements Logger interface.
type BaseLogger struct {
	sinks []Sink // all registered sinks see Sink

	errorOutput io.Writer // were to write errors occurred during logging
}

func NewBaseLogger(sinks []Sink, errorOutput io.Writer) *BaseLogger {
	return &BaseLogger{sinks: sinks, errorOutput: errorOutput}
}

// NewCleanLogger constructs BaseLogger with no sinks set and
// os.Stderr as errorOutput.
func NewCleanLogger() *BaseLogger {
	return NewBaseLogger(make([]Sink, 0, 3), os.Stderr)
}

// WithSink registers provided sink to logger instance.
func (b *BaseLogger) WithSink(sink Sink) *BaseLogger {
	b.sinks = append(b.sinks, sink)
	return b
}

// WithConsoleSink adds console sink to existing base logger object.
func (b *BaseLogger) WithConsoleSink(
	level LogLevel,
	colorize bool,
) *BaseLogger {
	var formatter Formatter
	if colorize {
		formatter = NewDefaultStagesFormatter()
	} else {
		formatter = NewNoColorStagesFormatter()
	}

	b.sinks = append(
		b.sinks,
		NewBaseSink(
			formatter,
			NewConsoleAppender(),
			level,
		),
	)

	return b
}

// WithJsonFileSink adds json sink to existing base logger object.
func (b *BaseLogger) WithJsonFileSink(path string, level LogLevel) *BaseLogger {
	fileAppender, err := NewFileAppender(path)
	if err != nil {
		// failure of logging setup, just panic
		panic(fmt.Errorf("could not create file appender: %w", err))
	}

	b.sinks = append(
		b.sinks,
		NewBaseSink(
			NewJsonFormatter(),
			fileAppender,
			level,
		),
	)

	return b
}

// WithFileSink adds file sink to existing base logger object.
func (b *BaseLogger) WithFileSink(
	path string,
	level LogLevel,
	colorize bool,
) *BaseLogger {

	fileAppender, err := NewFileAppender(path)
	if err != nil {
		// failure of logging setup, just panic
		panic(fmt.Errorf("could not create file appender: %w", err))
	}

	var formatter Formatter
	if colorize {
		formatter = NewDefaultStagesFormatter()
	} else {
		formatter = NewNoColorStagesFormatter()
	}

	b.sinks = append(
		b.sinks,
		NewBaseSink(
			formatter,
			fileAppender,
			level,
		),
	)

	return b
}

// Close iteratively closes all registered sinks.
//
// All errors are joined as well as logged to errorOutput.
func (b *BaseLogger) Close() error {
	var errs []error
	for _, sink := range b.sinks {
		err := sink.Close()
		if err != nil {
			errs = append(errs, err)
			fmt.Fprintf(b.errorOutput, "could not close sink: %v\n", err)
		}
	}

	return errors.Join(errs...)
}

func (b *BaseLogger) Info(msg string, structuredData ...interface{}) {
	b.log(LevelInfo, msg, structuredData...)
}

func (b *BaseLogger) Debug(msg string, structuredData ...interface{}) {
	b.log(LevelDebug, msg, structuredData...)
}

func (b *BaseLogger) Warning(msg string, structuredData ...interface{}) {
	b.log(LevelWarning, msg, structuredData...)
}

func (b *BaseLogger) Error(msg string, structuredData ...interface{}) {
	b.log(LevelError, msg, structuredData...)
}

// log constructs LogRecord and passes down
// given record to all registered sinks.
func (b *BaseLogger) log(
	level LogLevel,
	msg string,
	structuredData ...interface{},
) {
	// (key, value) pairs, total number of elements should be even
	if len(structuredData)%2 == 1 {
		fmt.Fprintf(
			b.errorOutput,
			"Either key or value missing in log record\n",
		)
		return
	}

	record := NewLogRecord(msg, time.Now(), level, structuredData)
	for _, sink := range b.sinks {
		sink.Flush(record, b.errorOutput)
	}
}
