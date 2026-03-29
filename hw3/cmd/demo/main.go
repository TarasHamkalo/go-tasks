package main

import (
	"fmt"
	"time"

	prompt "github.com/c-bata/go-prompt"
)
import "sync"

var mu sync.Mutex
var LivePrefixState struct {
	LivePrefix string
	IsEnable   bool
}

func executor(in string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Println("Your input: " + in)
	if in == "" {
		LivePrefixState.IsEnable = false
		LivePrefixState.LivePrefix = in
		return
	}
	LivePrefixState.LivePrefix = in + "> "
	LivePrefixState.IsEnable = true
}

func completer(in prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "users", Description: "Store the username and age"},
		{Text: "articles", Description: "Store the article text posted by user"},
		{Text: "comments", Description: "Store the text commented to articles"},
		{Text: "groups", Description: "Combine users with specific rules"},
	}
	return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
}

func changeLivePrefix() (string, bool) {
	return LivePrefixState.LivePrefix, LivePrefixState.IsEnable
}

func runRandomPrinter(writer prompt.ConsoleWriter) {
	for {
		mu.Lock()

		writer.WriteRawStr("\n")
		writer.WriteStr("Hello from my custom output!")
		writer.WriteRawStr("\n")
		writer.Flush()

		mu.Unlock()

		time.Sleep(2 * time.Second)
	}
}

func main() {

	//writer := prompt.NewStdoutWriter()
	//go runRandomPrinter(writer)
	//prompt.Input()
	//prompt.Input()
	msg := make(chan string)
	p := prompt.New(
		executor,
		completer,
		prompt.OptionPrefix(">>> "),
		prompt.OptionLivePrefix(changeLivePrefix),
		prompt.OptionTitle("live-prefix-example"),
		prompt.OptionWithAsyncMessageChan(msg),
	)

	go (func() {
		time.Sleep(2 * time.Second)
		msg <- "[event] super important\n[event] super super important"
	})()

	p.Input()
}
