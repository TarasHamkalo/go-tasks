package core

import (
	"context"
	"fmt"
	"sync"
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
	logger      *zap.Logger
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

		logger: logger,
	}
}

func (d *Downloader) Start(ctx context.Context) {
	go d.startEventHandlerLoop(ctx)
}

func (d *Downloader) startEventHandlerLoop(ctx context.Context) {
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

		case <-ctx.Done():
			// TODO: await tasks completion for few seconds
			d.logger.Sync()
			d.logger.Info("event loop stopped")
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
) string {
	taskCtx, cancelFunc := context.WithCancel(ctx)

	download := NewDownload(url, destination)
	downloadTask := NewDownloadTask(download.Id(), url, destination)

	download.SetTaskId(downloadTask.Id())

	d.downloadsStore.Add(download)
	d.downloadTasks.Add(NewTaskEntry(downloadTask, cancelFunc))

	go downloadTask.Execute(taskCtx, d)

	return download.Id()
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
