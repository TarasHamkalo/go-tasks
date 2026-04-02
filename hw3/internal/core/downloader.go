package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type Downloader struct {
	userAgent string

	statusUpdateInterval time.Duration

	downloadTasks *TasksStore

	downloadsStore *DownloadStore

	// should be accessed only by given package
	eventsChan chan DownloadEvent

	// should be accessed by lib consumer
	subscribers []chan ExternalDownloadEvent
	subMu       sync.RWMutex

	shutdownChan chan struct{}

	shuttingDown atomic.Bool

	logger *zap.Logger
}

func NewDefaultDownloader(logger *zap.Logger) *Downloader {
	return NewDownloader(
		"BOT FIT/CTU (student project)",
		time.Millisecond*200,
		logger,
	)
}

func NewDownloader(
	userAgent string,
	statusUpdateInterval time.Duration,
	logger *zap.Logger,
) *Downloader {

	return &Downloader{
		userAgent:            userAgent,
		statusUpdateInterval: statusUpdateInterval,

		downloadTasks:  NewTasksStore(),
		downloadsStore: NewDownloadStore(),

		eventsChan: make(chan DownloadEvent, 10),

		subscribers: make([]chan ExternalDownloadEvent, 0),

		shutdownChan: make(chan struct{}),

		logger: logger,
	}
}

func (d *Downloader) Start() {
	go d.startEventHandlerLoop()
}

//	func (d *Downloader) Shutdown(ctx context.Context) {
//		d.logger.Info("Downloader shutting down")
//		d.shuttingDown.Store(true)
//		drained := d.downloadTasks.DrainOnly()
//
//		d.downloadTasks.CancelAll()
//
//		// using another chan for indicating start of graceful shutdown,
//		// is not necessary for given design
//		select {
//		case <-drained:
//			d.logger.Info("All tasks finished gracefully (canceled or completed)")
//		case <-ctx.Done():
//			d.logger.Warn("Graceful shutdown timed out, event loop exits")
//		}
//
//		close(d.shutdownChan)
//	}

func (d *Downloader) Shutdown(ctx context.Context) {
	d.logger.Info("Downloader shutting down")

	// stop receiving new downloads
	d.shuttingDown.Store(true)

	// before event loop can quit, all tasks should exit
	// (their events has to be processed)
	// so compute total given time and split it into two halves
	//	1. giving time for tasks to finish
	//  2. cancel all unfinished and give time for event loop to drain events

	total := time.Until(deadline(ctx))
	if total <= 0 {
		total = 0
	}

	graceTime := total / 2
	drainTime := total - graceTime

	d.logger.Debug("Downloader awaiting task completion for", zap.Duration("time", graceTime))

	graceCtx, cancelGrace := context.WithTimeout(context.Background(), graceTime)
	defer cancelGrace()

	// to be sure, set tasks store into mode, at which no new tasks accepted
	// drained chan is closed when no tasks left
	drained := d.downloadTasks.DrainOnly()

	// await tasks completion for total/2
	select {
	case <-drained:
		d.logger.Info("Tasks finished gracefully")

	case <-graceCtx.Done():
		d.logger.Warn("Graceful phase timed out, cancelling tasks")
		d.downloadTasks.CancelAll()
	}
	// in case tasks didn't finish, cancel all (grace ctx timed out)
	// give time for event loop to process their cancel events

	d.logger.Debug("Downloader awaiting events drain for", zap.Duration("time", drainTime))
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), drainTime)
	defer cancelDrain()

	select {
	case <-drained:
		d.logger.Info("All tasks removed after cancel")

	case <-drainCtx.Done():
		d.logger.Warn("Drain phase timed out")
	}

	// stop event loop
	close(d.shutdownChan)
}

func deadline(ctx context.Context) time.Time {
	dl, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(5 * time.Second)
	}
	return dl
}

func (d *Downloader) startEventHandlerLoop() {
	for {
		select {
		case event := <-d.eventsChan:
			fmt.Println("Event Received: ", event)
			d.logger.Debug("download event",
				zap.String("type", string(event.EventType())),
				zap.String("downloadId", event.DownloadId()),
				zap.String("taskId", event.TaskId()),
			)

			download, err := d.downloadsStore.Get(event.DownloadId())
			if err != nil {
				d.logger.Error("can not handle event",
					zap.String("type", string(event.EventType())),
					zap.String("downloadId", event.DownloadId()),
					zap.String("taskId", event.TaskId()),
					zap.Error(err),
				)

				d.broadcastPublic(
					NewExternalDownloadError(event.DownloadId(), err),
				)
				continue
			}

			d.handleDownloadEvent(event, download)
		case <-d.shutdownChan:
			d.logger.Sync()
			d.logger.Info("event loop shutdown")
			return

		}
	}
}

func (d *Downloader) handleDownloadEvent(
	event DownloadEvent,
	download *DownloadRecord,
) {
	switch event.EventType() {
	case DownloadEventStart:
		download.Start(event.Int64())
		d.broadcastPublic(
			NewExternalDownloadStart(event.DownloadId(), event.Int64()),
		)

	case DownloadEventUpdate:
		download.SetBytesDownloaded(event.Int64())

	case DownloadEventCancel:
		d.handleTaskRemoval(event, false)
		download.Cancel()

		d.broadcastPublic(
			NewExternalDownloadCancel(event.DownloadId()),
		)

	case DownloadEventError:
		d.handleTaskRemoval(event, true)

		download.Fail(event.Error())
		d.broadcastPublic(
			NewExternalDownloadError(event.DownloadId(), event.Error()),
		)
	case DownloadEventComplete:
		d.handleTaskRemoval(event, true)
		download.Complete(event.Int64())

		d.broadcastPublic(
			NewExternalDownloadComplete(event.DownloadId(), event.Int64()),
		)

	default:
		d.logger.Error("unknown event type", zap.Any("event", event))
	}
}

func (d *Downloader) handleTaskRemoval(event DownloadEvent, withCancel bool) {
	taskEntry, err := d.downloadTasks.Remove(event.TaskId())
	if err != nil {
		d.logger.Warn(
			"could not remove task",
			zap.String("type", string(event.EventType())),
			zap.String("downloadId", event.DownloadId()),
			zap.String("taskId", event.TaskId()),
			zap.Error(err),
		)
	}

	if withCancel {
		taskEntry.Cancel()
	}
}

// SubmitDownload returns downloadID
func (d *Downloader) SubmitDownload(
	ctx context.Context,
	url string,
	destination string,
) (string, error) {
	if d.shuttingDown.Load() {
		return "", fmt.Errorf("downloader is shutting down")
	}

	taskCtx, cancelFunc := context.WithCancel(ctx)

	download := NewDownload(url, destination)
	downloadTask := NewDownloadTask(download.Id(), url, destination)
	download.SetTaskId(downloadTask.Id())

	err := d.downloadTasks.Add(NewTaskEntry(downloadTask, cancelFunc))
	if err != nil {
		cancelFunc()
		return "", err
	}

	d.downloadsStore.Add(download)

	go downloadTask.Execute(taskCtx, d)

	return download.Id(), nil
}

func (d *Downloader) DownloadStatus(downloadId string) (*DownloadView, error) {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return nil, err
	}

	return download.DetachedView(), nil
}

func (d *Downloader) AllDownloadsStatus() []*DownloadView {
	return d.downloadsStore.GetAllViews()
}

func (d *Downloader) CancelDownload(downloadId string) error {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return err
	}

	taskEntry, err := d.downloadTasks.Get(download.TaskId())
	if err != nil {
		d.logger.Warn(
			"download is not cancellable anymore (no active task)",
			zap.String("downloadId", download.Id()),
		)

		return fmt.Errorf("download is not cancellable anymore")
	}

	taskEntry.Cancel()
	return nil
}

func (d *Downloader) CompletionChan(
	downloadId string,
) (<-chan struct{}, error) {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return nil, err
	}

	taskEntry, err := d.downloadTasks.Get(download.taskId)
	if err != nil {
		// task already finished and removed, just return closed chan
		ch := make(chan struct{})
		close(ch)
		return ch, nil
	}

	return taskEntry.Task.Done(), nil
}

func (d *Downloader) UserAgent() string {
	return d.userAgent
}

func (d *Downloader) Subscribe() <-chan ExternalDownloadEvent {
	d.subMu.Lock()
	defer d.subMu.Unlock()
	ch := make(chan ExternalDownloadEvent, 10)
	d.subscribers = append(d.subscribers, ch)

	return ch
}

func (d *Downloader) writeEventsChan() chan<- DownloadEvent {
	return d.eventsChan
}

func (d *Downloader) broadcastPublic(e ExternalDownloadEvent) {
	d.subMu.RLock()
	defer d.subMu.RUnlock()

	for _, sub := range d.subscribers {
		select {
		case sub <- e:
		default:
		}
	}
}
