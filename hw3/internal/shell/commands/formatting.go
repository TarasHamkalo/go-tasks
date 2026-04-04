package commands

import (
	"downloader/pkg/downloader"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func FormatSpeed(d downloader.DownloadView) string {
	if d.Status != downloader.StatusInProgress {
		return "0 B/s"
	}

	if d.StartTime.IsZero() || d.BytesDownloaded == 0 {
		return "0 B/s"
	}

	duration := time.Since(d.StartTime).Seconds()
	if duration <= 0 {
		return "0 B/s"
	}

	speed := float64(d.BytesDownloaded) / duration
	return FormatBytes(int64(speed)) + "/s"
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf(
		"%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp],
	)
}

func FormatPath(p string, lastN int, maxLen int) string {
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

func FormatExpected(b int64) string {
	if b == -1 {
		return "unknown"
	}
	return FormatBytes(b)
}

func FormatCompletion(d downloader.DownloadView) string {
	if d.Status == downloader.StatusCompleted {
		return "100.00 %"
	}

	if d.ExpectedSize == -1 {
		return "unknown"
	}

	return fmt.Sprintf(
		"%.2f%%", float64(d.BytesDownloaded)/float64(d.ExpectedSize),
	)
}
