package core

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type DownloadStatus string

const (
	StatusFailed     DownloadStatus = "failed"
	StatusRequested  DownloadStatus = "requested"
	StatusInProgress DownloadStatus = "in_progress"
	StatusCompleted  DownloadStatus = "completed"
)

type DownloadRecord struct {
	mu sync.RWMutex

	id string

	// taskId stores id of task executing this download, would be better to hold
	// as cross table, but currently left here
	taskId string

	url         string
	destination string

	status DownloadStatus

	bytesDownloaded int64

	// in case expected size is unknown, indicated by -1
	expectedSize int64

	// contains valid value when in complete state only
	totalSize int64

	// contains time when object was created
	requestedTime time.Time

	// contains time when started downloading
	startTime time.Time

	endTime time.Time

	cause error
}

func NewDownload(
	url string,
	destination string,
) *DownloadRecord {
	return &DownloadRecord{
		mu: sync.RWMutex{},

		id: uuid.New().String(),

		url:         url,
		destination: destination,

		status: StatusRequested,

		requestedTime: time.Now(),

		expectedSize: -1,
	}
}

func (d *DownloadRecord) WithTaskId(taskId string) *DownloadRecord {
	d.taskId = taskId
	return d
}

func (d *DownloadRecord) DetachedView() DownloadView {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DownloadView{
		Id:              d.id,
		Url:             d.url,
		Destination:     d.destination,
		Status:          d.status,
		BytesDownloaded: d.bytesDownloaded,
		ExpectedSize:    d.expectedSize,
		TotalSize:       d.totalSize,
		RequestedTime:   d.requestedTime,
		StartTime:       d.startTime,
		EndTime:         d.endTime,
		Cause:           d.cause,
	}
}

func (d *DownloadRecord) Start(expectedSize int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status != StatusRequested {
		return
	}

	d.status = StatusInProgress
	d.expectedSize = expectedSize
	d.startTime = time.Now()
}

func (d *DownloadRecord) SetBytesDownloaded(bytesDownloaded int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status != StatusInProgress {
		return
	}

	d.bytesDownloaded = bytesDownloaded
}

// Fail can entered failed state never been started
func (d *DownloadRecord) Fail(cause error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.status = StatusFailed
	d.cause = cause
	d.taskId = ""
	d.endTime = time.Now()
}

func (d *DownloadRecord) Complete(totalSize int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.status = StatusCompleted

	// for completeness update both
	d.bytesDownloaded = totalSize
	d.totalSize = totalSize

	d.taskId = ""
	d.endTime = time.Now()
}

func (d *DownloadRecord) Id() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.id
}

func (d *DownloadRecord) Status() DownloadStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status
}

func (d *DownloadRecord) TaskId() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.taskId
}
