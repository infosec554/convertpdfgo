package bufpool

import (
	"strings"
	"testing"
)

func TestGetReturnsEmpty(t *testing.T) {
	b := Get()
	defer Put(b)
	if b == nil {
		t.Fatalf("Get returned nil")
	}
	if b.Len() != 0 {
		t.Fatalf("buffer must be empty on Get, len=%d", b.Len())
	}
}

func TestPutResetsForReuse(t *testing.T) {
	b := Get()
	b.WriteString("dirty")
	Put(b)

	b2 := Get()
	defer Put(b2)
	if b2.Len() != 0 {
		t.Fatalf("buffer must be reset on next Get, len=%d", b2.Len())
	}
}

func TestPut_NilSafe(t *testing.T) {
	// Should not panic.
	Put(nil)
}

func TestPut_DropsOversizedBuffers(t *testing.T) {
	b := Get()
	b.WriteString(strings.Repeat("x", (1<<20)+1)) // > 1 MiB
	Put(b) // dropped, not pooled
	// No way to assert from outside that pool is empty, but at minimum
	// Put must not panic and the next Get must still hand back a fresh buf.
	b2 := Get()
	defer Put(b2)
	if b2.Len() != 0 {
		t.Fatalf("next buffer must be empty, len=%d", b2.Len())
	}
}
