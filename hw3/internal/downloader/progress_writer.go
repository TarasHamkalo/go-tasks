package downloader

import (
	"sync/atomic"
)

type ProgressWriter struct {
	bytesRead int64
}

func NewProgressWriter() *ProgressWriter {
	return &ProgressWriter{}
}

func (p *ProgressWriter) Reset() {
	p.bytesRead = 0
}

func (p *ProgressWriter) Write(b []byte) (n int, err error) {
	atomic.AddInt64(&p.bytesRead, int64(len(b)))
	return len(b), nil
}

func (p *ProgressWriter) BytesRead() int64 {
	return atomic.LoadInt64(&p.bytesRead)
}
