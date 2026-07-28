// Package devtools serves gin-kit's development dashboard: a request log,
// mail outbox, route list, redacted config report, and queue statistics
// behind a single mount point. It is strictly a development tool — the
// runtime refuses to enable it outside the development environment — and
// its request log deliberately stores no bodies, no query strings, and no
// headers beyond the user agent.
package devtools

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/Alfian57/gin-kit/runtime/mail"
	"github.com/Alfian57/gin-kit/runtime/queue"
	"github.com/gin-gonic/gin"
)

// Options defines an implementation type used by this package.
type Options struct {
	// Path is the dashboard mount point, defaulting to /_ginkit.
	Path string
	// Logger defaults to slog.Default().
	Logger *slog.Logger
	// Mapper derives the public error code recorded for failed requests,
	// exactly like the runtime error handler does. It defaults to
	// httpx.DefaultMapper.
	Mapper httpx.Mapper
	// MaxEntries caps the request log, defaulting to 200.
	MaxEntries int
	// MaxMails caps the mail outbox, defaulting to 50.
	MaxMails int
}

// DevTools records requests and mail and serves the dashboard over them.
type DevTools struct {
	// path store data used by this type.
	path string
	// logger store data used by this type.
	logger *slog.Logger
	// mapper store data used by this type.
	mapper httpx.Mapper
	// requests store data used by this type.
	requests *requestRing
	// outbox store data used by this type.
	outbox *mailRing
}

// New performs this package operation.
func New(options Options) *DevTools {
	options.Path = strings.TrimRight(options.Path, "/")
	if options.Path == "" {
		options.Path = "/_ginkit"
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Mapper == nil {
		options.Mapper = httpx.DefaultMapper
	}
	if options.MaxEntries < 1 {
		options.MaxEntries = 200
	}
	if options.MaxMails < 1 {
		options.MaxMails = 50
	}
	return &DevTools{
		path:     options.Path,
		logger:   options.Logger,
		mapper:   options.Mapper,
		requests: newRequestRing(options.MaxEntries),
		outbox:   newMailRing(options.MaxMails),
	}
}

// Middleware records completed requests into the devtools log. Requests to
// the dashboard itself are skipped so polling does not flood the log. Only
// the URL path is stored — never the query string, bodies, or headers other
// than the user agent.
func (d *DevTools) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == d.path || strings.HasPrefix(path, d.path+"/") {
			c.Next()
			return
		}
		started := time.Now()
		c.Next()
		entry := RequestEntry{
			Time:       started,
			Method:     c.Request.Method,
			Path:       path,
			Status:     c.Writer.Status(),
			DurationMS: time.Since(started).Milliseconds(),
			RequestID:  c.GetString(httpx.RequestIDKey),
			ClientIP:   c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		}
		if len(c.Errors) > 0 {
			if mapped := d.mapper(c.Errors.Last().Err, c); mapped != nil {
				entry.ErrorCode = mapped.Code
			}
		}
		d.requests.Add(entry)
	}
}

// Mount registers the dashboard page and its JSON API on router under the
// configured path. routes and queueStats supply live data at request time;
// either may be nil, which renders an empty tab.
func (d *DevTools) Mount(router gin.IRouter, routes func() gin.RoutesInfo, queueStats func(context.Context) (queue.Stats, error)) {
	group := router.Group(d.path)
	group.GET("", d.dashboard)
	group.GET("/api/requests", d.apiRequests)
	group.GET("/api/mails", d.apiMails)
	group.GET("/api/mails/:id/html", d.apiMailHTML)
	group.GET("/api/routes", d.apiRoutes(routes))
	group.GET("/api/config", d.apiConfig)
	group.GET("/api/queue", d.apiQueue(queueStats))
}

// WrapMailer decorates m so every sent or failed message is captured in the
// devtools mail outbox. The underlying mailer's behavior is unchanged.
func (d *DevTools) WrapMailer(m mail.Mailer) mail.Mailer {
	return &recordingMailer{next: m, outbox: d.outbox}
}
