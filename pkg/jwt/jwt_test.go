package jwt

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Tests need a stable, valid secret. Restore whatever was set before.
	prev := os.Getenv("JWT_SECRET_KEY")
	os.Setenv("JWT_SECRET_KEY", strings.Repeat("a", 64))
	code := m.Run()
	os.Setenv("JWT_SECRET_KEY", prev)
	os.Exit(code)
}

func TestGenerateAndParse(t *testing.T) {
	access, refresh, err := GenerateJWT(map[string]interface{}{
		"user_id":   "u-1",
		"user_role": "admin",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatalf("empty tokens")
	}
	if access == refresh {
		t.Fatalf("access and refresh must differ (different JTI)")
	}

	claims, err := ParseToken(access)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if got := claims["user_id"]; got != "u-1" {
		t.Fatalf("user_id: want u-1, got %v", got)
	}
	if got := claims["user_role"]; got != "admin" {
		t.Fatalf("user_role: want admin, got %v", got)
	}
	if got := claims["typ"]; got != TokenTypeAccess {
		t.Fatalf("typ: want %s, got %v", TokenTypeAccess, got)
	}
	if claims["jti"] == "" {
		t.Fatalf("jti must be set")
	}
}

func TestParseRejectsTampered(t *testing.T) {
	access, _, err := GenerateJWT(map[string]interface{}{"user_id": "u-1"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Flip a byte in the payload section. Signature won't match anymore.
	tampered := access[:len(access)-5] + "AAAAA"
	if _, err := ParseToken(tampered); err == nil {
		t.Fatalf("parse should reject tampered token")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	// Forge an already-expired token by signing with negative TTL.
	expired, err := sign(map[string]interface{}{"user_id": "u-1"}, "j-1", TokenTypeAccess,
		time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ParseToken(expired); err == nil {
		t.Fatalf("parse should reject expired token")
	}
}

func TestExpiresAt(t *testing.T) {
	access, _, _ := GenerateJWT(map[string]interface{}{"user_id": "u-1"})
	claims, _ := ParseToken(access)
	exp := ExpiresAt(claims)
	if time.Until(exp) <= 0 {
		t.Fatalf("expires_at must be in the future, got %v", exp)
	}
	if time.Until(exp) > AccessTokenTTL+time.Minute {
		t.Fatalf("expires_at must be within ~AccessTokenTTL, got %v", time.Until(exp))
	}
}

func TestMustValidateSecret(t *testing.T) {
	cases := []struct {
		name     string
		val      string
		shouldPanic bool
	}{
		{"empty", "", true},
		{"placeholder dev", "default_dev_secret_change_me", true},
		{"placeholder default", "default_secret_key", true},
		{"too short", "short", true},
		{"valid 32", strings.Repeat("x", 32), false},
		{"valid 64", strings.Repeat("y", 64), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("JWT_SECRET_KEY", tc.val)
			defer os.Setenv("JWT_SECRET_KEY", strings.Repeat("a", 64))

			defer func() {
				r := recover()
				if tc.shouldPanic && r == nil {
					t.Fatalf("expected panic for %q", tc.val)
				}
				if !tc.shouldPanic && r != nil {
					t.Fatalf("unexpected panic for %q: %v", tc.val, r)
				}
			}()
			MustValidateSecret()
		})
	}
}
