package commands

import (
	"context"
	"downloader/pkg/downloader"
	"fmt"
	"regexp"

	"github.com/c-bata/go-prompt"
)

// DownloadCommand provides implementation of
// "download <url> <destination>" command.
type DownloadCommand struct {
	downloader *downloader.Downloader

	// pattern used to match input to download command, see constructors
	pattern *regexp.Regexp

	// destinationsHistory stores previously accepted destinations
	destinationsHistory []prompt.Suggest

	// urlsHistory stores previously accepted urls
	urlsHistory []prompt.Suggest

	*BaseCommand
}

// NewDownloadCommand constructs default download command
func NewDownloadCommand(downloader *downloader.Downloader) *DownloadCommand {
	cmd := &DownloadCommand{
		downloader: downloader,
		pattern: regexp.MustCompile(
			"^download\\s(?P<url>\\S{1,150})\\s(?P<destination>.{1,150})?$",
		),
	}

	cmd.BaseCommand = NewBaseCommand(
		"download",
		"download <url> <destination>, max 150 chars per field",
		&BaseCommandHandler{
			handle:           cmd.handle,
			matches:          cmd.matches,
			suggestArguments: cmd.suggestArguments,
		},
	)

	return cmd
}

func (cmd *DownloadCommand) matches(s string) bool {
	return cmd.pattern.MatchString(s)
}

func (cmd *DownloadCommand) handle(s string) {
	// parse named regex groups
	matches := cmd.pattern.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range cmd.pattern.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}

	// TODO: extend command parameters to include user defined timeout
	downloadId, err := cmd.downloader.SubmitDownload(
		context.TODO(), result["url"], result["destination"],
	)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Printf("Download submitted with id: %s\n", downloadId)

	cmd.urlsHistory = append(
		cmd.urlsHistory,
		prompt.Suggest{Text: result["url"], Description: "Download url"},
	)

	cmd.destinationsHistory = append(
		cmd.destinationsHistory,
		prompt.Suggest{
			Text:        result["destination"],
			Description: "Download destination",
		},
	)
}

func (cmd *DownloadCommand) suggestArguments(parts []string) []prompt.Suggest {
	if len(parts) > 3 {
		// both url and destination should be already specified
		return []prompt.Suggest{}
	}

	if len(parts) == 3 {
		// destination should be suggested
		return prompt.FilterHasPrefix(cmd.destinationsHistory, parts[2], true)
	}

	if len(parts) == 2 {
		// url should be suggested
		return prompt.FilterHasPrefix(cmd.urlsHistory, parts[1], true)
	}

	// only command specified, suggest all known urls
	return cmd.urlsHistory
}
