package hash

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSHA256_KnownValue(t *testing.T) {
	// SHA-256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	dir := t.TempDir()
	path := filepath.Join(dir, "h.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := FileSHA256(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("hash mismatch:\n  got  %s\n  want %s", got, want)
	}
}

func TestFileSHA256_MissingFile(t *testing.T) {
	if _, err := FileSHA256("/nonexistent/path/to/file"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestReaderSHA256(t *testing.T) {
	got, err := ReaderSHA256(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("hash mismatch:\n  got  %s\n  want %s", got, want)
	}
}

func TestReaderSHA256_Empty(t *testing.T) {
	got, err := ReaderSHA256(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	// SHA-256("") known constant.
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Fatalf("hash mismatch:\n  got  %s\n  want %s", got, want)
	}
}
