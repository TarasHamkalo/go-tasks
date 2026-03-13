package logging

import (
	"fmt"
)

// ConsoleAppender implements Appender, writes log records to os.Stdout.
type ConsoleAppender struct{}

func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{}
}

func (c *ConsoleAppender) Append(record []byte) error {
	_, err := fmt.Println(string(record))
	return err
}

func (c *ConsoleAppender) Close() error {
	return nil
}
