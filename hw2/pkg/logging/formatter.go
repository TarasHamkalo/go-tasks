package logging

import (
	"fmt"
	"strings"
	"time"
)

type AnsiColor string

const (
	NONE  AnsiColor = ""
	RESET AnsiColor = "\033[0m"

	// basic colors
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

type FormatterStage int

const (
	TIMESTAMP  FormatterStage = iota
	LEVEL      FormatterStage = iota
	MESSAGE    FormatterStage = iota
	PROPERTIES FormatterStage = iota
)

var LOG_LEVEL_COLOR_MAP map[LogLevel]AnsiColor = map[LogLevel]AnsiColor{
	LEVEL_DEBUG:   BRIGHT_BLUE,
	LEVEL_INFO:    BRIGHT_GREEN,
	LEVEL_WARNING: BRIGHT_YELLOW,
	LEVEL_ERROR:   BRIGHT_RED,
}

type Formatter interface {
	Format(record *LogRecord) string
}

type StagesFormatter struct {
	stages      map[FormatterStage]func(record *LogRecord, output *strings.Builder)
	stagesOrder []FormatterStage
}

func (s *StagesFormatter) WithTimestamp(
	layout string,
	color AnsiColor,
) *StagesFormatter {
	if _, ok := s.stages[TIMESTAMP]; !ok {
		s.stagesOrder = append(s.stagesOrder, TIMESTAMP)
	}

	s.stages[TIMESTAMP] = func(record *LogRecord, output *strings.Builder) {
		timestamp := record.Timestamp.Format(layout)
		output.WriteString(fmt.Sprintf("%s%s%s ", color, timestamp, RESET))
	}

	return s
}

func (s *StagesFormatter) WithLogLevel(
	colorMap map[LogLevel]AnsiColor,
) *StagesFormatter {
	if _, ok := s.stages[LEVEL]; !ok {
		s.stagesOrder = append(s.stagesOrder, LEVEL)
	}

	s.stages[LEVEL] = func(record *LogRecord, output *strings.Builder) {
		color, isPresent := colorMap[record.Level]
		if !isPresent {
			color = LOG_LEVEL_COLOR_MAP[record.Level]
		}

		output.WriteString(
			fmt.Sprintf("%s%s%s ", color, record.Level, RESET),
		)
	}

	return s
}

func (s *StagesFormatter) WithMessage(color AnsiColor) *StagesFormatter {
	if _, ok := s.stages[MESSAGE]; !ok {
		s.stagesOrder = append(s.stagesOrder, MESSAGE)
	}

	s.stages[MESSAGE] = func(record *LogRecord, output *strings.Builder) {
		output.WriteString(
			fmt.Sprintf("%s%s%s ", color, record.Message, RESET),
		)
	}

	return s
}

func (s *StagesFormatter) WithProperties(
	keyColor AnsiColor,
	valueColor AnsiColor,
) *StagesFormatter {
	if _, ok := s.stages[PROPERTIES]; !ok {
		s.stagesOrder = append(s.stagesOrder, PROPERTIES)
	}

	s.stages[PROPERTIES] = func(record *LogRecord, output *strings.Builder) {
		output.WriteString("\t\t")
		for key, value := range record.Properties {
			output.WriteString(
				fmt.Sprintf(
					"%s%s%s=%s%s%s ", keyColor, key, RESET, valueColor, value, RESET,
				),
			)
		}
	}
	return s
}

func NewDefaultStagesFormatter() *StagesFormatter {
	s := &StagesFormatter{
		stages: make(
			map[FormatterStage]func(record *LogRecord, output *strings.Builder), 4,
		),
	}

	return s.
		WithTimestamp(time.TimeOnly, MAGENTA).
		WithLogLevel(LOG_LEVEL_COLOR_MAP).
		WithMessage(NONE).
		WithProperties(CYAN, GREEN)
}

func NewStagesFormatter() *StagesFormatter {
	return &StagesFormatter{
		stages: make(
			map[FormatterStage]func(record *LogRecord, output *strings.Builder), 4,
		),
		stagesOrder: make([]FormatterStage, 0, 4),
	}
}

func (s *StagesFormatter) Format(record *LogRecord) string {
	output := &strings.Builder{}
	output.Grow(len(record.Message) + len(time.TimeOnly))
	for _, stage := range s.stagesOrder {
		s.stages[stage](record, output)
	}

	return output.String()
}
