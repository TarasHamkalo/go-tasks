package logging

import (
	"fmt"
	"strings"
)

type Formatter interface {
	Format(record *LogRecord) string
}

type BaseFormatter struct{}

func NewBaseFormatter() *BaseFormatter {
	return &BaseFormatter{}
}

func (b *BaseFormatter) Format(record *LogRecord) string {
	return fmt.Sprintf(
		"%v [%s] %s\t%s",
		record.Timestamp,
		record.Level,
		record.Message,
		BuildPropertiesString(record.Properties),
	)
}

func BuildPropertiesString(properties map[string]string) string {
	builder := strings.Builder{}
	for key, value := range properties {
		builder.WriteString(fmt.Sprintf("%v=%v; ", key, value))
	}

	return builder.String()
}
