package logging

import (
	"fmt"
	"io"
)

// Sink handles serialization and write/store logic of LogRecord object.
//
// Should be closed after use.
type Sink interface {
	Flush(record *LogRecord, errorOutput io.Writer)

	Close() error
}

// BaseSink implements Sink, defines single (formatter, appender) pair.
type BaseSink struct {
	formatter Formatter
	appender  Appender
	level     LogLevel
}

func NewBaseSink(
	formatter Formatter,
	appender Appender,
	level LogLevel,
) *BaseSink {
	return &BaseSink{formatter: formatter, appender: appender, level: level}
}

// Flush verifies log level and writes given record with defined appender
// in format provided by formatter.
func (b *BaseSink) Flush(record *LogRecord, errorOutput io.Writer) {
	if b.level > record.Level {
		return
	}

	data, err := b.formatter.Format(record)
	if err != nil {
		fmt.Fprintf(errorOutput, "Log formatting failed: %v\n", err)
		return
	}

	err = b.appender.Append(data)
	if err != nil {
		fmt.Fprintf(errorOutput, "Log write failed: %v\n", err)
	}
}

func (b *BaseSink) Close() error {
	return b.appender.Close()
}
