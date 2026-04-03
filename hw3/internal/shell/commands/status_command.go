package commands

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
				"formatSpeed":    formatSpeed,
				"formatBytes":    formatBytes,
				"formatPath":     formatPath,
				"formatExpected": formatExpected,
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
