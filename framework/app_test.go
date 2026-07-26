package framework

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/gin-gonic/gin"
)

func TestNewInstallsHealthSecurityAndRequestIDDefaults(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/hello", func(c *gin.Context) {
		httpx.OK(c, gin.H{"hello": "world"})
	})
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("unexpected response: %d headers=%v", recorder.Code, recorder.Header())
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers not installed")
	}
}

func TestRecoveryUsesStableInternalError(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/panic", func(*gin.Context) { panic("secret panic") })
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "secret panic") {
		t.Fatalf("panic leaked or wrong status: %d %s", recorder.Code, recorder.Body)
	}
}

func TestNotFoundAndMethodNotAllowedUseCanonicalEnvelope(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/known", func(c *gin.Context) { httpx.OK(c, "ok") })
	for _, test := range []struct {
		method string
		path   string
		status int
		code   string
	}{
		{http.MethodGet, "/missing", http.StatusNotFound, "not_found"},
		{http.MethodPost, "/known", http.StatusMethodNotAllowed, "method_not_allowed"},
	} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("%s %s: %d %s", test.method, test.path, recorder.Code, recorder.Body)
		}
	}
}

func TestReadinessAndShutdownHooks(t *testing.T) {
	app, err := New(Options{Readiness: map[string]Check{
		"database": func(context.Context) error { return errors.New("offline") },
	}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d", recorder.Code)
	}
	var called bool
	app.OnShutdown(func(context.Context) error {
		called = true
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("shutdown hook was not called")
	}
}

func TestRateLimitRejectsExcessRequests(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{
		RateLimit: RateLimitOptions{Enabled: true, RequestsPerMinute: 1, Burst: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/limited", func(c *gin.Context) { httpx.OK(c, "ok") })
	first := httptest.NewRecorder()
	app.Router().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/limited", nil))
	second := httptest.NewRecorder()
	app.Router().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/limited", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected statuses: %d, %d", first.Code, second.Code)
	}
}

func TestTrustedProxiesControlClientIP(t *testing.T) {
	for _, test := range []struct {
		name      string
		proxies   []string
		remote    string
		forwarded string
		want      string
	}{
		{"default distrusts forwarded headers", nil, "203.0.113.7:1234", "1.2.3.4", "203.0.113.7"},
		{"trusted proxy honored", []string{"10.0.0.0/8"}, "10.1.2.3:1234", "1.2.3.4", "1.2.3.4"},
		{"untrusted peer ignored", []string{"10.0.0.0/8"}, "203.0.113.7:1234", "1.2.3.4", "203.0.113.7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, err := New(Options{HTTP: HTTPOptions{TrustedProxies: test.proxies}})
			if err != nil {
				t.Fatal(err)
			}
			app.Router().GET("/ip", func(c *gin.Context) {
				httpx.OK(c, gin.H{"ip": c.ClientIP()})
			})
			request := httptest.NewRequest(http.MethodGet, "/ip", nil)
			request.RemoteAddr = test.remote
			request.Header.Set("X-Forwarded-For", test.forwarded)
			recorder := httptest.NewRecorder()
			app.Router().ServeHTTP(recorder, request)
			if !strings.Contains(recorder.Body.String(), `"ip":"`+test.want+`"`) {
				t.Fatalf("client IP: %s", recorder.Body.String())
			}
		})
	}
}

func TestNewRejectsInvalidTrustedProxy(t *testing.T) {
	if _, err := New(Options{HTTP: HTTPOptions{TrustedProxies: []string{"not-a-proxy"}}}); err == nil {
		t.Fatal("expected error for invalid trusted proxy")
	}
}

func TestMetricsEndpointExposesRoutePatterns(t *testing.T) {
	app, err := New(Options{Metrics: MetricsOptions{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/tasks/:id", func(c *gin.Context) { httpx.OK(c, "ok") })
	for _, path := range []string{"/tasks/123", "/tasks/456", "/absent"} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	}
	scrape := httptest.NewRecorder()
	app.Router().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if scrape.Code != http.StatusOK {
		t.Fatalf("scrape status = %d", scrape.Code)
	}
	body := scrape.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/tasks/:id",status="200"} 2`) {
		t.Fatalf("route pattern label missing:\n%s", body)
	}
	if !strings.Contains(body, `route="unmatched"`) || !strings.Contains(body, "http_request_duration_seconds") {
		t.Fatalf("unmatched label or duration histogram missing:\n%s", body)
	}
	if app.Metrics() == nil || app.Metrics().Registry() == nil {
		t.Fatal("metrics accessor not wired")
	}
}

func TestMetricsAndPProfDisabledByDefault(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/metrics", "/debug/pprof/"} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"not_found"`) {
			t.Fatalf("%s: %d %s", path, recorder.Code, recorder.Body)
		}
	}
	if app.Metrics() != nil {
		t.Fatal("metrics should be nil when disabled")
	}
}

func TestPProfMountsWhenEnabled(t *testing.T) {
	app, err := New(Options{PProf: PProfOptions{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "profiles") {
		t.Fatalf("pprof index: %d", recorder.Code)
	}
}

func TestRequestScopedLoggerCarriesRequestID(t *testing.T) {
	var buffer strings.Builder
	app, err := New(Options{Logger: slog.New(slog.NewJSONHandler(&buffer, nil))})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/log", func(c *gin.Context) {
		httpx.Logger(c).Info("handled")
		httpx.OK(c, "ok")
	})
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/log", nil))
	requestID := recorder.Header().Get("X-Request-ID")
	if requestID == "" || !strings.Contains(buffer.String(), `"request_id":"`+requestID+`"`) {
		t.Fatalf("request-scoped logger missing request ID %q:\n%s", requestID, buffer.String())
	}
	if !strings.Contains(buffer.String(), `"path":"/log"`) {
		t.Fatalf("request-scoped logger missing path:\n%s", buffer.String())
	}
}

func TestRunReturnsServerError(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "bad address", ShutdownTimeout: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected listen error")
	}
}
