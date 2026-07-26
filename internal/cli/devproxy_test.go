package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDevProxyHoldsThenReleases(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello from backend")
	}))
	defer backend.Close()
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := newDevProxy(5 * time.Second)
	p.SetBuilding()

	type result struct {
		status int
		body   string
	}
	results := make(chan result, 1)
	go func() {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		results <- result{status: rec.Code, body: rec.Body.String()}
	}()

	select {
	case r := <-results:
		t.Fatalf("request returned during build: %d %q", r.status, r.body)
	case <-time.After(50 * time.Millisecond):
	}

	p.SetReady(target)

	select {
	case r := <-results:
		if r.status != http.StatusOK {
			t.Fatalf("released request status = %d, want %d", r.status, http.StatusOK)
		}
		if r.body != "hello from backend" {
			t.Fatalf("released request body = %q, want backend body", r.body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("held request was not released after SetReady")
	}
}

func TestDevProxyOverlayNegotiation(t *testing.T) {
	output := "main.go:3: undefined: x"
	p := newDevProxy(time.Second)
	p.SetFailed(output)

	t.Run("html client gets the overlay", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Build failed") {
			t.Fatalf("overlay body missing title: %q", body)
		}
		if !strings.Contains(body, output) {
			t.Fatalf("overlay body missing compiler output: %q", body)
		}
	})

	t.Run("plain client gets raw output", func(t *testing.T) {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
			t.Fatalf("Content-Type = %q, want text/plain", got)
		}
		if rec.Body.String() != output {
			t.Fatalf("body = %q, want raw output %q", rec.Body.String(), output)
		}
	})

	t.Run("overlay escapes markup in output", func(t *testing.T) {
		p.SetFailed(`main.go:9: cannot use "<b>bold</b>"`)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		body := rec.Body.String()
		if strings.Contains(body, "<b>bold</b>") {
			t.Fatalf("overlay body contains unescaped markup: %q", body)
		}
		if !strings.Contains(body, "&lt;b&gt;bold&lt;/b&gt;") {
			t.Fatalf("overlay body missing escaped markup: %q", body)
		}
	})
}

func TestDevProxyHoldTimeout(t *testing.T) {
	p := newDevProxy(30 * time.Millisecond)
	p.SetBuilding()

	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", got)
	}
	if !strings.Contains(rec.Body.String(), "gin-kit dev: rebuild timed out") {
		t.Fatalf("body = %q, want rebuild timeout message", rec.Body.String())
	}
}
