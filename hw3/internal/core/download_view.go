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

func (d DownloadView) String() string {
	return fmt.Sprintf(
		"Download:\n"+
			"  Id:              %s\n"+
			"  Url:             %s\n"+
			"  Destination:     %s\n"+
			"  Status:          %v\n"+
			"  BytesDownloaded: %d\n"+
			"  ExpectedSize:    %d\n"+
			"  TotalSize:       %d\n"+
			"  RequestedTime:   %s\n"+
			"  StartTime:       %s\n"+
			"  EndTime:         %s\n"+
			"  Cause:           %v",
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
