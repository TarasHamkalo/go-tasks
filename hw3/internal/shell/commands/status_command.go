package commands

import (
	"downloader/internal/core"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/c-bata/go-prompt"
)

const DefaultDownloadsTmplPath = "templates/downloads.tmpl"

//go:embed templates/downloads.tmpl
var DefaultDownloadsTmplFs embed.FS

type StatusCommand struct {
	downloader *core.Downloader

	pattern *regexp.Regexp

	downloadsTmpl *template.Template

	*BaseCommand
}

func NewStatusCommand(
	downloader *core.Downloader,
) *StatusCommand {
	return NewStatusCommandWithTmpl(
		downloader,
		DefaultDownloadsTmplFs,
		DefaultDownloadsTmplPath,
	)
}

// NewStatusCommandWithTmpl downloadTmpl should contain templates called
// "download_table" for displaying all downloads at once and
// "download_detail" for displaying single download.
func NewStatusCommandWithTmpl(
	downloader *core.Downloader,
	tmplFS embed.FS,
	tmplPath string,
) *StatusCommand {
	downloadsTmpl := template.Must(
		template.
			New(filepath.Base(tmplPath)).
			Funcs(template.FuncMap{
				"formatSpeed":      FormatSpeed,
				"formatBytes":      FormatBytes,
				"formatPath":       FormatPath,
				"formatCompletion": FormatCompletion,
				"formatExpected":   FormatExpected,
			}).
			ParseFS(tmplFS, tmplPath),
	)
	cmd := &StatusCommand{
		pattern: regexp.MustCompile(
			"^status\\s?(?P<downloadId>[\\w-]{1,150})?$",
		),
		downloader: downloader,

		downloadsTmpl: downloadsTmpl,
	}

	cmd.BaseCommand = NewBaseCommand(
		"status",
		"status [downloadId], max 150 chars per field",
		// yep, pelican, just close your eyes this time :)
		&BaseCommandHandler{
			handle:           cmd.handle,
			matches:          cmd.matches,
			suggestArguments: cmd.suggestArguments,
		},
	)

	return cmd
}

func (cmd *StatusCommand) matches(s string) bool {
	return cmd.pattern.MatchString(s)
}

func (cmd *StatusCommand) handle(s string) {
	matches := cmd.pattern.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range cmd.pattern.SubexpNames() {
		if i != 0 && name != "" && len(matches[i]) > 0 {
			result[name] = matches[i]
		}
	}

	downloadId, ok := result["downloadId"]
	if !ok {
		err := cmd.downloadsTmpl.ExecuteTemplate(
			os.Stdout, "downloads_table", cmd.downloader.AllDownloads(),
		)

		if err != nil {
			fmt.Printf("Could not show downloads: %v\n", err)
		}

		return
	}

	download, err := cmd.downloader.Download(downloadId)
	if err != nil {
		fmt.Printf("Could not show download: %v\n", err)
	}

	err = cmd.downloadsTmpl.ExecuteTemplate(
		os.Stdout, "download_detail", download,
	)
	if err != nil {
		fmt.Printf("Could not show download: %v\n", err)
	}
}

func (cmd *StatusCommand) suggestArguments(parts []string) []prompt.Suggest {
	suggestions := cmd.getIdsSuggestions()
	if len(parts) > 2 {
		return []prompt.Suggest{{}}
	}

	if len(parts) == 2 {
		return prompt.FilterHasPrefix(suggestions, parts[1], true)
	}

	return append(suggestions, prompt.Suggest{
		Text:        "",
		Description: "Show downloads table (less detailed)",
	})
}

func (cmd *StatusCommand) getIdsSuggestions() []prompt.Suggest {
	suggestions := make([]prompt.Suggest, 0, 10)
	ids := cmd.downloader.AllDownloadIds()
	for _, id := range ids {
		suggestions = append(suggestions, prompt.Suggest{
			Text:        id,
			Description: "Download Id",
		})
	}

	return suggestions
}
