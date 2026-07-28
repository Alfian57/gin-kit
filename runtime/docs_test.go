package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/Alfian57/gin-kit/runtime/openapi"
	"github.com/gin-gonic/gin"
)

func TestDocsDisabledByDefault(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/docs", "/openapi.json"} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s should be absent when docs are disabled: %d", path, recorder.Code)
		}
	}
}

func TestDocsServeLazyBuiltSpec(t *testing.T) {
	app, err := New(Options{Docs: DocsOptions{Enabled: true, Title: "Orders", Version: "2.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	// Routes registered after New (the normal boot flow) must be captured.
	app.Router().GET("/api/v1/orders/:id", func(c *gin.Context) { httpx.OK(c, "ok") })
	app.OpenAPI().Describe(openapi.Operation{
		Method: http.MethodGet, Path: "/api/v1/orders/:id",
		Summary: "Get an order", Security: true,
	})

	spec := httptest.NewRecorder()
	app.Router().ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if spec.Code != http.StatusOK {
		t.Fatalf("spec status = %d", spec.Code)
	}
	var document map[string]any
	if err := json.Unmarshal(spec.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	info := document["info"].(map[string]any)
	if info["title"] != "Orders" || info["version"] != "2.0.0" {
		t.Fatalf("info wrong: %v", info)
	}
	body := spec.Body.String()
	for _, expected := range []string{`"/api/v1/orders/{id}"`, `"/health/live"`, `"/health/ready"`, "bearerAuth", "Get an order"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("spec missing %s", expected)
		}
	}
	for _, excluded := range []string{`"/docs"`, `"/openapi.json"`} {
		if strings.Contains(body, excluded+":") {
			t.Fatalf("spec documents itself: %s", excluded)
		}
	}

	page := httptest.NewRecorder()
	app.Router().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "swagger-ui") {
		t.Fatalf("docs page: %d", page.Code)
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "unpkg.com") {
		t.Fatalf("docs CSP does not allow the CDN: %q", csp)
	}
}

func TestDocsBasicAuthProtection(t *testing.T) {
	app, err := New(Options{Docs: DocsOptions{
		Enabled: true, BasicAuthUsername: "admin", BasicAuthPassword: "secret",
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/docs", "/openapi.json"} {
		anonymous := httptest.NewRecorder()
		app.Router().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, path, nil))
		if anonymous.Code != http.StatusUnauthorized {
			t.Fatalf("%s without credentials: %d", path, anonymous.Code)
		}
		authorized := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.SetBasicAuth("admin", "secret")
		app.Router().ServeHTTP(authorized, request)
		if authorized.Code != http.StatusOK {
			t.Fatalf("%s with credentials: %d", path, authorized.Code)
		}
	}
}

func TestDocsSpecExcludesMetricsAndPProf(t *testing.T) {
	app, err := New(Options{
		Docs:    DocsOptions{Enabled: true},
		Metrics: MetricsOptions{Enabled: true},
		PProf:   PProfOptions{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	spec := httptest.NewRecorder()
	app.Router().ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	body := spec.Body.String()
	for _, excluded := range []string{`"/metrics"`, "/debug/pprof"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("spec includes infrastructure route %s", excluded)
		}
	}
}

func TestNewRejectsInvalidDocsConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		options DocsOptions
	}{
		{"half basic auth", DocsOptions{Enabled: true, BasicAuthUsername: "admin"}},
		{"same paths", DocsOptions{Enabled: true, Path: "/docs", SpecPath: "/docs"}},
		{"relative path", DocsOptions{Enabled: true, Path: "docs"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(Options{Docs: test.options}); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
}
