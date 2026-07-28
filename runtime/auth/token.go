package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

// APITokenPrefix define package-level implementation state.
const APITokenPrefix = "gk_"

// APIToken defines an implementation type used by this package.
type APIToken struct {
	// ID store data used by this type.
	ID string
	// UserID store data used by this type.
	UserID string
	// Name store data used by this type.
	Name string
	// TokenHash store data used by this type.
	TokenHash string
	// Abilities store data used by this type.
	Abilities []string
	// ExpiresAt store data used by this type.
	ExpiresAt *time.Time
	// LastUsedAt store data used by this type.
	LastUsedAt *time.Time
	// RevokedAt store data used by this type.
	RevokedAt *time.Time
}

// TokenStore defines an implementation type used by this package.
type TokenStore interface {
	// FindByTokenHash define an operation required by this interface.
	FindByTokenHash(context.Context, string) (*APIToken, error)
	// TouchLastUsed define an operation required by this interface.
	TouchLastUsed(context.Context, string, time.Time) error
}

// HashAPIToken performs this package operation.
func HashAPIToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}

// NewAPIToken performs this package operation.
func NewAPIToken(userID, name string, abilities []string, expiresIn *int64) (APIToken, string, error) {
	if userID == "" || (expiresIn != nil && *expiresIn <= 0) {
		return APIToken{}, "", errors.New("invalid token parameters")
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return APIToken{}, "", e
	}
	p := APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	var x *time.Time
	if expiresIn != nil {
		t := time.Now().Add(time.Duration(*expiresIn) * time.Second)
		x = &t
	}
	return APIToken{UserID: userID, Name: name, TokenHash: HashAPIToken(p), Abilities: normalizeAbilities(abilities), ExpiresAt: x}, p, nil
}

// normalizeAbilities performs this package operation.
func normalizeAbilities(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, a := range in {
		a = strings.TrimSpace(a)
		if a != "" && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// TokenFromContext performs this package operation.
func TokenFromContext(c *gin.Context) (*APIToken, bool) {
	v, ok := c.Get("gin-kit.auth.api_token")
	t, good := v.(*APIToken)
	return t, ok && good && t != nil
}

// Can performs this package operation.
func Can(c *gin.Context, a string) bool {
	t, ok := TokenFromContext(c)
	if !ok {
		return false
	}
	for _, x := range t.Abilities {
		if x == "*" || x == a {
			return true
		}
	}
	return false
}

// RequireToken performs this package operation.
func RequireToken(store TokenStore, abilities ...string) gin.HandlerFunc {
	if store == nil {
		panic("auth: RequireToken requires a non-nil store")
	}
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "missing_token", "Bearer token required."))
			return
		}
		p := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		t, e := store.FindByTokenHash(c, pHash(p))
		now := time.Now()
		if e != nil || t == nil || !strings.HasPrefix(p, APITokenPrefix) || t.RevokedAt != nil || (t.ExpiresAt != nil && !t.ExpiresAt.After(now)) {
			httpx.Fail(c, httpx.NewError(401, "invalid_token", "Token is invalid or expired."))
			return
		}
		for _, w := range normalizeAbilities(abilities) {
			if !contains(t.Abilities, w) {
				httpx.Fail(c, httpx.NewError(403, "insufficient_scope", "Token does not have the required ability."))
				return
			}
		}
		_ = store.TouchLastUsed(c, t.ID, now)
		c.Set("gin-kit.auth.api_token", t)
		c.Set(userIDKey, t.UserID)
		c.Next()
	}
}

// pHash performs this package operation.
func pHash(s string) string { return HashAPIToken(s) }

// contains performs this package operation.
func contains(h []string, w string) bool {
	for _, a := range h {
		if a == "*" || a == w {
			return true
		}
	}
	return false
}
