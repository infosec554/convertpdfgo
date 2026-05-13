package objectstore

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStore is an S3-compatible Store backed by MinIO. The same struct
// works against AWS S3 by pointing endpoint at s3.amazonaws.com and
// setting useSSL=true.
type MinIOStore struct {
	client *minio.Client
	bucket string
}

// NewMinIO connects, ensures the target bucket exists, and returns a Store.
// `endpoint` is host:port without scheme.
func NewMinIO(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStore, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := c.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := c.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &MinIOStore{client: c, bucket: bucket}, nil
}

func (s *MinIOStore) Put(ctx context.Context, key string, r io.Reader, size int64, opts PutOptions) (int64, error) {
	// PartSize / NumThreads explicitly tuned for large object uploads.
	// MinIO's default part size is 16 MiB sequential; 8 MiB × 4 parallel
	// finishes a 100 MB upload roughly 2-3× faster on a slow uplink
	// because the parts overlap network and disk I/O.
	//
	// PartSize must be ≥ 5 MiB per the S3 spec; we pick 8 MiB so the
	// last part is rarely tiny.
	info, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: opts.ContentType,
		PartSize:    8 * 1024 * 1024,
		NumThreads:  4,
	})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (s *MinIOStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// minio.Object errors on Read if the key is missing; probe once so the
	// caller sees the failure right after Get rather than mid-stream.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, err
	}
	return obj, nil
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err == nil || IsNotExist(err) {
		return nil
	}
	return err
}

func (s *MinIOStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *MinIOStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (PresignedURL, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return PresignedURL{}, err
	}
	return PresignedURL{URL: u.String(), Method: "GET", ExpiresAt: time.Now().Add(ttl)}, nil
}

func (s *MinIOStore) PresignPut(ctx context.Context, key string, ttl time.Duration, _ string) (PresignedURL, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return PresignedURL{}, err
	}
	return PresignedURL{URL: u.String(), Method: "PUT", ExpiresAt: time.Now().Add(ttl)}, nil
}

// Compile-time assertion: *MinIOStore satisfies Store.
var _ Store = (*MinIOStore)(nil)

// IsNotExist reports whether the given error from a Get/Delete corresponds
// to a missing key.
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey" || errors.Is(err, io.EOF)
}
