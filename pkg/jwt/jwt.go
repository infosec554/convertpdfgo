package jwt

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

var ErrTokenInvalid = errors.New("token is invalid or expired")

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

// MustValidateSecret aborts startup if JWT_SECRET_KEY is missing, default,
// or shorter than 32 bytes. Call from main() so the failure surfaces at
// boot, not on the first /login call.
func MustValidateSecret() {
	secret := os.Getenv("JWT_SECRET_KEY")
	const minLen = 32
	switch {
	case secret == "":
		panic("JWT_SECRET_KEY is required (>= 32 chars). Set it in your .env / deployment.")
	case secret == "default_dev_secret_change_me" || secret == "default_secret_key":
		panic("JWT_SECRET_KEY is the placeholder default. Replace it before starting the service.")
	case len(secret) < minLen:
		panic(fmt.Sprintf("JWT_SECRET_KEY is too short (%d chars); need at least %d.", len(secret), minLen))
	}
}

func getSecretKey() []byte {
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		// Dev fallback. Production must call MustValidateSecret at startup
		// to refuse this value.
		secret = "default_dev_secret_change_me"
	}
	return []byte(secret)
}

// GenerateJWT issues an access + refresh pair for the given claim payload.
// Each token gets a unique JTI so we can revoke it via Redis on logout.
func GenerateJWT(claims map[string]interface{}) (access string, refresh string, err error) {
	now := time.Now()
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	access, err = sign(claims, accessJTI, TokenTypeAccess, now, now.Add(AccessTokenTTL))
	if err != nil {
		return "", "", err
	}
	refresh, err = sign(claims, refreshJTI, TokenTypeRefresh, now, now.Add(RefreshTokenTTL))
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func sign(payload map[string]interface{}, jti, typ string, iat, exp time.Time) (string, error) {
	tok := jwt.New(jwt.SigningMethodHS256)
	c := tok.Claims.(jwt.MapClaims)
	for k, v := range payload {
		c[k] = v
	}
	c["jti"] = jti
	c["typ"] = typ
	c["iat"] = iat.Unix()
	c["exp"] = exp.Unix()
	return tok.SignedString(getSecretKey())
}

// ParseToken validates the signature/expiry and returns the claims map.
func ParseToken(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return getSecretKey(), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		out := make(map[string]interface{}, len(claims))
		for k, v := range claims {
			out[k] = v
		}
		return out, nil
	}
	return nil, ErrTokenInvalid
}

// ExtractClaims is the legacy helper used by the auth middleware. It returns
// the user_id / user_role / jti / typ / exp from the token.
func ExtractClaims(tokenString string) (map[string]interface{}, error) {
	parsed, err := ParseToken(tokenString)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if v, ok := parsed["user_id"]; ok {
		out["user_id"] = stringify(v)
	}
	if v, ok := parsed["user_role"]; ok {
		out["user_role"] = stringify(v)
	}
	if v, ok := parsed["jti"]; ok {
		out["jti"] = stringify(v)
	}
	if v, ok := parsed["typ"]; ok {
		out["typ"] = stringify(v)
	}
	if v, ok := parsed["exp"]; ok {
		out["exp"] = v
	}
	return out, nil
}

// ExpiresAt returns the token's expiry time, useful when computing how long
// to keep a JTI in the Redis blacklist.
func ExpiresAt(claims map[string]interface{}) time.Time {
	switch v := claims["exp"].(type) {
	case float64:
		return time.Unix(int64(v), 0)
	case int64:
		return time.Unix(v, 0)
	}
	return time.Time{}
}

func stringify(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
