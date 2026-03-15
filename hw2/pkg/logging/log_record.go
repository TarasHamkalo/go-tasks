package logging

import "time"

type LogLevel uint8

// Should be in order of increasing severity
const (
	LevelDebug   LogLevel = iota
	LevelInfo    LogLevel = iota
	LevelWarning LogLevel = iota
	LevelError   LogLevel = iota
)

// LogRecord model of how single log entry represented.
type LogRecord struct {
	Message   string
	Timestamp time.Time
	Level     LogLevel

	// Properties field is array of key value pairs.
	//
	// Keys are located on even indexes (including zero).
	// Values are located on odd indexes.
	Properties []interface{}
}

func NewLogRecord(
	message string,
	timestamp time.Time,
	level LogLevel,
	properties []interface{},
) *LogRecord {
	return &LogRecord{
		Message:    message,
		Timestamp:  timestamp,
		Level:      level,
		Properties: properties,
	}
}

func (l LogLevel) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelDebug:
		return "DEBUG"
	case LevelError:
		return "ERROR"
	default:
		return ""
	}
}
