package shell

import (
	"downloader/internal/core"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/c-bata/go-prompt"
)

//go:embed templates/downloads.tmpl
var DefaultDownloadsTmpl embed.FS

type StatusCommand struct {
	downloader *core.Downloader

	pattern *regexp.Regexp

	downloadsTableTmpl *template.Template

	*BaseCommand
}

func NewStatusCommand(
	downloader *core.Downloader,
) *StatusCommand {
	downloadsTable := template.Must(
		template.
			New("downloads.tmpl").
			Funcs(template.FuncMap{
				"formatSpeed":    formatSpeed,
				"formatBytes":    formatBytes,
				"formatPath":     formatPath,
				"formatExpected": formatExpected,
			}).
			ParseFS(DefaultDownloadsTmpl, "*/*.tmpl"),
	)

	cmd := &StatusCommand{
		pattern: regexp.MustCompile(
			"^status\\s?(?P<downloadId>[\\w-]{1,150})?$",
		),
		downloader: downloader,

		downloadsTableTmpl: downloadsTable,
	}

	cmd.BaseCommand = NewBaseCommand(
		"status",
		"status [downloadId], max 150 chars per field",
		// yep, pelican, just close your eyes this time :)
		&BaseCommandHandler{
			handle: func(s string) {
				cmd.handle(s)
			},
			matches: func(s string) bool {
				return cmd.matches(s)
			},
			suggestArguments: func(parts []string) []prompt.Suggest {
				return cmd.suggestArguments(parts)
			},
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
		err := cmd.downloadsTableTmpl.ExecuteTemplate(
			os.Stdout, "downloads_table", cmd.downloader.AllDownloads(),
		)

		if err != nil {
			fmt.Printf("Could not show downloads: %v\n", err)
		}
	}

	// TODO: add template to single
	fmt.Println(downloadId)
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

func formatSpeed(d core.DownloadView) string {
	if d.StartTime.IsZero() || d.BytesDownloaded == 0 {
		return "0 B/s"
	}

	duration := time.Since(d.StartTime).Seconds()
	if duration <= 0 {
		return "0 B/s"
	}

	speed := float64(d.BytesDownloaded) / duration
	return formatBytes(int64(speed)) + "/s"
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func formatPath(p string, lastN int, maxLen int) string {
	if p == "" {
		return ""
	}

	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	if len(parts) > lastN {
		parts = parts[len(parts)-lastN:]
	}

	result := strings.Join(parts, "/")
	if len(result) > maxLen {
		return "..." + result[len(result)-maxLen+3:]
	}

	return result
}

func formatExpected(b int64) string {
	if b == -1 {
		return "unknown"
	}
	return formatBytes(b)
}
