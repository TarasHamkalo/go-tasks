package downloader

type ExternalEventType string

const (
	// ExternalEventStart indicates body download started.
	ExternalEventStart ExternalEventType = "start"

	// ExternalEventCancel indicates cancellation of download succeeded.
	ExternalEventCancel ExternalEventType = "cancel"

	// ExternalEventError indicates download error occurred.
	ExternalEventError ExternalEventType = "error"

	// ExternalEventComplete indicates download succeeded.
	ExternalEventComplete ExternalEventType = "complete"
)

// ExternalDownloadEvent represents a user-facing notification about
// a download lifecycle event (start, completion, cancellation, error).
//
// These events are emitted by Downloader after the corresponding
// DownloadRecord state transition has been successfully applied,
// so the data is consistent and ready to be queried by the client.
//
// The meaning of Bytes() and Error() depends on Type():
//   - Start:    Bytes() = expected size (or -1 if unknown), Error() = nil
//   - Cancel:   Bytes() = 0, Error() = nil
//   - Complete: Bytes() = total downloaded size, Error() = nil
//   - Error:    Bytes() = 0, Error() = underlying cause (may be nil)
type ExternalDownloadEvent struct {
	// downloadId identifies subject/target of event
	downloadId string

	eventType ExternalEventType

	bytes int64

	err error
}

func NewExternalDownloadStart(
	downloadId string,
	expectedSize int64,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  ExternalEventStart,
		bytes:      expectedSize,
	}
}

func NewExternalDownloadCancel(
	downloadId string,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  ExternalEventCancel,
	}
}

func NewExternalDownloadComplete(
	downloadId string,
	totalSize int64,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  ExternalEventComplete,
		bytes:      totalSize,
	}
}

func NewExternalDownloadError(
	downloadId string,
	err error,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  ExternalEventError,
		err:        err,
	}
}

func (e ExternalDownloadEvent) DownloadId() string {
	return e.downloadId
}

func (e ExternalDownloadEvent) Type() ExternalEventType {
	return e.eventType
}

func (e ExternalDownloadEvent) Bytes() int64 {
	return e.bytes
}

func (e ExternalDownloadEvent) Err() error {
	return e.err
}
