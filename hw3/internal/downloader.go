package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

type Downloader struct {
	userAgent string
}

func NewDownloader() *Downloader {
	return &Downloader{
		userAgent: "BOT FIT/CTU (student project)",
	}
}

func (d *Downloader) Download(url string, destination string) error {
	// TODO: client := &http.Client{ Timeout: time.Second * 5, }; ?
	// TODO: NewRequestWithContext
	// TODO: |os.O_EXCL
	file, err := os.OpenFile(
		destination,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error closing file: %v", err)
		}
	}(file)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error closing body: %s\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	//var totalBytes = float64(resp.ContentLength)
	teeReader := io.TeeReader(resp.Body, NewProgressWriter(resp.ContentLength))
	if _, err := io.Copy(file, teeReader); err != nil {
		return err
	}

	return nil
}
