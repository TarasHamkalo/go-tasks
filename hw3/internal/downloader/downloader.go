package downloader

type Downloader struct {
	userAgent string

	downloadTasks *TasksStore

	eventsChan chan DownloadEvent
}

func NewDownloader() *Downloader {
	return &Downloader{
		userAgent:     "BOT FIT/CTU (student project)",
		downloadTasks: NewTasksStore(),
		eventsChan:    make(chan DownloadEvent),
	}
}

func (d *Downloader) SubmitDownload(
	url string,
	destination string,
) {
	downloadTask := NewDownloadTask(url, destination)
	download := NewDownload(downloadTask.Id(), url, destination)

	d.downloadTasks.AddTask(downloadTask)

	go downloadTask.Execute(d)
}

func (d *Downloader) UserAgent() string {
	return d.userAgent
}

func (d *Downloader) EventsChan() chan<- DownloadEvent {
	return d.eventsChan
}
