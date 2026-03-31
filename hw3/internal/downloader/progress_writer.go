package downloader

import (
	"sync/atomic"
	"time"
)

type Progress struct {
	Speed         float64
	BytesRead     int64
	ExpectedTotal int64
}

type ProgressWriter struct {
	bytesRead int64

	// expectedSize used to calculate percentage of read bytes, use -1 if unkonwn
	expectedSize int64

	// trackingStartTime is time when received first byte
	trackingStartTime time.Time
}

func (p *ProgressWriter) Reset(expectedSize int64) {
	p.bytesRead = 0
	p.trackingStartTime = time.Time{}
	p.expectedSize = expectedSize
}

func NewProgressWriter(expectedSize int64) *ProgressWriter {
	return &ProgressWriter{
		expectedSize: expectedSize,
	}
}

func (p *ProgressWriter) Write(b []byte) (n int, err error) {
	if p.trackingStartTime.IsZero() {
		p.trackingStartTime = time.Now()
	}

	//p.bytesRead += int64(len(b))
	atomic.AddInt64(&p.bytesRead, int64(len(b)))
	return len(b), nil
}

// Stat returns bytes per second and in case expectedSize is -1
// returns bytes read bytes or percentage otherwise.
func (p *ProgressWriter) Stat() Progress {
	bytesRead := atomic.LoadInt64(&p.bytesRead)
	duration := time.Since(p.trackingStartTime)

	return Progress{
		Speed:         float64(bytesRead) / duration.Seconds(),
		BytesRead:     bytesRead,
		ExpectedTotal: p.expectedSize,
	}
}
