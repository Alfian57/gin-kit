package framework

import (
	"log/slog"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

// contextLogger stores a request-scoped logger carrying the request ID so
// handlers can retrieve it with httpx.Logger.
func contextLogger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(httpx.LoggerKey, base.With(
			"request_id", c.GetString(httpx.RequestIDKey),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		))
		c.Next()
	}
}
