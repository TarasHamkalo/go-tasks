package commands

import (
	"downloader/pkg/downloader"
	"fmt"
	"regexp"
	"time"

	"github.com/c-bata/go-prompt"
)

// CancelCommand provides implementation of "cancel <downloadId>" command.
type CancelCommand struct {
	downloader *downloader.Downloader

	// pattern used to match input to cancel command, see constructors
	pattern *regexp.Regexp

	// cachedSuggestions used to limit amount of queries to Downloader
	// about existing DownloadRecords
	cachedSuggestions []prompt.Suggest

	// cachedTime when cache was created
	cachedTime time.Time

	// cacheLifeDuration how long to cache
	cacheLifeDuration time.Duration

	*BaseCommand
}

// NewCancelCommand constructs cancel command with 2 seconds cache lifetime
func NewCancelCommand(downloader *downloader.Downloader) *CancelCommand {
	return NewCancelCommandWithCacheLifetime(
		downloader,
		2*time.Second,
	)
}

// NewCancelCommandWithCacheLifetime constructs cancel command
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
	// parse named regex groups
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
		// more than 2 strings in input, ignore
		return []prompt.Suggest{{}}
	}

	if len(parts) == 2 {
		// seconds string is being specified, could be
		// ids filtering
		return prompt.FilterHasPrefix(suggestions, parts[1], true)
	}

	// only command name matched
	return suggestions
}

// getIdsSuggestionsCached if cache is valid return it, otherwise rebuild
// suggestions and update cache
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

// getIdsSuggestions quires for all downloads and filters to "cancellable"
func (cmd *CancelCommand) getIdsSuggestions() []prompt.Suggest {
	suggestions := make([]prompt.Suggest, 0, 10)
	downloads := cmd.downloader.GetAllDownloads()
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
