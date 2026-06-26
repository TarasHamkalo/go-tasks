package commands

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
)

// Command represent CLI command with ability to chain arbitrary.
// Each command defines a match rule and code to handle user input.
type Command interface {
	// Handle only one command in chain handles given prompt
	Handle(string) error

	// CompletePrompt all commands in chain suggest input completions
	CompletePrompt(string) []prompt.Suggest

	// Matches whether given command has to be executed (Handle called)
	Matches(string) bool

	// WithNext should set provided command as next of current and return next
	// to allow chain assignment
	WithNext(Command) Command

	Next() Command
}

// BaseCommand is base implementation all concrete commands can inherit from.
// Implements expected by Command interface WithNext, Handle, CompletePrompt
// behavior (boilerplate) and delegates concrete Matches, Handle, SuggestArguments
// to concrete implementation using BaseCommandHandler.
type BaseCommand struct {
	name        string
	description string

	next Command

	handler *BaseCommandHandler
}

// BaseCommandHandler aggregates all concrete handlers,
// expects them to be non-null.
type BaseCommandHandler struct {
	handle           func(string)
	matches          func(string) bool
	suggestArguments func([]string) []prompt.Suggest
}

// NewBaseCommand constructs base command with provided handler
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
WithNext sets next and returns it, expected usage is:

	head := NewCommand()
	head.
		WithNext(NewCommand()).
		WithNext(NewCommand())
*/
func (cmd *BaseCommand) WithNext(next Command) Command {
	cmd.next = next
	return next
}

func (cmd *BaseCommand) Next() Command {
	return cmd.next
}
