package core

import (
	"fmt"
	"time"
)

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

// generated for demo
func (d DownloadView) String() string {
	return fmt.Sprintf(
		"Id=%s\nUrl=%s\nDestination=%s\nStatus=%v\nBytesDownloaded=%d\nExpectedSize=%d\nTotalSize=%d\nRequestedTime=%s\nStartTime=%s\nEndTime=%s\nCause=%v",
		d.Id,
		d.Url,
		d.Destination,
		d.Status,
		d.BytesDownloaded,
		d.ExpectedSize,
		d.TotalSize,
		d.RequestedTime,
		d.StartTime,
		d.EndTime,
		d.Cause,
	)
}
