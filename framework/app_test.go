package framework

import (
	"context"
	"errors"
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

func TestRunReturnsServerError(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "bad address", ShutdownTimeout: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected listen error")
	}
}
