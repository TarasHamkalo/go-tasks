package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DownloadTask represents a temporary object of running download.
// Based on that, it does not contain status but rather holds data
// needed to cancel download or track its completion.
// For full metadata stored about donwload see DownloadRecord.
type DownloadTask struct {
	id string

	// cross reference
	downloadId string

	url string

	destination string

	// will be closed after task finishes
	errorExited atomic.Bool

	progress *ProgressWriter
}

func NewDownloadTask(
	downloadId string,
	url string,
	destination string,
) *DownloadTask {
	return &DownloadTask{
		id:         uuid.New().String(),
		downloadId: downloadId,

		url:         url,
		destination: destination,

		progress: NewProgressWriter(),
	}
}

func (d *DownloadTask) Id() string {
	return d.id
}

func (d *DownloadTask) Execute(ctx context.Context, downloader *Downloader) {
	file, err := d.openFile(downloader)
	if err != nil {
		d.errorExited.Store(true)
		return
	}

	defer (func() {
		d.closeFile(file, downloader)
		if !d.errorExited.Load() {
			return
		}

		if err := os.Remove(file.Name()); err != nil {
			downloader.logger.Warn(
				"error removing partially written file", zap.Error(err),
			)
		}
	})()

	resp, err := d.doRequest(ctx, downloader)
	if err != nil {
		d.errorExited.Store(true)
		return
	}
	defer d.closeResponse(resp, downloader)

	if err := d.validateResponse(resp, downloader); err != nil {
		d.errorExited.Store(true)
		return
	}

	d.startDownload(downloader, resp)
	if err := d.downloadBody(ctx, file, resp, downloader); err != nil {
		d.errorExited.Store(true)
		return
	}

	d.finishDownload(downloader)
}

func (d *DownloadTask) openFile(downloader *Downloader) (*os.File, error) {
	file, err := os.OpenFile(
		d.destination,
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0644,
	)
	if err != nil {
		downloader.writeEventsChan() <- NewDownloadError(
			d, fmt.Errorf("error opening file: %v", err),
		)
		return nil, err
	}
	return file, nil
}

func (d *DownloadTask) closeFile(file *os.File, downloader *Downloader) {
	if err := file.Close(); err != nil {
		downloader.logger.Warn("error closing file", zap.Error(err))
	}
}

func (d *DownloadTask) doRequest(
	ctx context.Context,
	downloader *Downloader,
) (*http.Response, error) {
	// No further timeouts over context as it would include body download as well
	req, err := http.NewRequestWithContext(ctx, "GET", d.url, nil)
	if err != nil {
		downloader.writeEventsChan() <- NewDownloadError(
			d, fmt.Errorf("error building request: %v", err),
		)
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", downloader.UserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			downloader.writeEventsChan() <- NewDownloadCancel(d)
			return nil, err
		}

		downloader.writeEventsChan() <- NewDownloadError(
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
		downloader.logger.Warn("error closing http body", zap.Error(err))
	}
}

func (d *DownloadTask) validateResponse(
	resp *http.Response,
	downloader *Downloader,
) error {
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad status: %s", resp.Status)
		downloader.writeEventsChan() <- NewDownloadError(d, err)
		return err
	}
	return nil
}

func (d *DownloadTask) startDownload(
	downloader *Downloader,
	resp *http.Response,
) {
	downloader.writeEventsChan() <- NewDownloadStart(d, resp.ContentLength)
	d.progress.Reset()
}

func (d *DownloadTask) downloadBody(
	ctx context.Context,
	file *os.File,
	resp *http.Response,
	downloader *Downloader,
) error {
	tickerCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	go d.tickProgress(tickerCtx, downloader)

	teeReader := io.TeeReader(resp.Body, d.progress)
	if _, err := io.Copy(file, teeReader); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			downloader.writeEventsChan() <- NewDownloadCancel(d)
			return err
		}

		downloader.writeEventsChan() <- NewDownloadError(
			d,
			fmt.Errorf("error downloading content: %s", err),
		)
		return err
	}

	return nil
}

func (d *DownloadTask) tickProgress(
	ctx context.Context,
	downloader *Downloader,
) {
	ticker := time.NewTicker(downloader.statusUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			downloader.writeEventsChan() <- NewDownloadUpdate(
				d, d.progress.BytesRead(),
			)
		case <-ctx.Done():
			return
		}
	}
}

func (d *DownloadTask) finishDownload(downloader *Downloader) {
	downloader.writeEventsChan() <- NewDownloadComplete(
		d, d.progress.BytesRead(),
	)
}
