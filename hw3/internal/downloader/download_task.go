package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type DownloadTask struct {
	id string

	downloadId string // cross reference

	url string

	destination string

	progress *ProgressWriter

	//cancel func later
}

func NewDownloadTask(downloadId, url, destination string) *DownloadTask {
	return &DownloadTask{
		id:          uuid.New().String(),
		downloadId:  downloadId,
		url:         url,
		destination: destination,
		progress:    NewProgressWriter(),
	}
}

func (d *DownloadTask) Id() string {
	return d.id
}

func (d *DownloadTask) Execute(downloader *Downloader) {
	// TODO: client := &http.Client{ Timeout: time.Second * 5, }; ?
	// TODO: NewRequestWithContext
	// TODO: |os.O_EXCL
	file, err := os.OpenFile(
		d.destination,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)

	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d.downloadId,
			fmt.Errorf("error opening file: %v", err),
		)
		return
	}

	defer func() {
		err := file.Close()
		if err != nil {
			downloader.EventsChan() <- NewDownloadError(
				d.downloadId,
				fmt.Errorf("error closing file: %v", err),
			)
		}
	}()

	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d.downloadId,
			fmt.Errorf("error building request: %v", err),
		)
		return
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", downloader.UserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d.downloadId,
			fmt.Errorf("http client error: %v", err),
		)
		return
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			downloader.EventsChan() <- NewDownloadError(
				d.downloadId,
				fmt.Errorf("error closing http body: %s\n", err),
			)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		downloader.EventsChan() <- NewDownloadError(
			d.downloadId,
			fmt.Errorf("bad status: %s", resp.Status),
		)
		return
	}

	downloader.EventsChan() <- NewDownloadStart(d.id, resp.ContentLength)
	d.progress.Reset()

	tickerDone := make(chan struct{})
	defer close(tickerDone)

	go d.tickProgress(downloader, 200, tickerDone)
	teeReader := io.TeeReader(resp.Body, d.progress)
	if _, err := io.Copy(file, teeReader); err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d.downloadId,
			fmt.Errorf("error downloading content: %s", err),
		)
		return
	}

	// TODO: should have completion event
	downloader.EventsChan() <- NewDownloadComplete(d.id, d.progress.BytesRead())
}

func (d *DownloadTask) tickProgress(
	downloader *Downloader,
	ms int64,
	done <-chan struct{},
) {
	ticker := time.NewTicker(time.Duration(ms) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			downloader.EventsChan() <- NewDownloadUpdate(
				d.downloadId,
				d.progress.BytesRead(),
			)
		case <-done:
			return
		}
	}
}
