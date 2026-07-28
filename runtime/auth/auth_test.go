package auth

import (
	"strings"
	"testing"
	"time"
)

func TestManagerIssueParseAndExpiry(t *testing.T) {
	manager, err := New([]byte(strings.Repeat("s", 32)), "orders", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(token)
	if err != nil || claims.UserID != "user-1" || claims.Issuer != "orders" {
		t.Fatalf("unexpected claims: %#v, %v", claims, err)
	}
	if _, err := New([]byte("short"), "", time.Minute); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestRefreshTokensAreHashed(t *testing.T) {
	plain, hashed, err := NewRefreshToken()
	if err != nil || plain == hashed || HashRefreshToken(plain) != hashed {
		t.Fatalf("invalid refresh token: %q %q %v", plain, hashed, err)
	}
}
