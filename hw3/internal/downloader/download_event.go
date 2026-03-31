package downloader

type DownloadEventType string

const (
	// DownloadEventStart implies data field with expected size
	DownloadEventStart DownloadEventType = "start"

	// DownloadEventUpdate implies data field contains
	// count of currently downloaded bytes
	DownloadEventUpdate DownloadEventType = "update"

	// DownloadEventError implies data field with error object
	DownloadEventError DownloadEventType = "error"

	// DownloadEventComplete implies data field with totalSize provided
	DownloadEventComplete DownloadEventType = "complete"
)

type DownloadEvent struct {
	downloadId string
	eventType  DownloadEventType
	data       any
}

func NewDownloadStart(id string, expectedSize int64) DownloadEvent {
	return DownloadEvent{
		downloadId: id,
		eventType:  DownloadEventStart,
		data:       expectedSize,
	}
}

func NewDownloadUpdate(id string, bytesDownloaded int64) DownloadEvent {
	return DownloadEvent{
		downloadId: id,
		eventType:  DownloadEventUpdate,
		data:       bytesDownloaded,
	}
}

func NewDownloadError(id string, err error) DownloadEvent {
	return DownloadEvent{
		downloadId: id,
		eventType:  DownloadEventError,
		data:       err,
	}
}

func NewDownloadComplete(id string, totalSize int64) DownloadEvent {
	return DownloadEvent{
		downloadId: id,
		eventType:  DownloadEventComplete,
		data:       totalSize,
	}
}

func (b *DownloadEvent) DownloadId() string {
	return b.downloadId
}

func (b *DownloadEvent) EventType() DownloadEventType {
	return b.eventType
}

// Int64 returns data cast to Int64, panics in case called
// on wrong type of event
func (e *DownloadEvent) Int64() int64 {
	return e.data.(int64)
}

// Error returns data cast to Error, panics in case called
// on wrong type of event
func (e *DownloadEvent) Error() error {
	return e.data.(error)
}
func (b *DownloadEvent) Data() any {
	return b.data
}
