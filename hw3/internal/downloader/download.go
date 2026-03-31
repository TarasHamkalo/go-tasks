package downloader

import "time"

type DownloadStatus string

const (
	StatusFailed     DownloadStatus = "failed"
	StatusRequested  DownloadStatus = "requested"
	StatusInProgress DownloadStatus = "in_progress"
	StatusCompleted  DownloadStatus = "completed"
)

type Download struct {
	id     string
	taskId string

	url         string
	destination string

	status DownloadStatus

	bytesDownloaded int64

	size int64

	startTime time.Time

	endTime time.Time

	cause error
}

func NewDownload(
	id string,
	taskId string,
	url string,
	destination string,
) *Download {
	return &Download{
		id:          id,
		taskId:      taskId,
		url:         url,
		destination: destination,
		status:      StatusRequested,
	}
}

func (d *Download) Start(expectedSize int64) {
	if d.status != StatusRequested {
		return
	}

	d.status = StatusInProgress
	d.size = expectedSize
	d.startTime = time.Now()
}

func (d *Download) SetBytesDownloaded(bytesDownloaded int64) {
	if d.status != StatusInProgress {
		return
	}
	d.bytesDownloaded = bytesDownloaded
}

func (d *Download) Fail(cause error) {
	d.status = StatusFailed
	d.cause = cause
	d.taskId = ""
	d.endTime = time.Now()
}

func (d *Download) Complete(totalSize int64) {
	d.status = StatusCompleted
	d.size = totalSize
	d.taskId = ""
	d.endTime = time.Now()
}

func (d *Download) Id() string {
	return d.id
}

func (d *Download) TaskId() string {
	return d.taskId
}

func (d *Download) Url() string {
	return d.url
}

func (d *Download) Destination() string {
	return d.destination
}

func (d *Download) Status() DownloadStatus {
	return d.status
}

func (d *Download) BytesDownloaded() int64 {
	return d.bytesDownloaded
}

func (d *Download) Size() int64 {
	return d.size
}

func (d *Download) StartTime() time.Time {
	return d.startTime
}

func (d *Download) EndTime() time.Time {
	return d.endTime
}

func (d *Download) Cause() error {
	return d.cause
}
