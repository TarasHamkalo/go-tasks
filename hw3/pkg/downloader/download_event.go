package downloader

type DownloadEventType string

const (
	// DownloadEventStart indicates start of response body download process
	// (it is, headers are already received).
	// DownloadEventStart implies that data field contains expected resource
	// size or -1 in case unknown.
	DownloadEventStart DownloadEventType = "start"

	// DownloadEventCancel indicates that download has to be canceled due to user
	// interrupt.
	// DownloadEventCancel implies data field is empty.
	DownloadEventCancel DownloadEventType = "cancel"

	// DownloadEventUpdate indicates update of download status and
	// implies data field contains count of currently downloaded bytes.
	DownloadEventUpdate DownloadEventType = "update"

	// DownloadEventError indicates that error occurred during download process
	// and implies data field contains error cause.
	DownloadEventError DownloadEventType = "error"

	// DownloadEventComplete indicates completion of body download process.
	// DownloadEventComplete implies data field contains total size downloaded.
	DownloadEventComplete DownloadEventType = "complete"
)

// DownloadEvent represents an internal event emitted by DownloadTask and
// processes inside of event loop of Downloader.
// The meaning of Data depends on Type():
//   - Start:    int64 (expected size or -1 in case unknown)
//   - Update:   int64 (bytes downloaded)
//   - Complete: int64 (total size)
//   - Error:    error
//   - Cancel:   nil
//
// More description of events can be found on DownloadEventType doc.
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

func (d DownloadEvent) TaskId() string {
	return d.taskId
}

func (d DownloadEvent) DownloadId() string {
	return d.downloadId
}

func (d DownloadEvent) Type() DownloadEventType {
	return d.eventType
}

// Int64 returns event data as int64.
// Valid only for Start, Update and Complete events.
// Panics otherwise.
func (d DownloadEvent) Int64() int64 {
	return d.data.(int64)
}

// Error returns data cast to Error, panics in case called
// on wrong type of event
func (d DownloadEvent) Error() error {
	return d.data.(error)
}

func (d DownloadEvent) Data() any {
	return d.data
}
