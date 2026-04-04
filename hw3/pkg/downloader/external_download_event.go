package downloader

type EDownloadEventType string

const (
	EDownloadEventStart EDownloadEventType = "start"

	EDownloadEventCancel EDownloadEventType = "cancel"

	EDownloadEventError EDownloadEventType = "error"

	EDownloadEventComplete EDownloadEventType = "complete"
)

type ExternalDownloadEvent struct {
	// downloadId identifies subject/target of event
	downloadId string

	eventType EDownloadEventType

	bytes int64

	err error
}

func NewExternalDownloadStart(
	downloadId string,
	expectedSize int64,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  EDownloadEventStart,
		bytes:      expectedSize,
	}
}

func NewExternalDownloadCancel(
	downloadId string,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  EDownloadEventCancel,
	}
}

func NewExternalDownloadComplete(
	downloadId string,
	totalSize int64,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  EDownloadEventComplete,
		bytes:      totalSize,
	}
}

func NewExternalDownloadError(
	downloadId string,
	err error,
) ExternalDownloadEvent {
	return ExternalDownloadEvent{
		downloadId: downloadId,
		eventType:  EDownloadEventError,
		err:        err,
	}
}

func (e ExternalDownloadEvent) DownloadId() string {
	return e.downloadId
}

func (e ExternalDownloadEvent) Type() EDownloadEventType {
	return e.eventType
}

func (e ExternalDownloadEvent) Bytes() int64 {
	return e.bytes
}

func (e ExternalDownloadEvent) Error() error {
	return e.err
}
