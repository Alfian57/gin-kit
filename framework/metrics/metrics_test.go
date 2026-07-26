package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestNewUsesProvidedRegistryAndNamespace(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, err := New(Options{Registry: registry, Namespace: "myapp"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Registry() != registry {
		t.Fatal("custom registry was not used")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(m.Middleware())
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(scrape.Body.String(), `myapp_http_requests_total{method="GET",route="/ping",status="200"} 1`) {
		t.Fatalf("namespaced counter missing:\n%s", scrape.Body.String())
	}
}

func TestNewRejectsDuplicateRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()
	if _, err := New(Options{Registry: registry}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{Registry: registry}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}
