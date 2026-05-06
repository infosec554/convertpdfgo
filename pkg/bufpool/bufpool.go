package bufpool

import (
	"bytes"
	"sync"
)

// pool reuses *bytes.Buffer across requests to keep GC pressure off the
// hot multipart-upload / template-render paths.
var pool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// Get returns an empty buffer ready for use. Always return it via Put when
// you are done; do not keep the reference past that point.
func Get() *bytes.Buffer {
	b := pool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// Put returns the buffer to the pool. Buffers larger than 1 MiB are dropped
// so we don't pin large allocations indefinitely.
func Put(b *bytes.Buffer) {
	if b == nil {
		return
	}
	const maxKeep = 1 << 20 // 1 MiB
	if b.Cap() > maxKeep {
		return
	}
	pool.Put(b)
}
