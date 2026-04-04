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
// It stores metadata needed to properly process IO operations and
// trigger updates of DownloadRecord (downloads' metadata).
// Given task does not modify DonwloadRecord entry directly, but initiates
// events (handled in Downloader).
// For full metadata stored about donwload see DownloadRecord.
type DownloadTask struct {

	// id is task id (default UUID)
	id string

	// downloadId is DownloadRecord associated with given task
	downloadId string

	// url is http resource being downloaded
	url string

	// destination is path on local file system, where resource gonna be stored
	// in case of partial completion (IO error/task cancellation) resource won't
	// be written to disk/removed.
	// -- if everything goes well =)
	destination string

	// errorExited is used to indicate whether task error occurred and partially
	// written resource should be removed from FS
	errorExited atomic.Bool

	// progress is instance of ProgressWriter tracking this download process
	// used to query status updates (in separate routine) and send update events
	// to downloader
	progress *ProgressWriter
}

// NewDownloadTask create default DownloadTask with ID set to UUID
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

		errorExited: atomic.Bool{}, // initial false

		progress: NewProgressWriter(),
	}
}

func (d *DownloadTask) Id() string {
	return d.id
}

// Execute handles IO operations associated with HTTP resource download process.
// Context passed to it, is used to cancel both HTTP headers request and
// body download/write process.
// NOTE: no further timeouts are set over HTTP request as body download process
// can last nondeterministically long.
func (d *DownloadTask) Execute(ctx context.Context, downloader *Downloader) {
	file, err := d.openFile(downloader)
	if err != nil {
		// file was not created
		d.errorExited.Store(true)
		return
	}

	// register function to close file and in case error occurred, remove it
	// NOTE: other way would be to write temporary file and then local IO copy
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

	// No further timeouts over context as it would include body download as well
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

// openFile opens file at destination, in case exists error occur
// Given error creates event sent to downloader
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

// doRequest makes initial get request to url, errors which occur
// are sent to downloader via events channel, in case context cancellation
// occur, cancel event sent instead of error.
func (d *DownloadTask) doRequest(
	ctx context.Context,
	downloader *Downloader,
) (*http.Response, error) {
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

// validateResponse verify response status and sent event to downloader in case
// of error
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

// startDownload notifies downloader about download start, occurs
// right before download body starts.
func (d *DownloadTask) startDownload(
	downloader *Downloader,
	resp *http.Response,
) {
	downloader.writeEventsChan() <- NewDownloadStart(d, resp.ContentLength)
	d.progress.Reset()
}

// downloadBody handles IO (download and pipe) associated with body download.
// Registers a routine polling for amount of read bytes from progressWriter.
// In case of error produces DownloadErrorEvent, in case of ctx cancellation
// DownloadCancelEvent.
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

// tickProgress registers tickers with interval provided by downloader,
// and sends DownloadUpdateEvents till ctx is not canceled.
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

// finishDownload indicates proper completion of download, sending event
// to downloader.
func (d *DownloadTask) finishDownload(downloader *Downloader) {
	downloader.writeEventsChan() <- NewDownloadComplete(
		d, d.progress.BytesRead(),
	)
}
