package logging

import (
	"strconv"
	"strings"
	"time"
)

type AnsiColor string

const (
	NO_COLOR AnsiColor = ""
	RESET    AnsiColor = "\033[0m"

	BLACK   AnsiColor = "\033[30m"
	RED     AnsiColor = "\033[31m"
	GREEN   AnsiColor = "\033[32m"
	YELLOW  AnsiColor = "\033[33m"
	BLUE    AnsiColor = "\033[34m"
	MAGENTA AnsiColor = "\033[35m"
	CYAN    AnsiColor = "\033[36m"
	WHITE   AnsiColor = "\033[37m"

	BRIGHT_RED    AnsiColor = "\033[91m"
	BRIGHT_GREEN  AnsiColor = "\033[92m"
	BRIGHT_YELLOW AnsiColor = "\033[93m"
	BRIGHT_BLUE   AnsiColor = "\033[94m"
)

var LOG_LEVEL_COLOR_MAP = map[LogLevel]AnsiColor{
	LEVEL_DEBUG:   BRIGHT_BLUE,
	LEVEL_INFO:    BRIGHT_GREEN,
	LEVEL_WARNING: BRIGHT_YELLOW,
	LEVEL_ERROR:   BRIGHT_RED,
}

type Formatter interface {
	Format(record *LogRecord) string
}

type stageFunc func(record *LogRecord, output *strings.Builder)

type StagesFormatterBuilder struct {
	stages []stageFunc
}

type StagesFormatter struct {
	stages []stageFunc
}

func NewStagesFormatterBuilder() *StagesFormatterBuilder {
	return &StagesFormatterBuilder{
		stages: make([]stageFunc, 0, 4),
	}
}

func NewDefaultStagesFormatter() *StagesFormatter {
	return NewStagesFormatterBuilder().
		WithTimestamp(time.TimeOnly, MAGENTA).
		WithLogLevel(LOG_LEVEL_COLOR_MAP).
		WithMessage(NO_COLOR).
		WithProperties(CYAN, GREEN).
		Build()
}

func (s *StagesFormatterBuilder) WithTimestamp(
	layout string,
	color AnsiColor,
) *StagesFormatterBuilder {

	s.stages = append(s.stages, func(record *LogRecord, output *strings.Builder) {
		timestamp := record.Timestamp.Format(layout)

		output.WriteString(string(color))
		output.WriteString(timestamp)
		output.WriteString(string(RESET))
		output.WriteByte(' ')
	})

	return s
}

func (s *StagesFormatterBuilder) WithLogLevel(
	colorMap map[LogLevel]AnsiColor,
) *StagesFormatterBuilder {
	s.stages = append(s.stages, func(record *LogRecord, output *strings.Builder) {
		color, ok := colorMap[record.Level]
		if !ok {
			color = LOG_LEVEL_COLOR_MAP[record.Level]
		}

		output.WriteString(string(color))
		output.WriteString(record.Level.String())
		output.WriteString(string(RESET))
		output.WriteByte(' ')
	})

	return s
}

func (s *StagesFormatterBuilder) WithMessage(
	color AnsiColor,
) *StagesFormatterBuilder {
	s.stages = append(s.stages, func(record *LogRecord, output *strings.Builder) {
		output.WriteString(string(color))
		output.WriteString(record.Message)
		output.WriteString(string(RESET))
		output.WriteByte(' ')
	})

	return s
}

func (s *StagesFormatterBuilder) WithProperties(
	keyColor AnsiColor,
	valueColor AnsiColor,
) *StagesFormatterBuilder {

	s.stages = append(s.stages, func(record *LogRecord, output *strings.Builder) {
		output.WriteString("\t\t")
		propertiesCount := len(record.Properties) / 2
		for i := 0; i < propertiesCount; i++ {

			key := record.Properties[i*2]
			value := record.Properties[i*2+1]

			output.WriteString(string(keyColor))
			writeAny(output, key)
			output.WriteString(string(RESET))

			output.WriteByte('=')

			output.WriteString(string(valueColor))
			writeAny(output, value)
			output.WriteString(string(RESET))

			output.WriteByte(' ')
		}
	})

	return s
}

func (s *StagesFormatterBuilder) Build() *StagesFormatter {
	return &StagesFormatter{
		stages: s.stages,
	}
}

func (s *StagesFormatter) Format(record *LogRecord) string {
	var output strings.Builder
	output.Grow(len(record.Message) + 32) // at least timestamp
	for _, stage := range s.stages {
		stage(record, &output)
	}

	return output.String()
}

func writeAny(b *strings.Builder, v any) {
	switch x := v.(type) {
	case string:
		b.WriteString(x)
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		b.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
	case bool:
		b.WriteString(strconv.FormatBool(x))
	default:
		b.WriteString("<?>")
	}
}
