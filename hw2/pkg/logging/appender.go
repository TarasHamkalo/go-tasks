package logging

import "fmt"

type Appender interface {
	Append(record []byte)
}

type ConsoleAppender struct{}

func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{}
}

func (c *ConsoleAppender) Append(record []byte) {
	fmt.Println(string(record))
}
