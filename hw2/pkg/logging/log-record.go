package logging

import "time"

type LogLevel uint8

/*
* Should be in order of increasing severity
 */
const (
	LEVEL_DEBUG   LogLevel = iota
	LEVEL_INFO             = iota
	LEVEL_WARNING          = iota
	LEVEL_ERROR            = iota
)

type LogRecord struct {
	Message   string
	Timestamp time.Time
	Level     LogLevel

	Properties map[string]string
}

func NewLogRecord(
	message string,
	timestamp time.Time,
	level LogLevel,
	properties map[string]string,
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
	case LEVEL_INFO:
		return "INFO"
	case LEVEL_WARNING:
		return "WARNING"
	case LEVEL_DEBUG:
		return "DEBUG"
	case LEVEL_ERROR:
		return "ERROR"
	default:
		return ""
	}
}
