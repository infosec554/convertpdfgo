// Package loginlimiter throttles password / OTP attempts per email so a
// guesser can't trivially brute-force a credential. Backed by Redis so
// multiple API replicas share the same counter.
package loginlimiter

import (
	"context"
	"errors"
	"strconv"
	"time"

	"convertpdfgo/storage"
)

// ErrLocked is returned when the email has too many recent failures.
var ErrLocked = errors.New("too many login attempts; try again later")

// Limiter tracks failures per key with a sliding TTL.
type Limiter struct {
	rs       storage.IRedisStorage
	maxFails int
	window   time.Duration
}

// New returns a Limiter that locks an email after `maxFails` failed attempts
// within `window`. Recommended: 5 fails / 10 minutes.
func New(rs storage.IRedisStorage, maxFails int, window time.Duration) *Limiter {
	return &Limiter{rs: rs, maxFails: maxFails, window: window}
}

// Check returns ErrLocked if the email is currently locked. Always check
// before bcrypt — both to avoid wasting CPU and to make the lockout window
// observable to the caller.
func (l *Limiter) Check(ctx context.Context, email string) error {
	if l == nil || l.rs == nil {
		return nil
	}
	v, err := l.rs.Get(ctx, key(email))
	if err != nil {
		return nil // miss / redis hiccup → fail open, lockout is best-effort
	}
	n, _ := strconv.Atoi(v)
	if n >= l.maxFails {
		return ErrLocked
	}
	return nil
}

// RecordFailure increments the counter for email. Resets the TTL each time
// so the lockout slides forward on continued attempts.
func (l *Limiter) RecordFailure(ctx context.Context, email string) {
	if l == nil || l.rs == nil {
		return
	}
	current, _ := l.rs.Get(ctx, key(email))
	n, _ := strconv.Atoi(current)
	n++
	_ = l.rs.SetX(ctx, key(email), strconv.Itoa(n), l.window)
}

// Reset clears the counter on a successful login.
func (l *Limiter) Reset(ctx context.Context, email string) {
	if l == nil || l.rs == nil {
		return
	}
	_ = l.rs.SetX(ctx, key(email), "0", time.Second) // expire fast
}

// LockedEntry is one row in the admin "currently locked accounts" view.
type LockedEntry struct {
	Email     string
	FailCount int
}

// ListLocked returns every email with a currently-locked counter. SCAN's
// the login_fail:* keyspace; safe for an admin endpoint, not a hot path.
// Honours maxResults so the response stays bounded on a busy site.
func (l *Limiter) ListLocked(ctx context.Context, maxResults int) ([]LockedEntry, error) {
	if l == nil || l.rs == nil {
		return nil, nil
	}
	if maxResults <= 0 || maxResults > 1000 {
		maxResults = 200
	}
	keys, err := l.rs.Keys(ctx, "login_fail:*", maxResults)
	if err != nil {
		return nil, err
	}
	out := make([]LockedEntry, 0, len(keys))
	for _, k := range keys {
		v, err := l.rs.Get(ctx, k)
		if err != nil {
			continue
		}
		n, _ := strconv.Atoi(v)
		// Drop "0"s left by Reset() that haven't expired yet.
		if n < l.maxFails {
			continue
		}
		out = append(out, LockedEntry{
			Email:     k[len("login_fail:"):],
			FailCount: n,
		})
	}
	return out, nil
}

// Unlock — admin override. Wipes the lock entirely so the next attempt
// goes through bcrypt again.
func (l *Limiter) Unlock(ctx context.Context, email string) error {
	if l == nil || l.rs == nil {
		return nil
	}
	return l.rs.Del(ctx, key(email))
}

func key(email string) string {
	return "login_fail:" + email
}
