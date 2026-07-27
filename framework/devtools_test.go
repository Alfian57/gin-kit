package framework

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewRefusesDevToolsOutsideDevelopment(t *testing.T) {
	for _, environment := range []string{"production", "staging"} {
		t.Run(environment, func(t *testing.T) {
			_, err := New(Options{
				Environment: environment,
				DevTools:    DevToolsOptions{Enabled: true},
			})
			if err == nil || !strings.Contains(err.Error(), "devtools") {
				t.Fatalf("devtools accepted in %s: %v", environment, err)
			}
		})
	}
	if _, err := New(Options{DevTools: DevToolsOptions{Enabled: true, Path: "_ginkit"}}); err == nil {
		t.Fatal("relative devtools path accepted")
	}
}

func TestDevToolsDisabledByDefault(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if app.DevTools() != nil {
		t.Fatal("DevTools() should be nil when disabled")
	}
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_ginkit", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("/_ginkit should be absent when devtools are disabled: %d", recorder.Code)
	}
}

func TestDevToolsRecordsRequestsWithoutRecordingItself(t *testing.T) {
	app, err := New(Options{DevTools: DevToolsOptions{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	if app.DevTools() == nil {
		t.Fatal("DevTools() should not be nil when enabled")
	}
	app.Router().GET("/boom", func(c *gin.Context) {
		c.Error(errors.New("kaput")) // surfaces through the error handler
	})

	live := httptest.NewRecorder()
	app.Router().ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("/health/live status = %d", live.Code)
	}
	app.Router().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/_ginkit/api/requests", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/_ginkit/api/requests status = %d", recorder.Code)
	}
	var payload struct {
		Data []struct {
			Path      string `json:"path"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
			ErrorCode string `json:"error_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("expected the two application requests only, got %d: %s", len(payload.Data), recorder.Body.String())
	}
	// Newest first: /boom then /health/live.
	if payload.Data[0].Path != "/boom" || payload.Data[0].Status != http.StatusInternalServerError {
		t.Fatalf("failed request not recorded: %+v", payload.Data[0])
	}
	if payload.Data[0].ErrorCode != "internal_error" {
		t.Fatalf("mapped error code not recorded: %+v", payload.Data[0])
	}
	if payload.Data[1].Path != "/health/live" || payload.Data[1].Status != http.StatusOK || payload.Data[1].RequestID == "" {
		t.Fatalf("/health/live not recorded with a request ID: %+v", payload.Data[1])
	}

	again := httptest.NewRecorder()
	app.Router().ServeHTTP(again, httptest.NewRequest(http.MethodGet, "/_ginkit/api/requests", nil))
	if err := json.Unmarshal(again.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("devtools requests recorded themselves: %d entries", len(payload.Data))
	}
}

func TestDevToolsDashboardAndSpecExclusion(t *testing.T) {
	app, err := New(Options{
		Docs:     DocsOptions{Enabled: true},
		DevTools: DevToolsOptions{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	app.Router().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/_ginkit", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "gin-kit") {
		t.Fatalf("dashboard not served: %d", page.Code)
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("dashboard CSP not overridden: %q", csp)
	}

	spec := httptest.NewRecorder()
	app.Router().ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if spec.Code != http.StatusOK {
		t.Fatalf("spec status = %d", spec.Code)
	}
	if strings.Contains(spec.Body.String(), "/_ginkit") {
		t.Fatalf("OpenAPI spec documents the devtools routes: %s", spec.Body.String())
	}
}
