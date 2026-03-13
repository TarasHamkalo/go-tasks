package main

import (
	"fmt"
	. "hw2/pkg/logging"
	"os"
	"time"
)

// DemoLogger used to log content not related to examples
var DemoLogger = NewCleanLogger().
	WithSink(
		NewBaseSink(
			NewStagesFormatterBuilder().
				WithMessage(Yellow).
				WithProperties(Cyan, Magenta).
				Build(),
			NewConsoleAppender(),
			LevelDebug,
		),
	)

func DemoSection(title string) {
	DemoLogger.Info("------------------------------------------------------------")
	DemoLogger.Info(title)
	DemoLogger.Info("------------------------------------------------------------")
}

// Create formatter and configure logger with it
type SimpleFormatter struct{}

func (f *SimpleFormatter) Format(record *LogRecord) ([]byte, error) {
	out := fmt.Sprintf(
		"[%s] %s: %s",
		record.Timestamp.Format(time.RFC3339),
		record.Level.String(),
		record.Message,
	)

	return []byte(out), nil
}

func ExampleCustomFormatter() {
	DemoSection("Example: create and register custom Formatter")
	log := NewCleanLogger().
		WithSink(
			NewBaseSink(
				&SimpleFormatter{},
				NewConsoleAppender(),
				LevelDebug,
			),
		)

	defer log.Close()
	log.Info("Using custom formatter")
}

//------------------------------------------------------------------------------

// Create custom appender and configure logger with it
type MemoryAppender struct {
	Records [][]byte
}

func NewMemoryAppender() *MemoryAppender {
	return &MemoryAppender{
		Records: make([][]byte, 0),
	}
}

func (m *MemoryAppender) Append(record []byte) error {
	m.Records = append(m.Records, record)
	return nil
}

func (m *MemoryAppender) Close() error {
	return nil
}

func ExampleCustomAppender() {
	DemoSection(
		"Example: create and register custom appender (store messages in memory)",
	)

	appender := NewMemoryAppender()
	formatter := NewJsonFormatter()

	sink := NewBaseSink(formatter, appender, LevelDebug)

	log := NewBaseLogger([]Sink{sink}, os.Stderr)
	defer log.Close()

	log.Info("Stored in memory")
	log.Info("Stored in memory")

	DemoLogger.Info("Stored records", "n", len(appender.Records))
}

// ------------------------------------------------------------------------------

// Log levels example
func ExampleLogLevels() {
	DemoSection("Example: log levels (warning threshold)")
	log := NewCleanLogger().
		WithConsoleSink(LevelWarning, true) // warning level, colorize=true

	defer log.Close()

	log.Debug("debug message - will NOT appear")
	log.Info("info message - will NOT appear")
	log.Warning("warning message - will appear")
	log.Error("error message - will appear")
}

// Log to multiple destinations with different levels and colorization
func ExampleMultipleSinks() {
	DemoSection(
		"Example: multiple log destinations with " +
			"different levels (console + json + file)",
	)

	DemoLogger.Info(
		"Files are:", "json", "logs/all-logs.jsonl", "plaintext", "logs/error.log",
	)

	log := NewCleanLogger().
		WithConsoleSink(LevelWarning, true).
		WithJsonFileSink("logs/all-logs.jsonl", LevelDebug).
		WithFileSink("logs/error.log", LevelError, false)

	defer log.Close()

	log.Debug("Debug message", "version", "1.0.0", "tag", "initial")
	log.Info("Info message", "version", "1.0.0", "tag", "initial")
	log.Warning("Warning message", "version", "1.0.0", "tag", "initial")
	log.Error("Error message", "version", "1.0.0", "tag", "initial")
}

func ExampleConsoleSink() {
	DemoSection("Example: console sink (colorized vs clean)")

	logColorized := NewCleanLogger().
		WithConsoleSink(LevelDebug, true)

	logClean := NewCleanLogger().
		WithConsoleSink(LevelDebug, false)

	defer logColorized.Close()
	defer logClean.Close()

	logColorized.Info("hello [colorized] console")
	logClean.Info("hello [clean] console")
}

func ExampleCustomTextFormat() {
	DemoSection("Example: custom text formatting (colors per field and ordering)")
	// specify colors per log level
	customLogLevelColorMap := map[LogLevel]AnsiColor{
		LevelDebug:   BrightYellow,
		LevelInfo:    BrightRed,
		LevelWarning: White,
		LevelError:   BrightGreen,
	}

	log := NewCleanLogger().
		WithSink(
			NewBaseSink(
				NewStagesFormatterBuilder().
					WithLogLevel(customLogLevelColorMap).
					WithMessage(Green).
					WithTimestamp(time.DateOnly, Cyan).
					WithProperties(Black, White).
					Build(),
				NewConsoleAppender(),
				LevelDebug,
			),
		)

	defer log.Close()

	log.Debug("debug colored", "prop", "value")
	log.Info("info colored", "prop", "value")
	log.Warning("warning colored", "prop", "value")
	log.Error("error colored", "prop", "value")
}

func ExampleFileLogging() {
	DemoSection("Example: file logging (to \"logs/app.log\")")

	log := NewCleanLogger().
		WithFileSink("logs/app.log", LevelDebug, false)

	defer log.Close()

	log.Info("application started")
	log.Error("something failed")
}

func main() {
	ExampleCustomFormatter()
	ExampleCustomAppender()
	ExampleLogLevels()
	ExampleConsoleSink()
	ExampleMultipleSinks()
	ExampleCustomTextFormat()
	ExampleFileLogging()
}
