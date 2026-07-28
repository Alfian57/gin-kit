package devtools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/mail"
	"github.com/Alfian57/gin-kit/runtime/queue"
	"github.com/gin-gonic/gin"
)

func newTestRouter(t *testing.T, d *DevTools) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(d.Middleware())
	d.Mount(router, router.Routes, func(context.Context) (queue.Stats, error) {
		return queue.Stats{Driver: "sync"}, nil
	})
	return router
}

func getJSON(t *testing.T, router *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("%s did not return JSON: %v (%s)", path, err, recorder.Body.String())
	}
	return recorder, payload
}

func TestDashboardServedWithRelaxedCSP(t *testing.T) {
	router := newTestRouter(t, New(Options{}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_ginkit", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "gin-kit") {
		t.Fatalf("dashboard not served: %d", recorder.Code)
	}
	csp := recorder.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'unsafe-inline'") || !strings.Contains(csp, "frame-src 'self'") {
		t.Fatalf("dashboard CSP wrong: %q", csp)
	}
}

func TestAPIEndpointsAnswerInTheEnvelope(t *testing.T) {
	d := New(Options{})
	router := newTestRouter(t, d)
	for _, path := range []string{
		"/_ginkit/api/requests",
		"/_ginkit/api/mails",
		"/_ginkit/api/routes",
		"/_ginkit/api/config",
		"/_ginkit/api/queue",
	} {
		recorder, payload := getJSON(t, router, path)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, recorder.Code)
		}
		if _, ok := payload["data"]; !ok {
			t.Fatalf("%s missing the data envelope: %s", path, recorder.Body.String())
		}
	}
	_, payload := getJSON(t, router, "/_ginkit/api/queue")
	stats := payload["data"].(map[string]any)
	if stats["driver"] != "sync" {
		t.Fatalf("queue stats not surfaced: %v", payload)
	}
}

func TestMiddlewareRecordsRequestsButSkipsItself(t *testing.T) {
	d := New(Options{})
	router := newTestRouter(t, d)
	router.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/hello?secret=1", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/_ginkit/api/requests", nil))

	entries := d.requests.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("expected exactly the application request, got %d entries", len(entries))
	}
	if entries[0].Path != "/hello" || entries[0].Status != http.StatusOK {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if strings.Contains(entries[0].Path, "secret") {
		t.Fatalf("query string recorded: %+v", entries[0])
	}
}

func TestMailPreviewHeadersAndFallback(t *testing.T) {
	d := New(Options{})
	router := newTestRouter(t, d)
	mailer := d.WrapMailer(stubMailer{})

	htmlMessage := mail.NewMessage().To("a@example.com").Subject("HTML").HTML("<h1>Hi there</h1>")
	textMessage := mail.NewMessage().To("a@example.com").Subject("Text").Text("plain <body>")
	if err := mailer.Send(context.Background(), htmlMessage); err != nil {
		t.Fatal(err)
	}
	if err := mailer.Send(context.Background(), textMessage); err != nil {
		t.Fatal(err)
	}

	htmlPreview := httptest.NewRecorder()
	router.ServeHTTP(htmlPreview, httptest.NewRequest(http.MethodGet, "/_ginkit/api/mails/1/html", nil))
	if htmlPreview.Code != http.StatusOK || !strings.Contains(htmlPreview.Body.String(), "<h1>Hi there</h1>") {
		t.Fatalf("html preview: %d %s", htmlPreview.Code, htmlPreview.Body.String())
	}
	if csp := htmlPreview.Header().Get("Content-Security-Policy"); csp != mailPreviewCSP {
		t.Fatalf("preview CSP = %q", csp)
	}
	if xfo := htmlPreview.Header().Get("X-Frame-Options"); xfo != "SAMEORIGIN" {
		t.Fatalf("preview X-Frame-Options = %q", xfo)
	}
	if contentType := htmlPreview.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("preview content type = %q", contentType)
	}

	textPreview := httptest.NewRecorder()
	router.ServeHTTP(textPreview, httptest.NewRequest(http.MethodGet, "/_ginkit/api/mails/2/html", nil))
	if !strings.Contains(textPreview.Body.String(), "<pre>plain &lt;body&gt;</pre>") {
		t.Fatalf("text fallback not escaped into <pre>: %s", textPreview.Body.String())
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/_ginkit/api/mails/99/html", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing mail preview status = %d", missing.Code)
	}

	list, payload := getJSON(t, router, "/_ginkit/api/mails")
	if strings.Contains(list.Body.String(), "Hi there") || strings.Contains(list.Body.String(), "plain") {
		t.Fatalf("mail list leaks bodies: %s", list.Body.String())
	}
	if items := payload["data"].([]any); len(items) != 2 {
		t.Fatalf("mail list length = %d", len(items))
	}
}
