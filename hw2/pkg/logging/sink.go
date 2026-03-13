package logging

import (
	"fmt"
	"io"
)

type Sink interface {
	Flush(record *LogRecord, errorOutput io.Writer)
}

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
