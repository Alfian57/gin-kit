package runtime

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitOptions defines an implementation type used by this package.
type RateLimitOptions struct {
	// Enabled store data used by this type.
	Enabled bool
	// RequestsPerMinute store data used by this type.
	RequestsPerMinute int
	// Burst store data used by this type.
	Burst int
	// Key store data used by this type.
	Key func(*gin.Context) string
}

// rateClient defines an implementation type used by this package.
type rateClient struct {
	// limiter store data used by this type.
	limiter *rate.Limiter
	// lastSeen store data used by this type.
	lastSeen time.Time
}

// RateLimiter is an in-memory limiter that can be installed selectively on
// routes or route groups when different policies are needed.
type RateLimiter struct {
	// mu store data used by this type.
	mu sync.Mutex
	// clients store data used by this type.
	clients map[string]*rateClient
	// rate store data used by this type.
	rate rate.Limit
	// burst store data used by this type.
	burst int
	// key store data used by this type.
	key func(*gin.Context) string
}

// NewRateLimiter performs this package operation.
func NewRateLimiter(options RateLimitOptions) *RateLimiter {
	burst := options.Burst
	if burst < 1 {
		burst = max(1, options.RequestsPerMinute/4)
	}
	key := options.Key
	if key == nil {
		key = func(c *gin.Context) string { return c.ClientIP() }
	}
	return &RateLimiter{
		clients: make(map[string]*rateClient),
		rate:    rate.Limit(float64(options.RequestsPerMinute) / 60),
		burst:   burst,
		key:     key,
	}
}

// Middleware performs this package operation.
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := l.key(c)
		now := time.Now()
		l.mu.Lock()
		entry := l.clients[key]
		if entry == nil {
			entry = &rateClient{limiter: rate.NewLimiter(l.rate, l.burst)}
			l.clients[key] = entry
		}
		entry.lastSeen = now
		allowed := entry.limiter.Allow()
		if len(l.clients) > 10_000 {
			for candidate, client := range l.clients {
				if now.Sub(client.lastSeen) > 10*time.Minute {
					delete(l.clients, candidate)
				}
			}
		}
		l.mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			c.Header("X-RateLimit-Limit", fmt.Sprint(int(float64(l.rate)*60)))
			httpx.Fail(c, httpx.NewError(http.StatusTooManyRequests, "rate_limited", "Too many requests."))
			return
		}
		c.Next()
	}
}
