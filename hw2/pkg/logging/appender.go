package logging

type Appender interface {
	Append(record []byte) error

	Close() error
}
