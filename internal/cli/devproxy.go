package cli

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// devState defines an implementation type used by this package.
type devState int

const (
	// devBuilding define package-level implementation state.
	devBuilding devState = iota
	// devReady define package-level implementation state.
	devReady
	// devFailed define package-level implementation state.
	devFailed
)

// devProxy is the HTTP front door of gin-kit dev. It reverse-proxies to the
// application when a build is ready, holds requests while a rebuild is in
// flight, and answers with a compile-error overlay after a failed build.
type devProxy struct {
	// mu store data used by this type.
	mu sync.Mutex
	// state store data used by this type.
	state devState
	// target store data used by this type.
	target *url.URL
	// proxy store data used by this type.
	proxy *httputil.ReverseProxy
	// failOutput store data used by this type.
	failOutput string
	// changed store data used by this type.
	changed chan struct{}
	// holdTimeout store data used by this type.
	holdTimeout time.Duration
}

// newDevProxy performs this package operation.
func newDevProxy(holdTimeout time.Duration) *devProxy {
	return &devProxy{state: devBuilding, changed: make(chan struct{}), holdTimeout: holdTimeout}
}

// setState transitions the proxy and wakes every held request by closing and
// replacing the change channel.
func (p *devProxy) setState(state devState, target *url.URL, output string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = state
	if target != nil && (p.target == nil || p.target.String() != target.String()) {
		p.target = target
		p.proxy = httputil.NewSingleHostReverseProxy(target)
	}
	p.failOutput = output
	close(p.changed)
	p.changed = make(chan struct{})
}

// SetBuilding holds incoming requests until the next state change.
func (p *devProxy) SetBuilding() { p.setState(devBuilding, nil, "") }

// SetReady routes requests to target.
func (p *devProxy) SetReady(target *url.URL) { p.setState(devReady, target, "") }

// SetFailed answers requests with the build output until a build succeeds.
func (p *devProxy) SetFailed(output string) { p.setState(devFailed, nil, output) }

// ServeHTTP performs this package operation.
func (p *devProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	deadline := time.Now().Add(p.holdTimeout)
	for {
		p.mu.Lock()
		state, reverse, output, changed := p.state, p.proxy, p.failOutput, p.changed
		p.mu.Unlock()
		switch state {
		case devReady:
			if reverse == nil {
				http.Error(w, "gin-kit dev: no upstream", http.StatusBadGateway)
				return
			}
			reverse.ServeHTTP(w, r)
			return
		case devFailed:
			writeDevFailure(w, r, output)
			return
		default: // devBuilding: hold the request until the state changes.
			wait := time.Until(deadline)
			if wait <= 0 {
				writeDevHoldTimeout(w)
				return
			}
			timer := time.NewTimer(wait)
			select {
			case <-changed:
				timer.Stop()
			case <-timer.C:
				writeDevHoldTimeout(w)
				return
			case <-r.Context().Done():
				timer.Stop()
				return
			}
		}
	}
}

// writeDevHoldTimeout performs this package operation.
func writeDevHoldTimeout(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintln(w, "gin-kit dev: rebuild timed out")
}

// writeDevFailure performs this package operation.
func writeDevFailure(w http.ResponseWriter, r *http.Request, output string) {
	w.Header().Set("Cache-Control", "no-store")
	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(overlayHTML(output))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(output))
}

// wantsHTML reports whether the client negotiates an HTML response.
func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// devOverlayTemplate define package-level implementation state.
var devOverlayTemplate = template.Must(template.New("overlay").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="2">
<title>Build failed</title>
<style>
  body { margin: 0; background: #121713; color: #e8ede8; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  header { padding: 1.25rem 1.5rem; border-bottom: 1px solid #2a332b; }
  h1 { margin: 0; font-size: 1.1rem; color: #B9F53D; }
  p { margin: 0.4rem 0 0; color: #9aa79b; font-size: 0.85rem; }
  pre { margin: 0; padding: 1.5rem; white-space: pre-wrap; word-break: break-word; font-size: 0.9rem; line-height: 1.5; }
</style>
</head>
<body>
<header>
<h1>Build failed</h1>
<p>gin-kit dev &mdash; this page reloads automatically after the next successful build.</p>
</header>
<pre>{{.}}</pre>
</body>
</html>
`))

// overlayHTML renders the compile-error overlay with the output escaped.
func overlayHTML(output string) []byte {
	var buf bytes.Buffer
	if err := devOverlayTemplate.Execute(&buf, output); err != nil {
		return []byte("<pre>" + template.HTMLEscapeString(output) + "</pre>")
	}
	return buf.Bytes()
}
