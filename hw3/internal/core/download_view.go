package core

import "time"

type DownloadView struct {
	Id string

	Url         string
	Destination string

	Status DownloadStatus

	BytesDownloaded int64
	ExpectedSize    int64
	TotalSize       int64

	RequestedTime time.Time
	StartTime     time.Time
	EndTime       time.Time

	Cause error
}
