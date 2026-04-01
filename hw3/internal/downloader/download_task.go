package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// DownloadTask represents a temporary object of running download.
// Based on that, it does not contain status but rather holds data
// need to cancel download or track its completion.
// For full metadata stored about donwload see DownloadRecord.
type DownloadTask struct {
	id string

	// cross reference
	downloadId string

	url string

	destination string

	// will be closed after task finishes
	done chan struct{}

	//cancel func later

	progress *ProgressWriter
}

func NewDownloadTask(downloadId, url, destination string) *DownloadTask {
	return &DownloadTask{
		id:          uuid.New().String(),
		downloadId:  downloadId,
		url:         url,
		destination: destination,
		progress:    NewProgressWriter(),
		done:        make(chan struct{}),
	}
}

func (d *DownloadTask) Id() string {
	return d.id
}

func (d *DownloadTask) Done() <-chan struct{} {
	return d.done
}

// TODO: refactor exec
func (d *DownloadTask) Execute(downloader *Downloader) {
	defer close(d.done)

	file, err := d.openFile(downloader)
	if err != nil {
		return
	}

	defer d.closeFile(file, downloader)

	resp, err := d.doRequest(downloader)
	if err != nil {
		return
	}
	defer d.closeResponse(resp, downloader)

	if err := d.validateResponse(resp, downloader); err != nil {
		return
	}

	d.startDownload(downloader, resp)

	if err := d.downloadBody(file, resp, downloader); err != nil {
		return
	}

	d.finishDownload(downloader)
}

func (d *DownloadTask) openFile(downloader *Downloader) (*os.File, error) {
	// TODO: |os.O_EXCL
	file, err := os.OpenFile(
		d.destination,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d, fmt.Errorf("error opening file: %v", err),
		)
		return nil, err
	}
	return file, nil
}

func (d *DownloadTask) closeFile(file *os.File, downloader *Downloader) {
	if err := file.Close(); err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d, fmt.Errorf("error closing file: %v", err),
		)
	}
}

// TODO: client := &http.Client{ Timeout: time.Second * 5, }; ?
// TODO: NewRequestWithContext
func (d *DownloadTask) doRequest(
	downloader *Downloader,
) (*http.Response, error) {
	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d, fmt.Errorf("error building request: %v", err),
		)
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", downloader.UserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d, fmt.Errorf("http client error: %v", err),
		)
		return nil, err
	}

	return resp, nil
}

func (d *DownloadTask) closeResponse(
	resp *http.Response,
	downloader *Downloader,
) {
	if err := resp.Body.Close(); err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d, fmt.Errorf("error closing http body: %v", err),
		)
	}
}

func (d *DownloadTask) validateResponse(
	resp *http.Response,
	downloader *Downloader,
) error {
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad status: %s", resp.Status)
		downloader.EventsChan() <- NewDownloadError(d, err)
		return err
	}
	return nil
}

func (d *DownloadTask) startDownload(
	downloader *Downloader,
	resp *http.Response,
) {
	downloader.EventsChan() <- NewDownloadStart(d, resp.ContentLength)
	d.progress.Reset()
}

func (d *DownloadTask) downloadBody(
	file *os.File,
	resp *http.Response,
	downloader *Downloader,
) error {
	tickerDone := make(chan struct{})
	defer close(tickerDone)
	go d.tickProgress(downloader, tickerDone)

	teeReader := io.TeeReader(resp.Body, d.progress)

	if _, err := io.Copy(file, teeReader); err != nil {
		downloader.EventsChan() <- NewDownloadError(
			d,
			fmt.Errorf("error downloading content: %s", err),
		)
		return err
	}

	return nil
}

func (d *DownloadTask) tickProgress(
	downloader *Downloader,
	done <-chan struct{},
) {
	ticker := time.NewTicker(downloader.statusUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			downloader.EventsChan() <- NewDownloadUpdate(d, d.progress.BytesRead())
		case <-done:
			return
		}
	}
}

func (d *DownloadTask) finishDownload(downloader *Downloader) {
	downloader.EventsChan() <- NewDownloadComplete(d, d.progress.BytesRead())
}
