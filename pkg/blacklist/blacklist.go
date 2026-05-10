// Package blacklist tracks revoked JWT IDs in Redis so logged-out tokens
// can't be replayed before their natural expiry.
package blacklist

import (
	"context"
	"time"

	"convertpdfgo/storage"
)

type Blacklist struct {
	rs storage.IRedisStorage
}

func New(rs storage.IRedisStorage) *Blacklist {
	return &Blacklist{rs: rs}
}

// Revoke marks the given JTI as invalid for the remaining lifetime of the
// token. The TTL is set to expiry-now so Redis cleans up automatically.
func (b *Blacklist) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if b == nil || b.rs == nil || jti == "" {
		return nil
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // already expired, nothing to do
	}
	return b.rs.SetX(ctx, "revoked:"+jti, "1", ttl)
}

// IsRevoked reports whether a JTI is on the blacklist.
func (b *Blacklist) IsRevoked(ctx context.Context, jti string) bool {
	if b == nil || b.rs == nil || jti == "" {
		return false
	}
	v, err := b.rs.Get(ctx, "revoked:"+jti)
	return err == nil && v == "1"
}

// RevokeUser invalidates every token issued to the given user before `now`.
// Used by admin "force logout" and password change. We store a Unix
// timestamp; the auth middleware compares it against the token's iat
// claim. TTL matches the longest possible refresh-token lifetime so the
// gate stays in Redis as long as any old token could still be in flight.
func (b *Blacklist) RevokeUser(ctx context.Context, userID string, ttl time.Duration) error {
	if b == nil || b.rs == nil || userID == "" {
		return nil
	}
	now := time.Now().Unix()
	return b.rs.SetX(ctx, "user_revoked:"+userID, intToStr(now), ttl)
}

// UserRevokedAfter returns the Unix-second cut-off such that any token
// issued before it must be rejected. Returns 0 when no entry exists.
func (b *Blacklist) UserRevokedAfter(ctx context.Context, userID string) int64 {
	if b == nil || b.rs == nil || userID == "" {
		return 0
	}
	v, err := b.rs.Get(ctx, "user_revoked:"+userID)
	if err != nil || v == "" {
		return 0
	}
	return strToInt(v)
}

func intToStr(n int64) string {
	// Avoid pulling strconv into the hot path's import set; this is
	// monotonic so no negative case to worry about.
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}

func strToInt(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
