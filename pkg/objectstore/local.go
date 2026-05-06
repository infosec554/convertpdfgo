package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStore writes objects to a directory on the local filesystem. It's the
// default for development and existing deployments — operations behave like
// the original code path, plus a Store interface for future S3 swap-in.
type LocalStore struct {
	root string
}

func NewLocal(root string) *LocalStore {
	return &LocalStore{root: root}
}

func (s *LocalStore) path(key string) string {
	return filepath.Join(s.root, key)
}

func (s *LocalStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ PutOptions) (int64, error) {
	full := s.path(key)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, fmt.Errorf("mkdir: %w", err)
	}
	dst, err := os.Create(full)
	if err != nil {
		return 0, fmt.Errorf("create: %w", err)
	}
	defer dst.Close()
	n, err := io.Copy(dst, r)
	if err != nil {
		_ = os.Remove(full)
		return 0, fmt.Errorf("write: %w", err)
	}
	return n, nil
}

func (s *LocalStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(s.path(key))
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	err := os.Remove(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(s.path(key))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// PresignGet on a local filesystem can't issue real presigned URLs; we return
// a path-based URL so upstream code can detect "no presign available" by
// comparing Method == "" or by feature-checking the backend.
func (s *LocalStore) PresignGet(_ context.Context, key string, ttl time.Duration) (PresignedURL, error) {
	return PresignedURL{
		URL:       "/file/" + key,
		Method:    "GET",
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (s *LocalStore) PresignPut(_ context.Context, _ string, _ time.Duration, _ string) (PresignedURL, error) {
	return PresignedURL{}, errors.New("presigned PUT not supported on local filesystem; use POST /file/upload")
}
