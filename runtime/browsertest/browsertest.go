// Package browsertest provides Playwright helpers for end-to-end browser
// tests against a gin-kit application. Tests skip cleanly when the
// Playwright driver or browsers are not installed, so `go test ./...` stays
// green on machines and CI runners without them.
//
// One-time local setup:
//
//	go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium
package browsertest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Alfian57/gin-kit/runtime"
	"github.com/playwright-community/playwright-go"
)

// Browser bundles a running Playwright driver and a Chromium instance.
type Browser struct {
	// PW store data used by this type.
	PW *playwright.Playwright
	// Chromium store data used by this type.
	Chromium playwright.Browser
}

// Launch starts Playwright and Chromium, or skips the test: when
// PLAYWRIGHT_SKIP is set, or when the driver/browsers are not installed.
// Everything stops via t.Cleanup.
func Launch(t *testing.T) *Browser {
	t.Helper()
	if os.Getenv("PLAYWRIGHT_SKIP") != "" {
		t.Skip("browsertest: PLAYWRIGHT_SKIP is set")
	}
	pw, err := playwright.Run()
	if err != nil {
		t.Skipf("browsertest: playwright driver unavailable (run browsertest.Install once): %v", err)
	}
	chromium, err := pw.Chromium.Launch()
	if err != nil {
		pw.Stop()
		t.Skipf("browsertest: chromium unavailable (run browsertest.Install once): %v", err)
	}
	t.Cleanup(func() {
		chromium.Close()
		pw.Stop()
	})
	return &Browser{PW: pw, Chromium: chromium}
}

// NewPage opens a page that closes with the test.
func (b *Browser) NewPage(t *testing.T) playwright.Page {
	t.Helper()
	page, err := b.Chromium.NewPage()
	if err != nil {
		t.Fatalf("browsertest: new page: %v", err)
	}
	t.Cleanup(func() { page.Close() })
	return page
}

// StartServer serves the application's router on 127.0.0.1 with a random
// port and returns the base URL (e.g. http://127.0.0.1:41234). It uses a
// plain HTTP server — not Application.Run — so there is no signal handling
// or fixed address; shutdown and application cleanup happen via t.Cleanup.
func StartServer(t *testing.T, application *runtime.Application) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("browsertest: listen: %v", err)
	}
	server := &http.Server{Handler: application.Router(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("browsertest: serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = application.Close(ctx)
	})
	return "http://" + listener.Addr().String()
}

// Install downloads the Playwright driver and Chromium. Intended for
// one-time local setup or provisioning scripts.
func Install() error {
	return playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}})
}
