package commands

import (
	"downloader/pkg/downloader"
	"fmt"
	"regexp"
	"time"

	"github.com/c-bata/go-prompt"
)

type CancelCommand struct {
	downloader *downloader.Downloader

	pattern *regexp.Regexp

	cachedSuggestions []prompt.Suggest
	cachedTime        time.Time
	cacheLifeDuration time.Duration
	*BaseCommand
}

func NewCancelCommand(downloader *downloader.Downloader) *CancelCommand {
	return NewCancelCommandWithCacheLifetime(
		downloader,
		5*time.Second,
	)
}

func NewCancelCommandWithCacheLifetime(
	downloader *downloader.Downloader,
	cacheLifeDuration time.Duration,
) *CancelCommand {
	cmd := &CancelCommand{
		pattern: regexp.MustCompile(
			"^cancel\\s(?P<downloadId>[\\w-]{1,150})$",
		),
		downloader:        downloader,
		cacheLifeDuration: cacheLifeDuration,
	}

	cmd.BaseCommand = NewBaseCommand(
		"cancel",
		"cancel <downloadId>, max 150 chars per field",
		// yep, pelican, just close your eyes this time :)
		&BaseCommandHandler{
			handle:           cmd.handle,
			matches:          cmd.matches,
			suggestArguments: cmd.suggestArguments,
		},
	)

	return cmd
}

func (cmd *CancelCommand) matches(s string) bool {
	return cmd.pattern.MatchString(s)
}

func (cmd *CancelCommand) handle(s string) {
	matches := cmd.pattern.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range cmd.pattern.SubexpNames() {
		if i != 0 && name != "" && len(matches[i]) > 0 {
			result[name] = matches[i]
		}
	}

	downloadId := result["downloadId"]

	err := cmd.downloader.CancelDownload(downloadId)
	if err != nil {
		fmt.Printf("Failed to cancel download %s: %v\n", downloadId, err)
		return
	}

	fmt.Printf("Download %s cancelled\n", downloadId)
}

func (cmd *CancelCommand) suggestArguments(parts []string) []prompt.Suggest {
	suggestions := cmd.getIdsSuggestionsCached()
	if len(parts) > 2 {
		return []prompt.Suggest{{}}
	}

	if len(parts) == 2 {
		return prompt.FilterHasPrefix(suggestions, parts[1], true)
	}

	return suggestions
}

func (cmd *CancelCommand) getIdsSuggestionsCached() []prompt.Suggest {
	if len(cmd.cachedSuggestions) == 0 {
		cmd.cachedSuggestions = cmd.getIdsSuggestions()
		cmd.cachedTime = time.Now()
		return cmd.cachedSuggestions
	}

	if time.Since(cmd.cachedTime) < cmd.cacheLifeDuration {
		return cmd.cachedSuggestions
	}

	cmd.cachedSuggestions = cmd.getIdsSuggestions()
	cmd.cachedTime = time.Now()
	return cmd.cachedSuggestions
}

func (cmd *CancelCommand) getIdsSuggestions() []prompt.Suggest {
	suggestions := make([]prompt.Suggest, 0, 10)
	downloads := cmd.downloader.AllDownloads()
	for _, download := range downloads {
		// pretty wasteful suggestion as copies of structs created
		// just to get id
		if download.Status == downloader.StatusRequested ||
			download.Status == downloader.StatusInProgress {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        download.Id,
				Description: "Download Id",
			})
		}
	}

	return suggestions
}
