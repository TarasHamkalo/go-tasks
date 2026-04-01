package core

type DownloadEventType string

const (
	// DownloadEventStart implies data field with expected size
	DownloadEventStart DownloadEventType = "start"

	// DownloadEventCancel implies data field is empty (user interrupt)
	DownloadEventCancel DownloadEventType = "cancel"

	// DownloadEventUpdate implies data field contains
	// count of currently downloaded bytes
	DownloadEventUpdate DownloadEventType = "update"

	// DownloadEventError implies data field with error object
	DownloadEventError DownloadEventType = "error"

	// DownloadEventComplete implies data field with totalSize provided
	DownloadEventComplete DownloadEventType = "complete"
)

type DownloadEvent struct {
	// taskId identifies source of event
	taskId string

	// downloadId identifies subject/target of event
	downloadId string

	eventType DownloadEventType

	// data stores data related to given event type, see constructors
	data any
}

func NewDownloadStart(d *DownloadTask, expectedSize int64) DownloadEvent {
	return DownloadEvent{
		taskId:     d.id,
		downloadId: d.downloadId,
		eventType:  DownloadEventStart,
		data:       expectedSize,
	}
}

func NewDownloadCancel(d *DownloadTask) DownloadEvent {
	return DownloadEvent{
		taskId:     d.id,
		downloadId: d.downloadId,
		eventType:  DownloadEventCancel,
	}
}

func NewDownloadUpdate(d *DownloadTask, bytesDownloaded int64) DownloadEvent {
	return DownloadEvent{
		taskId:     d.id,
		downloadId: d.downloadId,
		eventType:  DownloadEventUpdate,
		data:       bytesDownloaded,
	}
}

func NewDownloadError(d *DownloadTask, err error) DownloadEvent {
	return DownloadEvent{
		taskId:     d.id,
		downloadId: d.downloadId,
		eventType:  DownloadEventError,
		data:       err,
	}
}

func NewDownloadComplete(d *DownloadTask, totalSize int64) DownloadEvent {
	return DownloadEvent{
		taskId:     d.id,
		downloadId: d.downloadId,
		eventType:  DownloadEventComplete,
		data:       totalSize,
	}
}

func (d *DownloadEvent) TaskId() string {
	return d.taskId
}

func (d *DownloadEvent) DownloadId() string {
	return d.downloadId
}

func (d *DownloadEvent) EventType() DownloadEventType {
	return d.eventType
}

// Int64 returns data cast to Int64, panics in case called
// on wrong type of event
func (d *DownloadEvent) Int64() int64 {
	return d.data.(int64)
}

// Error returns data cast to Error, panics in case called
// on wrong type of event
func (d *DownloadEvent) Error() error {
	return d.data.(error)
}

func (d *DownloadEvent) Data() any {
	return d.data
}
