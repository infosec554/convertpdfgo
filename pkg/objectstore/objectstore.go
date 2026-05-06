// Package objectstore abstracts file storage so the rest of the app doesn't
// know whether bytes live on local disk, MinIO, or S3. The default
// implementation is a local-filesystem adapter that mirrors today's behaviour;
// swap it for an S3/MinIO-backed one when ready (see minio.go skeleton).
package objectstore

import (
	"context"
	"io"
	"time"
)

// PutOptions controls metadata for an upload. Empty fields are filled with
// sensible defaults by the implementation.
type PutOptions struct {
	ContentType string
}

// PresignedURL is a time-limited URL for direct client → object-store transfer.
type PresignedURL struct {
	URL       string
	Method    string // "GET" or "PUT"
	ExpiresAt time.Time
}

// Store is the contract every storage backend must satisfy.
type Store interface {
	// Put uploads data to key. Returns the size written.
	Put(ctx context.Context, key string, r io.Reader, size int64, opts PutOptions) (int64, error)

	// Get returns a reader for the object at key. Caller closes it.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object. No-op if the key does not exist.
	Delete(ctx context.Context, key string) error

	// Exists reports whether key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// PresignGet returns a URL the client can hit directly to download.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error)

	// PresignPut returns a URL the client can hit directly to upload.
	PresignPut(ctx context.Context, key string, ttl time.Duration, contentType string) (PresignedURL, error)
}
