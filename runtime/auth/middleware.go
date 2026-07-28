package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/Alfian57/gin-kit/runtime/session"
	"github.com/gin-gonic/gin"
)

const (
	// claimsKey define package-level implementation state.
	claimsKey = "gin-kit.auth.claims"
	// userIDKey define package-level implementation state.
	userIDKey = "user_id"
)

// RequireAuth authenticates requests with a Bearer token, stores the verified
// claims in the request context, and rejects unauthenticated requests with a
// canonical 401 envelope. It panics when manager is nil so a wiring mistake
// fails at route registration, not per request.
func RequireAuth(manager *Manager) gin.HandlerFunc {
	return require(manager, false)
}

// RequireLogin authenticates requests with an OAuth browser session when one
// is present, otherwise it requires the same Bearer token accepted by
// RequireAuth. Install SessionMiddleware after the session middleware.
func RequireLogin(manager *Manager) gin.HandlerFunc {
	return require(manager, true)
}

// require performs this package operation.
func require(manager *Manager, allowSession bool) gin.HandlerFunc {
	if manager == nil {
		panic("auth: authentication middleware requires a non-nil manager")
	}
	return func(c *gin.Context) {
		if allowSession && c.GetString(userIDKey) != "" {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.Header("WWW-Authenticate", "Bearer")
			httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "missing_token", "Bearer token required."))
			return
		}
		claims, err := manager.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.Header("WWW-Authenticate", "Bearer")
			httpx.Fail(c, httpx.WrapError(http.StatusUnauthorized, "invalid_token", "Token is invalid or expired.", err))
			return
		}
		c.Set(claimsKey, claims)
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

// SessionMiddleware restores an OAuth-authenticated user ID from the encrypted
// session cookie. It must run after session.Middleware and before RequireLogin.
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID, _ := session.Get(c, sessionUserKey).(string); userID != "" {
			c.Set(userIDKey, userID)
		}
		c.Next()
	}
}

// LoginSession records the authenticated user in the encrypted session.
func LoginSession(c *gin.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("auth: session user ID is required")
	}
	return session.Set(c, sessionUserKey, userID)
}

// LogoutSession removes the authenticated user from the encrypted session.
func LogoutSession(c *gin.Context) error { return session.Forget(c, sessionUserKey) }

// ClaimsFromContext returns the claims stored by RequireAuth.
func ClaimsFromContext(c *gin.Context) (*Claims, bool) {
	value, exists := c.Get(claimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*Claims)
	return claims, ok && claims != nil
}

// UserID returns the authenticated user ID, or "" when unauthenticated.
func UserID(c *gin.Context) string {
	if userID := c.GetString(userIDKey); userID != "" {
		return userID
	}
	claims, ok := ClaimsFromContext(c)
	if !ok {
		return ""
	}
	return claims.UserID
}
