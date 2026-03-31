package internal

import (
	"time"
)

type ProgressWriter struct {
	bytesRead int64

	// expectedSize used to calculate percentage of read bytes, use -1 if unkonwn
	expectedSize int64

	// trackingStartTime is time when received first byte
	trackingStartTime time.Time
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

	p.bytesRead += int64(len(b))
	return len(b), nil
}

// Stat returns bytes per second and in case expectedSize is -1
// returns bytes read bytes or percentage otherwise.
func (p *ProgressWriter) Stat() (float64, float64) {
	var speed float64
	var progress float64

	duration := time.Since(p.trackingStartTime)
	if duration.Seconds() > 0 {
		speed = float64(p.bytesRead) / duration.Seconds()
	}

	if p.expectedSize < 0 {
		// raw
		progress = float64(p.bytesRead)
	} else {
		// percentage
		progress = float64(p.bytesRead) / float64(p.expectedSize)
	}

	return speed, progress
}
