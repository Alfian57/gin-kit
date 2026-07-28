package devtools

import (
	"context"
	_ "embed"
	"html"
	"net/http"
	"sort"
	"strconv"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/Alfian57/gin-kit/runtime/queue"
	"github.com/gin-gonic/gin"
)

//go:embed dashboard.html
var dashboardHTML []byte

// dashboardCSP relaxes the strict default Content-Security-Policy for the
// dashboard page only, the same override mechanism the docs page uses: the
// single-file dashboard needs its inline script and styles, and frame-src
// 'self' allows the sandboxed mail preview iframe.
const dashboardCSP = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'"

// mailPreviewCSP confines rendered mail HTML: sandboxed, no scripts, no
// network fetches beyond images, so a hostile message cannot touch the
// application origin.
const mailPreviewCSP = "sandbox; default-src 'none'; img-src data: https:; style-src 'unsafe-inline'"

func (d *DevTools) dashboard(c *gin.Context) {
	c.Header("Content-Security-Policy", dashboardCSP)
	c.Data(http.StatusOK, "text/html; charset=utf-8", dashboardHTML)
}

func (d *DevTools) apiRequests(c *gin.Context) {
	httpx.OK(c, d.requests.Snapshot())
}

// apiMails lists the outbox without message bodies; the preview endpoint is
// the only place bodies leave the process.
func (d *DevTools) apiMails(c *gin.Context) {
	entries := d.outbox.Snapshot()
	for index := range entries {
		entries[index].Text = ""
		entries[index].HTML = ""
	}
	httpx.OK(c, entries)
}

// apiMailHTML serves one message body as a locked-down HTML document for
// the dashboard's sandboxed preview iframe. Messages without an HTML body
// fall back to the escaped text body in a <pre>.
func (d *DevTools) apiMailHTML(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusNotFound, "not_found", "The requested resource was not found."))
		return
	}
	entry, found := d.outbox.Find(id)
	if !found {
		httpx.Fail(c, httpx.NewError(http.StatusNotFound, "not_found", "The requested resource was not found."))
		return
	}
	c.Header("Content-Security-Policy", mailPreviewCSP)
	c.Header("X-Frame-Options", "SAMEORIGIN")
	body := entry.HTML
	if body == "" {
		body = "<pre>" + html.EscapeString(entry.Text) + "</pre>"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
}

// RouteEntry is one registered route in the devtools route list.
type RouteEntry struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

func (d *DevTools) apiRoutes(routes func() gin.RoutesInfo) gin.HandlerFunc {
	return func(c *gin.Context) {
		list := make([]RouteEntry, 0)
		if routes != nil {
			for _, route := range routes() {
				list = append(list, RouteEntry{Method: route.Method, Path: route.Path, Handler: route.Handler})
			}
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Path != list[j].Path {
				return list[i].Path < list[j].Path
			}
			return list[i].Method < list[j].Method
		})
		httpx.OK(c, list)
	}
}

func (d *DevTools) apiConfig(c *gin.Context) {
	httpx.OK(c, configReport())
}

func (d *DevTools) apiQueue(queueStats func(context.Context) (queue.Stats, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if queueStats == nil {
			httpx.OK(c, queue.Stats{})
			return
		}
		stats, err := queueStats(c.Request.Context())
		if err != nil {
			d.logger.ErrorContext(c.Request.Context(), "devtools: queue stats failed", "error", err)
			httpx.Handle(c, d.mapper, err)
			return
		}
		httpx.OK(c, stats)
	}
}
