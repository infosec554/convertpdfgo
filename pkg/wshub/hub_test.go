package wshub

import (
	"sync"
	"sync/atomic"
	"testing"
)

// fakeClient is a Client stub that records every Send into a channel
// so tests can assert exact delivery without a real websocket.
type fakeClient struct {
	uid     string
	admin   bool
	mu      sync.Mutex
	got     [][]byte
	dropped atomic.Int32
}

func (f *fakeClient) UserID() string { return f.uid }
func (f *fakeClient) IsAdmin() bool  { return f.admin }
func (f *fakeClient) Send(p []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, p)
}
func (f *fakeClient) snapshot() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.got))
	copy(out, f.got)
	return out
}

func TestSendToUser_FansOutToAllUserClients(t *testing.T) {
	h := NewHub()
	a1 := &fakeClient{uid: "u1"}
	a2 := &fakeClient{uid: "u1"} // same user, second tab
	b := &fakeClient{uid: "u2"}
	h.Register(a1)
	h.Register(a2)
	h.Register(b)

	h.SendToUser("u1", []byte("hi"))

	if got := len(a1.snapshot()); got != 1 {
		t.Fatalf("a1: want 1 message, got %d", got)
	}
	if got := len(a2.snapshot()); got != 1 {
		t.Fatalf("a2: want 1 message, got %d", got)
	}
	if got := len(b.snapshot()); got != 0 {
		t.Fatalf("b (other user): must NOT receive, got %d", got)
	}
}

func TestSendToUser_EmptyIDIsNoop(t *testing.T) {
	h := NewHub()
	c := &fakeClient{uid: ""}
	h.Register(c)
	h.SendToUser("", []byte("nope"))
	if got := len(c.snapshot()); got != 0 {
		t.Fatalf("guest must never receive (got %d)", got)
	}
}

func TestBroadcastAdmins_OnlyAdmins(t *testing.T) {
	h := NewHub()
	user := &fakeClient{uid: "u1"}
	adm := &fakeClient{uid: "admin1", admin: true}
	h.Register(user)
	h.Register(adm)

	h.BroadcastAdmins([]byte("alert"))

	if got := len(user.snapshot()); got != 0 {
		t.Fatalf("plain user must NOT receive admin broadcast")
	}
	if got := len(adm.snapshot()); got != 1 {
		t.Fatalf("admin must receive (got %d)", got)
	}
}

func TestUnregister_StopsDelivery(t *testing.T) {
	h := NewHub()
	c := &fakeClient{uid: "u1"}
	h.Register(c)
	h.Unregister(c)

	h.SendToUser("u1", []byte("late"))
	if got := len(c.snapshot()); got != 0 {
		t.Fatalf("unregistered client must not receive")
	}
}

func TestUnregister_IsIdempotent(t *testing.T) {
	h := NewHub()
	c := &fakeClient{uid: "u1"}
	h.Register(c)
	h.Unregister(c)
	h.Unregister(c) // must not panic
}

func TestCounts(t *testing.T) {
	h := NewHub()
	h.Register(&fakeClient{uid: "u1"})
	h.Register(&fakeClient{uid: "u1"}) // 2nd tab
	h.Register(&fakeClient{uid: "u2"})
	h.Register(&fakeClient{uid: "admin1", admin: true})

	users, conns, admins := h.Counts()
	if users != 3 {
		t.Fatalf("users: want 3, got %d", users)
	}
	if conns != 4 {
		t.Fatalf("conns: want 4, got %d", conns)
	}
	if admins != 1 {
		t.Fatalf("admins: want 1, got %d", admins)
	}
}

// TestConcurrent_RegisterUnregisterBroadcast hammers the Hub from many
// goroutines to surface any data race the RWMutex misses (run with
// `go test -race`). Failure mode: Go race detector or a panic.
func TestConcurrent_RegisterUnregisterBroadcast(t *testing.T) {
	h := NewHub()
	const workers = 16
	const iters = 200

	var wg sync.WaitGroup
	wg.Add(workers * 3)

	// Register/unregister storm.
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c := &fakeClient{uid: "u1"}
				h.Register(c)
				h.Unregister(c)
			}
		}()
	}
	// Concurrent SendToUser readers.
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				h.SendToUser("u1", []byte("x"))
			}
		}()
	}
	// Concurrent admin broadcasts.
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				h.BroadcastAdmins([]byte("a"))
			}
		}()
	}
	wg.Wait()
}
