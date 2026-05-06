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
