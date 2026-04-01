package downloader

import "fmt"

type Downloader struct {
	userAgent string

	downloadTasks *TasksStore

	downloadsStore *DownloadStore

	eventsChan chan DownloadEvent
}

func NewDownloader() *Downloader {
	return &Downloader{
		userAgent: "BOT FIT/CTU (student project)",

		downloadTasks:  NewTasksStore(),
		downloadsStore: NewDownloadStore(),

		eventsChan: make(chan DownloadEvent, 10),
	}
}

func (d *Downloader) Start() {
	go (func() {
		for event := range d.eventsChan {
			fmt.Printf("[%s] [%s] [%v]\n", event.DownloadId(), event.EventType(), event.Data())
			// TODO: implement even handling
		}
	})()

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
