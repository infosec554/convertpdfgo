package notify

import "context"

// Bus is the seam between event producers (job worker, auth handler,
// contact handler) and event consumers (the WS Hub today; a Redis
// subscriber tomorrow when we need multi-replica fan-out).
//
// PublishUser routes to the owning user's WS subscribers. UserID == ""
// is a no-op so guest jobs don't waste cycles.
//
// PublishAdmin broadcasts to every connected admin.
//
// All methods are non-blocking: if a subscriber is slow, its messages
// are dropped at the Hub rather than back-pressuring producers. Real-time
// notification "best-effort" is the right trade — losing one update is
// strictly better than wedging the worker.
type Bus interface {
	PublishUser(ctx context.Context, userID string, e Envelope) error
	PublishAdmin(ctx context.Context, e Envelope) error
}

// Noop discards everything; used when WS is disabled and in tests.
type Noop struct{}

func (Noop) PublishUser(_ context.Context, _ string, _ Envelope) error { return nil }
func (Noop) PublishAdmin(_ context.Context, _ Envelope) error          { return nil }
