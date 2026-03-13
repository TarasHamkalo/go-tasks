package logging

type Sink interface {
	Flush(record *LogRecord)
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

func (b *BaseSink) Flush(record *LogRecord) {
	if b.level > record.Level {
		return
	}

	formatted := b.formatter.Format(record)
	b.appender.Append(formatted)
}
