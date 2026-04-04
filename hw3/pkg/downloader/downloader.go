// Package downloader provides functionality to manage async download tasks.
package downloader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Downloader is main object managing downloads lifecycle.
//
// Each download is represented by two structs DownloadRecord and DownloadTask.
//
// DownloadRecord stores download metadata, it is never deleted from Downloader,
// and can always be queried by its ID. DownloadTask is temporary object
// handling download process, and notifying Downloader via events.
//
// Each download is handled in separate go routine associated with private context.
// That context is stored inside of TasksStore and allows cancellation at any time.
//
// After submitting new download task, downloader acts a central unit applying
// updates received via Downloader's eventsChan (from DownloadTask) to corresponding
// DownloadRecord objects stored in DownloadsStore.
// (see startEventHandlerLoop)
//
// NOTE: Before submitting requests, Start method should be called
// to start event processing loop.
//
// NOTE: After shutdown new instance should be created.
type Downloader struct {
	// userAgent set when creating HTTP requests
	userAgent string

	// statusUpdateInterval how often to update DownloadRecord
	// e.g. more bytes downloaded
	statusUpdateInterval time.Duration

	// downloadTasks stores all running tasks (removed after completion)
	downloadTasks *TasksStore

	// downloadsStore stores all ever created DownloadRecord
	downloadsStore *DownloadsStore

	// eventsChan chan used to notify downloader about events occured inside of
	// DownloadTask
	// NOTE: should be accessed by writeEventsChan
	eventsChan chan DownloadEvent

	// subscribers stores event channels to each subcribed consumer of download
	// events
	subscribers []chan ExternalDownloadEvent
	// subMu used manage subscribers
	subMu sync.RWMutex

	// shutdownChan is closed when shutdown timeout exited
	// (indicating all downloader routines should exit, e.g. event loop)
	shutdownChan chan struct{}

	// shuttingDown indicates that shutdown process started
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
		downloadsStore: NewDownloadsStore(),

		eventsChan: make(chan DownloadEvent, 10),

		subscribers: make([]chan ExternalDownloadEvent, 0),

		shutdownChan: make(chan struct{}),

		logger: logger,
	}
}

// Start event processing loop
func (d *Downloader) Start() {
	go d.startEventHandlerLoop()
}

// Shutdown handles Downloader shutdown logic. Expects to receive
// a context with timeout, otherwise 5 seconds duration is assumed.
//
// Shutdown is handled in two phases, each having timeout/2 time:
//  1. don't accept new tasks, await existing to finish
//  2. cancel all tasks and await until they quit (see TasksStore DrainOnly)
//
// After timeout or graceful tasks completion allow event loop routine to quit
// and return.
//
// NOTE: In case tasks didn't quit by cancellation local system resources
// (downloaded files) can be partially written
func (d *Downloader) Shutdown(ctx context.Context) {
	d.logger.Info("Downloader shutting down")

	// stop receiving new downloads
	d.shuttingDown.Store(true)

	total := time.Until(deadline(ctx))
	if total <= 0 {
		total = 0
	}

	graceTime := total / 2
	drainTime := total - graceTime

	d.logger.Debug(
		"Downloader awaiting task completion for",
		zap.Duration("time", graceTime),
	)

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
	d.logger.Debug(
		"Downloader awaiting events drain for",
		zap.Duration("time", drainTime),
	)

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

// deadline is helper to compute deadline from context, if deadline not set
// 5 seconds assumed.
func deadline(ctx context.Context) time.Time {
	dl, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(5 * time.Second)
	}
	return dl
}

// startEventHandlerLoop handles all events received through eventsChan as:
//  1. logs event,
//  2. handle mutation of DownloadRecord,
//  3. broadcast update to subscribers.
//
// Stop when shutdownChan closed.
func (d *Downloader) startEventHandlerLoop() {
	for {
		select {
		case event := <-d.eventsChan:
			d.logger.Debug("download event",
				zap.String("type", string(event.Type())),
				zap.String("downloadId", event.DownloadId()),
				zap.String("taskId", event.TaskId()),
				zap.Any("data[any]", event.Data()),
			)

			download, err := d.downloadsStore.Get(event.DownloadId())
			if err != nil {
				d.logger.Error("can not handle event",
					zap.String("type", string(event.Type())),
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

// handleDownloadEvent applies updates to DownloadRecord according
// to event type.
func (d *Downloader) handleDownloadEvent(
	event DownloadEvent,
	download *DownloadRecord,
) {
	switch event.Type() {
	case DownloadEventStart:
		download.Start(event.Int64())
		d.broadcastPublic(
			NewExternalDownloadStart(event.DownloadId(), event.Int64()),
		)

	case DownloadEventUpdate:
		download.SetBytesDownloaded(event.Int64())

	case DownloadEventCancel:
		// if task provided cancellation event, don't cancel it twice
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

// handleTaskRemoval removes task and cancel context if needed
func (d *Downloader) handleTaskRemoval(event DownloadEvent, withCancel bool) {
	taskEntry, err := d.downloadTasks.Remove(event.TaskId())
	if err != nil {
		d.logger.Warn(
			"could not remove task",
			zap.String("type", string(event.Type())),
			zap.String("downloadId", event.DownloadId()),
			zap.String("taskId", event.TaskId()),
			zap.Error(err),
		)
		return
	}

	if withCancel {
		taskEntry.Cancel()
	}
}

// SubmitDownload creates structs to represent download and corresponding task.
// If stored successfully starts go routine handling given download.
// DownloadTask receives child context (of given ctx) with cancellation functino
// stored for later use.
func (d *Downloader) SubmitDownload(
	ctx context.Context,
	url string,
	destination string,
) (string, error) {

	if d.shuttingDown.Load() {
		return "", fmt.Errorf("downloader is shutting down")
	}

	taskCtx, cancel := context.WithCancel(ctx)

	download := NewDownloadRecord(url, destination)
	task := NewDownloadTask(download.Id(), url, destination)
	download.SetTaskId(task.Id())

	entry := NewTaskEntry(task, cancel)
	if err := d.downloadTasks.Add(entry); err != nil {
		cancel()
		return "", err
	}

	if err := d.downloadsStore.Add(download); err != nil {
		// rollback
		_, _ = d.downloadTasks.Remove(task.Id())
		cancel()
		return "", err
	}

	d.logger.Debug("Task added",
		zap.String("taskId", task.Id()),
		zap.String("downloadId", download.Id()),
	)

	go task.Execute(taskCtx, d)
	return download.Id(), nil
}

// GetDownload returns DownloadView which corresponds to given downloadId
func (d *Downloader) GetDownload(downloadId string) (*DownloadView, error) {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return nil, err
	}

	return download.DetachedView(), nil
}

func (d *Downloader) GetAllDownloads() []*DownloadView {
	return d.downloadsStore.GetAllViews()
}

func (d *Downloader) GetAllDownloadIds() []string {
	return d.downloadsStore.GetAllIds()
}

// CancelDownload finds download with given id and cancels context
// of associated download task.
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

// GetCompletionChan returns channel which gonna be closed download
// associated with downloadId completes.
func (d *Downloader) GetCompletionChan(
	downloadId string,
) (<-chan struct{}, error) {
	download, err := d.downloadsStore.Get(downloadId)
	if err != nil {
		return nil, err
	}

	return download.Done(), nil
}

// Subscribe returns private chanel to which events gonna be forwarded
func (d *Downloader) Subscribe() <-chan ExternalDownloadEvent {
	d.subMu.Lock()
	defer d.subMu.Unlock()
	ch := make(chan ExternalDownloadEvent, 10)
	d.subscribers = append(d.subscribers, ch)

	return ch
}

// broadcastPublic sends events to all registered subscribers.
// NOTE: when channel is full, given subscriber is ignored
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

func (d *Downloader) UserAgent() string {
	return d.userAgent
}

// writeEventsChan casts eventsChan to write only
func (d *Downloader) writeEventsChan() chan<- DownloadEvent {
	return d.eventsChan
}
