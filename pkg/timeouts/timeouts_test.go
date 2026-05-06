package timeouts

import (
	"testing"
	"time"
)

// Sanity guards: Process must comfortably exceed DB so handlers that do
// real work (compress, convert) don't get cut off by the wrong constant.
func TestTimeouts_RelativeOrdering(t *testing.T) {
	if DB <= 0 {
		t.Fatalf("DB timeout must be positive, got %v", DB)
	}
	if Process <= DB {
		t.Fatalf("Process (%v) must be greater than DB (%v)", Process, DB)
	}
}

func TestTimeouts_ReasonableBounds(t *testing.T) {
	if DB > 30*time.Second {
		t.Fatalf("DB timeout too high (%v) — DB ops should respond fast", DB)
	}
	if Process < time.Minute {
		t.Fatalf("Process timeout too low (%v) — pdfcpu/Gotenberg need >1min", Process)
	}
}
