package framework

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimitOptions struct {
	Enabled           bool
	RequestsPerMinute int
	Burst             int
	Key               func(*gin.Context) string
}

type rateClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is an in-memory limiter that can be installed selectively on
// routes or route groups when different policies are needed.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rateClient
	rate    rate.Limit
	burst   int
	key     func(*gin.Context) string
}

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
