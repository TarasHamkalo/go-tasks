package commands

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
)

type Command interface {
	// Handle only one command in chain handles given prompt
	Handle(string) error

	// CompletePrompt all commands in chain suggest
	CompletePrompt(string) []prompt.Suggest

	Matches(string) bool

	WithNext(Command) Command

	Next() Command
}

type BaseCommand struct {
	name        string
	description string

	next Command

	handler *BaseCommandHandler
}

type BaseCommandHandler struct {
	handle           func(string)
	matches          func(string) bool
	suggestArguments func([]string) []prompt.Suggest
}

func NewBaseCommand(
	name string,
	description string,
	handler *BaseCommandHandler,
) *BaseCommand {
	return &BaseCommand{
		name:        name,
		description: description,
		handler:     handler,
		next:        nil,
	}
}

func (cmd *BaseCommand) Matches(s string) bool {
	return cmd.handler.matches(s)
}

func (cmd *BaseCommand) Handle(s string) error {
	if cmd.Matches(s) {
		cmd.handler.handle(s)
		return nil
	}

	if cmd.next == nil {
		return fmt.Errorf("no such command")
	}

	return cmd.next.Handle(s)
}

func (cmd *BaseCommand) CompletePrompt(s string) []prompt.Suggest {
	// next complete, merge with ours
	suggestions := make([]prompt.Suggest, 0)
	if cmd.next != nil {
		suggestions = append(suggestions, cmd.next.CompletePrompt(s)...)
	}

	parts := strings.Split(s, " ")
	if parts[0] == cmd.name {
		// already filled command name, return arguments suggestion if any
		return append(suggestions, cmd.handler.suggestArguments(parts)...)
	}

	if strings.HasPrefix(cmd.name, parts[0]) {
		return append(suggestions, prompt.Suggest{
			Text:        cmd.name,
			Description: cmd.description,
		})
	}

	return suggestions
}

/*
WithNext usage:

	head := first
	first.
		WithNext(second).
		WithNext(third)
*/
func (cmd *BaseCommand) WithNext(next Command) Command {
	cmd.next = next
	return next
}

func (cmd *BaseCommand) Next() Command {
	return cmd.next
}
