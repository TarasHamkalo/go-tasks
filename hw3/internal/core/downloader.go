package core

import (
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

		subscribers: make([]chan ExternalDownloadEvent, 2),

		logger: logger,
	}
}

func (d *Downloader) Start() {
	go d.startEventHandlerLoop()
}

func (d *Downloader) startEventHandlerLoop() {
	for event := range d.eventsChan {
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

		switch event.EventType() {
		case DownloadEventStart:
			download.Start(event.Int64())
			d.broadcastPublic(
				NewExternalDownloadStart(event.DownloadId(), event.Int64()),
			)
		case DownloadEventUpdate:
			download.SetBytesDownloaded(event.Int64())
		case DownloadEventError:
			d.handleTaskRemoval(event)
			download.Fail(event.Error())

			d.broadcastPublic(
				NewExternalDownloadError(event.DownloadId(), event.Error()),
			)
		case DownloadEventComplete:
			d.handleTaskRemoval(event)
			download.Complete(event.Int64())

			d.broadcastPublic(
				NewExternalDownloadComplete(event.DownloadId(), event.Int64()),
			)

		default:
			d.logger.Error("unknown event type", zap.Any("event", event))
		}
	}
}

func (d *Downloader) handleTaskRemoval(event DownloadEvent) {
	_, err := d.downloadTasks.Remove(event.TaskId())
	if err != nil {
		d.logger.Warn(
			"task not found for event",
			zap.String("type", string(event.EventType())),
			zap.String("downloadId", event.DownloadId()),
			zap.String("taskId", event.TaskId()),
		)
	}
}

// SubmitDownload returns downloadID
func (d *Downloader) SubmitDownload(
	url string,
	destination string,
) string {
	download := NewDownload(url, destination)
	downloadTask := NewDownloadTask(download.Id(), url, destination)

	download.WithTaskId(downloadTask.Id())

	d.downloadTasks.Add(downloadTask)
	d.downloadsStore.Add(download)

	go downloadTask.Execute(d)

	return download.Id()
}

func (d *Downloader) CompletionChan(
	downloadId string,
) (<-chan struct{}, error) {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return nil, err
	}

	task, err := d.downloadTasks.Get(download.taskId)
	if err != nil {
		return nil, err
	}

	return task.Done(), nil
}

func (d *Downloader) UserAgent() string {
	return d.userAgent
}

func (d *Downloader) writeEventsChan() chan<- DownloadEvent {
	return d.eventsChan
}

func (d *Downloader) Subscribe() <-chan ExternalDownloadEvent {
	ch := make(chan ExternalDownloadEvent, 10)
	d.subscribers = append(d.subscribers, ch)

	return ch
}

func (d *Downloader) broadcastPublic(e ExternalDownloadEvent) {
	for _, sub := range d.subscribers {
		select {
		case sub <- e:
		default:
		}
	}
}
