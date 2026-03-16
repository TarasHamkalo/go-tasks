package logging

// Appender handles write/store logic of log record, should be closed after use.
type Appender interface {
	Append(record []byte) error

	Close() error
}
