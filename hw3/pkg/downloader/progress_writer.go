package downloader

import (
	"sync/atomic"
)

// ProgressWriter is used to count bytes read during response body download.
// It is safe to access given struct from multiple threads
// (e.g. one downloader thread and one reader/query thread).
// Expected usage is to add given writer to TeeReader.
type ProgressWriter struct {
	bytesRead int64
}

func NewProgressWriter() *ProgressWriter {
	return &ProgressWriter{}
}

func (p *ProgressWriter) Reset() {
	atomic.StoreInt64(&p.bytesRead, 0)
}

func (p *ProgressWriter) Write(b []byte) (n int, err error) {
	atomic.AddInt64(&p.bytesRead, int64(len(b)))
	return len(b), nil
}

func (p *ProgressWriter) BytesRead() int64 {
	return atomic.LoadInt64(&p.bytesRead)
}
