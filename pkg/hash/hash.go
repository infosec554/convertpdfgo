package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// FileSHA256 streams the file from disk and returns its hex-encoded SHA-256.
// Uses io.Copy so the file is never fully buffered in memory.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReaderSHA256 hashes the contents of r without consuming the original buffer.
// The caller is responsible for rewinding the reader if needed.
func ReaderSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
