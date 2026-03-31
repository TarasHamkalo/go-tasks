package downloader

import (
	"github.com/google/uuid"
)

type DownloadTask struct {
	id string

	progress *ProgressWriter

	//cancel func later
}

func NewDownloadTask() *DownloadTask {
	return &DownloadTask{
		id:       uuid.New().String(),
		progress: NewProgressWriter(-1),
	}
}

func (d *DownloadTask) Id() string {
	return d.id
}

//func (d *DownloadTask) Execute(downloader *Downloader) {
//	// TODO: client := &http.Client{ Timeout: time.Second * 5, }; ?
//	// TODO: NewRequestWithContext
//	// TODO: |os.O_EXCL
//	downloader.EventsChan() <- NewStartedEvent(d.Id())
//
//	file, err := os.OpenFile(
//		d.destination,
//		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
//		0644,
//	)
//
//	if err != nil {
//		downloader.EventsChan() <- NewErrorEvent(
//			d.Id(),
//			fmt.Errorf("error opening file: %v", err),
//		)
//		return
//	}
//
//	defer func() {
//		err := file.Close()
//		if err != nil {
//			downloader.EventsChan() <- NewErrorEvent(
//				d.Id(),
//				fmt.Errorf("error closing file: %v", err),
//			)
//		}
//	}()
//
//	req, err := http.NewRequest("GET", d.url, nil)
//	if err != nil {
//		downloader.EventsChan() <- NewErrorEvent(
//			d.Id(),
//			fmt.Errorf("error building request: %v", err),
//		)
//		return
//	}
//
//	req.Header.Set("Accept", "*/*")
//	req.Header.Set("User-Agent", downloader.UserAgent())
//
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		downloader.EventsChan() <- NewErrorEvent(
//			d.Id(),
//			fmt.Errorf("http client error: %v", err),
//		)
//		return
//	}
//
//	defer func() {
//		err := resp.Body.Close()
//		if err != nil {
//			downloader.EventsChan() <- NewErrorEvent(
//				d.Id(),
//				fmt.Errorf("error closing http body: %s\n", err),
//			)
//		}
//	}()
//
//	if resp.StatusCode != http.StatusOK {
//		downloader.EventsChan() <- NewErrorEvent(
//			d.Id(),
//			fmt.Errorf("bad status: %s", resp.Status),
//		)
//		return
//	}
//
//	d.progressWriter.Reset(resp.ContentLength)
//	teeReader := io.TeeReader(resp.Body, d.progressWriter)
//	if _, err := io.Copy(file, teeReader); err != nil {
//		downloader.EventsChan() <- NewErrorEvent(
//			d.Id(),
//			fmt.Errorf("error downloading content: %s", err),
//		)
//		return
//	}
//
//	// TODO: should have completion event
//	downloader.EventsChan() <- NewDoneEvent(d.Id())
//}
