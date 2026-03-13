package logging

import "fmt"

type Appender interface {
	Append(record string)
}

type ConsoleAppender struct{}

func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{}
}

func (c *ConsoleAppender) Append(record string) {
	fmt.Println(record)
}
