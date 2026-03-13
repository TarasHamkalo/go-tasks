package logging

import "fmt"

type ConsoleAppender struct{}

func NewConsoleAppender() *ConsoleAppender {
	return &ConsoleAppender{}
}

func (c *ConsoleAppender) Append(record []byte) error {
	fmt.Println(string(record))
	return nil
}
