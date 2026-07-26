package framework

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if len(id) == 0 || len(id) > 128 || strings.ContainsAny(id, "\r\n") {
			raw := make([]byte, 16)
			if _, err := rand.Read(raw); err != nil {
				id = "unavailable"
			} else {
				id = hex.EncodeToString(raw)
			}
		}
		c.Set(httpx.RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.InfoContext(c.Request.Context(), "request completed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", c.GetString(httpx.RequestIDKey),
		)
	}
}

func recovery(logger *slog.Logger, mapper httpx.Mapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := errors.New("panic recovered")
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					"panic", recovered,
					"stack", string(debug.Stack()),
					"request_id", c.GetString(httpx.RequestIDKey),
				)
				if !c.Writer.Written() {
					httpx.Handle(c, mapper, err)
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

func errorHandler(logger *slog.Logger, mapper httpx.Mapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		mapped := mapper(err, c)
		if mapped.Status >= http.StatusInternalServerError {
			logger.ErrorContext(c.Request.Context(), "request failed",
				"error", err,
				"request_id", c.GetString(httpx.RequestIDKey),
			)
		}
		httpx.Fail(c, mapped)
	}
}

func securityHeaders(ui bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if ui {
			c.Header("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'")
		}
		c.Next()
	}
}

func bodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			if origin != "" {
				if _, ok := allowed[origin]; !ok {
					httpx.Fail(c, httpx.NewError(http.StatusForbidden, "origin_not_allowed", "The request origin is not allowed."))
					return
				}
			}
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-CSRF-Token")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
