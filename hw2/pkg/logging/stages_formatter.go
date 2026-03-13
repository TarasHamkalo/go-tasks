package logging

import (
	"bytes"
	"fmt"
	"time"
)

// stageFunc defines stage of LogRecord formating/serialization.
// Partial results are aggregated inside of output.
type stageFunc func(record *LogRecord, output *bytes.Buffer)

// StagesFormatterBuilder allows to build immutable StagesFormater object.
//
// Stages order is preserved, so different reordering provides different format.
type StagesFormatterBuilder struct {
	stages []stageFunc
}

// StagesFormatter formats/serializes LogRecord in stages, with given ordering.
// Stages output order is preserved.
type StagesFormatter struct {
	stages []stageFunc
}

func NewStagesFormatterBuilder() *StagesFormatterBuilder {
	return &StagesFormatterBuilder{
		stages: make([]stageFunc, 0, 4),
	}
}

// NewDefaultStagesFormatter build default colorized output.
func NewDefaultStagesFormatter() *StagesFormatter {
	return NewStagesFormatterBuilder().
		WithTimestamp(time.TimeOnly, Magenta).
		WithLogLevel(LogLevelColorMap).
		WithMessage(NoColor).
		WithProperties(Cyan, Green).
		Build()
}

func NewNoColorStagesFormatter() *StagesFormatter {
	return NewStagesFormatterBuilder().
		WithTimestamp(time.TimeOnly, NoColor).
		WithLogLevel(map[LogLevel]AnsiColor{}).
		WithMessage(NoColor).
		WithProperties(NoColor, NoColor).
		Build()
}

// WithTimestamp add Timestamp formatting stage.
// It is, add timestamp to output with given layout and color.
func (s *StagesFormatterBuilder) WithTimestamp(
	layout string,
	color AnsiColor,
) *StagesFormatterBuilder {

	s.stages = append(s.stages, func(record *LogRecord, output *bytes.Buffer) {
		timestamp := record.Timestamp.Format(layout)

		output.WriteString(string(color))
		output.WriteString(timestamp)
		output.WriteString(string(Reset))
		output.WriteByte(' ')
	})

	return s
}

// WithLogLevel add LogLevel formatting stage.
// It is, add LogLevel string to output with colors defined colorMap.
func (s *StagesFormatterBuilder) WithLogLevel(
	colorMap map[LogLevel]AnsiColor,
) *StagesFormatterBuilder {
	s.stages = append(s.stages, func(record *LogRecord, output *bytes.Buffer) {
		color, ok := colorMap[record.Level]
		if !ok {
			color = NoColor
		}

		output.WriteString(string(color))
		output.WriteString(record.Level.String())
		output.WriteString(string(Reset))
		output.WriteByte(' ')
	})

	return s
}

// WithMessage add Message formatting stage.
// It is, add Message string to output with given color.
func (s *StagesFormatterBuilder) WithMessage(
	color AnsiColor,
) *StagesFormatterBuilder {
	s.stages = append(s.stages, func(record *LogRecord, output *bytes.Buffer) {
		output.WriteString(string(color))
		output.WriteString(record.Message)
		output.WriteString(string(Reset))
		output.WriteByte(' ')
	})

	return s
}

// WithProperties add Properties formatting stage.
// It is, add property (key=value) pairs to output with given colors.
func (s *StagesFormatterBuilder) WithProperties(
	keyColor AnsiColor,
	valueColor AnsiColor,
) *StagesFormatterBuilder {

	s.stages = append(s.stages, func(record *LogRecord, output *bytes.Buffer) {
		output.WriteString("\t\t")
		propertiesCount := len(record.Properties) / 2
		for i := 0; i < propertiesCount; i++ {

			key := record.Properties[i*2]
			value := record.Properties[i*2+1]

			output.WriteString(string(keyColor))
			output.WriteString(fmt.Sprint(key))
			output.WriteString(string(Reset))

			output.WriteByte('=')

			output.WriteString(string(valueColor))
			output.WriteString(fmt.Sprint(value))
			output.WriteString(string(Reset))

			output.WriteByte(' ')
		}
	})

	return s
}

// Build immutable StagesFormatter object.
func (s *StagesFormatterBuilder) Build() *StagesFormatter {
	return &StagesFormatter{
		stages: append([]stageFunc{}, s.stages...),
	}
}

func (s *StagesFormatter) Format(record *LogRecord) ([]byte, error) {
	var output bytes.Buffer
	output.Grow(len(record.Message) + 32) // at least message and timestamp
	for _, stage := range s.stages {
		stage(record, &output)
	}

	return output.Bytes(), nil
}
