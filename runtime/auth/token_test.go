package auth

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// tokenStore defines an implementation type used by this package.
type tokenStore struct {
	// token store data used by this type.
	token *APIToken
	// touched store data used by this type.
	touched bool
}

// FindByTokenHash performs this package operation.
func (s *tokenStore) FindByTokenHash(context.Context, string) (*APIToken, error) { return s.token, nil }

// TouchLastUsed performs this package operation.
func (s *tokenStore) TouchLastUsed(context.Context, string, time.Time) error {
	s.touched = true
	return nil
}

func TestNewAPITokenAndHash(t *testing.T) {
	plainToken, plain, err := NewAPIToken("u1", "cli", []string{" tickets:read ", "", "tickets:read", "*"}, nil)
	if err != nil || len(plain) < len(APITokenPrefix) || plainToken.TokenHash != HashAPIToken(plain) {
		t.Fatalf("unexpected token: %#v %q %v", plainToken, plain, err)
	}
	if len(plainToken.Abilities) != 2 || plainToken.ExpiresAt != nil {
		t.Fatalf("normalization/expiry failed: %#v", plainToken)
	}
}

func TestRequireTokenScopeAndTouch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, plain, _ := NewAPIToken("u1", "cli", []string{"read", "write"}, nil)
	token, _, _ := NewAPIToken("u1", "cli", []string{"read", "write"}, nil)
	store := &tokenStore{token: &token}
	r := gin.New()
	r.GET("/", RequireToken(store, "read", "write"), func(c *gin.Context) {
		if tok, ok := TokenFromContext(c); !ok || tok.UserID != "u1" || !Can(c, "read") {
			t.Error("context token missing")
		}
		c.Status(204)
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+plain)
	// Match the persisted hash to the presented secret.
	store.token.TokenHash = HashAPIToken(plain)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 || !store.touched {
		t.Fatalf("status=%d touched=%v", w.Code, store.touched)
	}
}
