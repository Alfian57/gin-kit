package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func protectedRouter(t *testing.T, manager *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/me", RequireAuth(manager), func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			t.Error("claims missing from authenticated context")
		}
		if claims.UserID != UserID(c) {
			t.Errorf("UserID mismatch: %q vs %q", claims.UserID, UserID(c))
		}
		c.String(http.StatusOK, UserID(c))
	})
	return router
}

func TestRequireAuthRejectsAndAccepts(t *testing.T) {
	secret := []byte(strings.Repeat("s", MinimumSecretLength))
	manager, err := New(secret, "orders", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := manager.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user-1", Issuer: "orders", IssuedAt: jwt.NewNumericDate(past),
			ExpiresAt: jwt.NewNumericDate(past.Add(time.Minute)),
		},
	}).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	otherIssuer, err := New(secret, "billing", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	wrongIssuer, err := otherIssuer.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	otherSecret, err := New([]byte(strings.Repeat("x", MinimumSecretLength)), "orders", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	wrongSecret, err := otherSecret.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}

	router := protectedRouter(t, manager)
	for _, test := range []struct {
		name          string
		authorization string
		status        int
		contains      string
	}{
		{"valid token", "Bearer " + valid, http.StatusOK, "user-1"},
		{"missing header", "", http.StatusUnauthorized, `"code":"missing_token"`},
		{"wrong scheme", "Basic dXNlcjpwYXNz", http.StatusUnauthorized, `"code":"missing_token"`},
		{"malformed token", "Bearer not-a-jwt", http.StatusUnauthorized, `"code":"invalid_token"`},
		{"expired token", "Bearer " + expired, http.StatusUnauthorized, `"code":"invalid_token"`},
		{"wrong issuer", "Bearer " + wrongIssuer, http.StatusUnauthorized, `"code":"invalid_token"`},
		{"wrong secret", "Bearer " + wrongSecret, http.StatusUnauthorized, `"code":"invalid_token"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.contains) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if test.status == http.StatusUnauthorized && recorder.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing WWW-Authenticate header on 401")
			}
		})
	}
}

func TestClaimsFromContextWithoutAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := ClaimsFromContext(c); ok {
		t.Fatal("expected no claims on unauthenticated context")
	}
	if UserID(c) != "" {
		t.Fatal("expected empty user ID on unauthenticated context")
	}
}

func TestRequireAuthPanicsOnNilManager(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil manager")
		}
	}()
	RequireAuth(nil)
}
