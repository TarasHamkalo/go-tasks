package shell

import (
	"context"
	"downloader/internal/core"
	"fmt"
	"regexp"

	"github.com/c-bata/go-prompt"
)

type DownloadCommand struct {
	downloader *core.Downloader

	pattern *regexp.Regexp

	destinationsHistory []prompt.Suggest

	urlsHistory []prompt.Suggest

	BaseCommand
}

func NewDownloadCommand(downloader *core.Downloader) *DownloadCommand {
	return &DownloadCommand{
		downloader: downloader,
		pattern: regexp.MustCompile(
			"^download\\s(?P<url>\\S{1,150})\\s(?P<destination>.{1,150})?$",
		),
		BaseCommand: *NewBaseCommand(
			"download",
			"download <url> <destination>, max 150 chars per field",
		),
	}
}

func (cmd *DownloadCommand) Matches(s string) bool {
	return cmd.pattern.MatchString(s)
}

func (cmd *DownloadCommand) doHandle(s string) {
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
		return prompt.FilterHasPrefix(cmd.urlsHistory, parts[1], true)
	}

	return cmd.urlsHistory
}
