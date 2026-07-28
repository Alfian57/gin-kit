package auth

import (
	"net/http"
	"strings"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
)

const (
	claimsKey = "gin-kit.auth.claims"
	userIDKey = "user_id"
)

// RequireAuth authenticates requests with a Bearer token, stores the verified
// claims in the request context, and rejects unauthenticated requests with a
// canonical 401 envelope. It panics when manager is nil so a wiring mistake
// fails at route registration, not per request.
func RequireAuth(manager *Manager) gin.HandlerFunc {
	if manager == nil {
		panic("auth: RequireAuth requires a non-nil manager")
	}
	return func(c *gin.Context) {
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
	claims, ok := ClaimsFromContext(c)
	if !ok {
		return ""
	}
	return claims.UserID
}
