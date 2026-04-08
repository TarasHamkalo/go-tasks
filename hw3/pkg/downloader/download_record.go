package downloader

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type DownloadStatus string

const (
	// StatusRequested initial status.
	StatusRequested DownloadStatus = "requested"
	// StatusInProgress when download body process started.
	StatusInProgress DownloadStatus = "in_progress"

	// StatusFailed, StatusCanceled, StatusCompleted represents final states
	// of download process
	StatusFailed    DownloadStatus = "failed"
	StatusCanceled  DownloadStatus = "canceled"
	StatusCompleted DownloadStatus = "completed"
)

// DownloadRecord stores all metadata associated with HTTP download.
// It is safe to access given record with multiple threads.
// State transitions and short metadata queries are done under locks (RW)
// and when most of metadata needed a deep copy of struct created (under lock).
type DownloadRecord struct {
	// id download id (default UUID)
	id string

	// taskId stores id of task executing this download
	// [future] would be better to move this association to Downloader object.
	taskId string

	// url is http resource being downloaded
	url string

	// destination is path on local file system, where resource gonna be stored
	// in case of success download
	destination string

	status DownloadStatus

	// bytesDownloaded current amount of byted downloaded (often updated)
	bytesDownloaded int64

	// expectedSize is expected resource size (set on start of download).
	// In case expected size is unknown, indicated by -1.
	expectedSize int64

	// totalSize is amount of bytes downloaded when download is finished,
	// successfully, so contains valid value when in complete state only.
	totalSize int64

	// requestedTime contains time when object was created.
	requestedTime time.Time

	// startTime contains time when started downloading resource body.
	startTime time.Time

	// endTime contains time when transitioned to one of final states.
	// (see DownloadStatus)
	endTime time.Time

	// cause is set when download has failed
	cause error

	// done used to track download completion (e.g. await it)
	// is channel closed when download is complete (enter any final state)
	// so multiple subscribers are fine.
	done chan struct{}

	mu sync.RWMutex
}

func NewDownloadRecord(
	url string,
	destination string,
) *DownloadRecord {
	return &DownloadRecord{
		id: uuid.New().String(),

		url:         url,
		destination: destination,

		status: StatusRequested,

		requestedTime: time.Now(),

		expectedSize: -1,

		done: make(chan struct{}),

		mu: sync.RWMutex{},

		// other variables default value is fine
	}
}

func (d *DownloadRecord) SetTaskId(taskId string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.taskId = taskId
}

// DetachedView create a deep copy of given download under read lock.
// DetachedView hides task id from users.
func (d *DownloadRecord) DetachedView() *DownloadView {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return &DownloadView{
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

// Start does transition to "in progress" state, when resource body is being downloaded
// and client might know expected size of download (otherwise set to -1).
// NOTE: record is write locked.
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

// SetBytesDownloaded update amount of downloaded bytes.
// NOTE: record is write locked.
func (d *DownloadRecord) SetBytesDownloaded(bytesDownloaded int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status != StatusInProgress {
		return
	}

	d.bytesDownloaded = bytesDownloaded
}

// Cancel does transition to canceled state,
// removing association with DownloadTask.
// NOTE: record is write locked.
func (d *DownloadRecord) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isInFinalState() {
		return
	}

	d.status = StatusCanceled
	d.taskId = ""
	d.endTime = time.Now()

	close(d.done)
}

// Fail does transition to failed state, recording cause and
// removing association with DownloadTask.
// NOTE: record is write locked.
func (d *DownloadRecord) Fail(cause error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isInFinalState() {
		return
	}

	d.status = StatusFailed
	d.cause = cause
	d.taskId = ""
	d.endTime = time.Now()

	close(d.done)
}

// Complete does transition to complete state, recording final download size and
// removing association with DownloadTask.
// NOTE: record is write locked.
func (d *DownloadRecord) Complete(totalSize int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isInFinalState() {
		return
	}

	d.status = StatusCompleted

	// for completeness update both
	d.bytesDownloaded = totalSize
	d.totalSize = totalSize

	d.taskId = ""
	d.endTime = time.Now()

	close(d.done)
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

func (d *DownloadRecord) Done() <-chan struct{} {
	// this one can be without lock as it never changes during lifecycle
	return d.done
}

// must be called with d.mu held
func (d *DownloadRecord) isInFinalState() bool {
	return d.status == StatusCompleted ||
		d.status == StatusCanceled ||
		d.status == StatusFailed
}
