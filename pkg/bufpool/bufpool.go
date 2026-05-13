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

// CopyBufSize is the per-call copy buffer for io.CopyBuffer. 1 MiB is the
// point where syscall overhead stops dominating io.Copy throughput on
// modern kernels — Go's default 32 KiB makes ~30× more read/write calls
// per large upload than necessary.
const CopyBufSize = 1 << 20

// slicePool keeps 1 MiB []byte slices alive across requests so the upload
// hot path (file upload, MinIO put, Gotenberg conversion) reuses them
// instead of allocating fresh on every request.
var slicePool = sync.Pool{
	New: func() any {
		b := make([]byte, CopyBufSize)
		return &b
	},
}

// GetBytes returns a reusable []byte of length CopyBufSize. Always return
// it via PutBytes; the slice header escapes the pool otherwise.
func GetBytes() []byte {
	return *slicePool.Get().(*[]byte)
}

// PutBytes returns a slice to the pool. Slices that have been re-sliced
// smaller (cap < CopyBufSize) are dropped — keeping them would pollute
// the pool with undersized buffers.
func PutBytes(b []byte) {
	if cap(b) < CopyBufSize {
		return
	}
	b = b[:cap(b)]
	slicePool.Put(&b)
}
