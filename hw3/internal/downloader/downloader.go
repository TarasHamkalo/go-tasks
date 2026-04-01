package downloader

import "fmt"

type Downloader struct {
	userAgent string

	downloadTasks *TasksStore

	downloadsStore *DownloadStore

	// should be accessed only by given package
	eventsChan chan DownloadEvent

	// should be accessed by lib consumer
	errorsChan chan error
}

func NewDownloader() *Downloader {
	return &Downloader{
		userAgent: "BOT FIT/CTU (student project)",

		downloadTasks:  NewTasksStore(),
		downloadsStore: NewDownloadStore(),

		eventsChan: make(chan DownloadEvent, 10),

		errorsChan: make(chan error, 10),
	}
}

func (d *Downloader) Start() {
	go d.startEventHandlerLoop()
}

func (d *Downloader) startEventHandlerLoop() {
	for event := range d.eventsChan {
		// TODO: add logging
		fmt.Printf("[%s] [%s] [%s] [%v]\n", event.TaskId(), event.DownloadId(), event.EventType(), event.Data())
		download, err := d.downloadsStore.Get(event.DownloadId())
		if err != nil {
			select {
			case d.errorsChan <- err:
			default:
			}

			continue
		}

		switch event.EventType() {
		case DownloadEventStart:
			download.Start(event.Int64())
		case DownloadEventUpdate:
			download.SetBytesDownloaded(event.Int64())
		case DownloadEventError:
			_, err := d.downloadTasks.Remove(event.TaskId())
			if err != nil {
				// TODO: logging, err is not much of a problem here
				fmt.Printf("[%s] [%s] Task not found\n", event.TaskId(), event.DownloadId())
			}

			download.Fail(event.Error())
		case DownloadEventComplete:
			_, err := d.downloadTasks.Remove(event.TaskId())
			if err != nil {
				// TODO: logging, err is not much of a problem here
				fmt.Printf("[%s] [%s] Task not found\n", event.TaskId(), event.DownloadId())
			}
			download.Complete(event.Int64())
		default:
			d.errorsChan <- fmt.Errorf(
				"unknown event type [%s] [%s] [%v]",
				event.DownloadId(), event.EventType(), event.Data(),
			)
		}
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

func (d *Downloader) EventsChan() chan<- DownloadEvent {
	return d.eventsChan
}

func (d *Downloader) ErrorsChan() <-chan error {
	return d.errorsChan
}
