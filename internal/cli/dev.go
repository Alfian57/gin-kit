package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const (
	devDebounceDelay = 250 * time.Millisecond
	devHoldTimeout   = 30 * time.Second
	devReadyTimeout  = 15 * time.Second
	devStopGrace     = 5 * time.Second
	devTailLines     = 100
)

// errDevChildExited reports that the server process exited while gin-kit dev
// was waiting for it to accept connections.
var errDevChildExited = errors.New("server process exited")

func devCommand() *cobra.Command {
	var proxyPort, appPort int
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run a hot-reload dev server with a holding proxy",
		Long: "Builds ./cmd/server to a binary, watches the project, and rebuilds on change.\n" +
			"Requests arriving during a rebuild are held by a local reverse proxy instead of\n" +
			"failing, and compile errors render as a browser overlay until the next\n" +
			"successful build. Use `gin-kit run` for the simple go run-based loop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDev(cmd, proxyPort, appPort)
		},
	}
	cmd.Flags().IntVar(&proxyPort, "port", 8080, "port the dev proxy listens on")
	cmd.Flags().IntVar(&appPort, "app-port", 0, "port the application listens on (0 picks a free port)")
	return cmd
}

func runDev(cmd *cobra.Command, proxyPort, appPort int) error {
	rootDir, manifest, err := projectRoot()
	if err != nil {
		return err
	}
	uiMode := manifest.Mode == "ui"

	// The CLI does not wire a signal-aware root context, so derive one here:
	// dev must stop the child, close the proxy, and remove its temp dir on
	// Ctrl+C or SIGTERM.
	ctx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// The application port stays constant across restarts so the proxy target
	// never changes.
	if appPort == 0 {
		appPort, err = pickFreePort()
		if err != nil {
			return fmt.Errorf("could not pick a free application port: %w", err)
		}
	}
	appAddr := fmt.Sprintf("127.0.0.1:%d", appPort)
	targetURL := &url.URL{Scheme: "http", Host: appAddr}

	tmpDir, err := os.MkdirTemp("", "gin-kit-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	binPath := filepath.Join(tmpDir, "server")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := addDevDirs(watcher, rootDir); err != nil {
		return err
	}

	proxy := newDevProxy(devHoldTimeout)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", proxyPort))
	if err != nil {
		return fmt.Errorf("dev proxy could not listen on port %d: %w", proxyPort, err)
	}
	server := &http.Server{Handler: proxy}
	defer server.Close()
	serverErr := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr <- serveErr
		}
	}()

	tail := newTailBuffer(devTailLines)
	var child *exec.Cmd
	var childDone chan error
	defer func() {
		if child != nil {
			stopChild(child, childDone)
		}
	}()

	startChild := func() (*exec.Cmd, chan error, error) {
		c := exec.Command(binPath)
		c.Dir = rootDir
		// The real environment wins over .env in generated projects, so this
		// PORT assignment is authoritative.
		c.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", appPort))
		c.Stdout = io.MultiWriter(os.Stdout, tail)
		c.Stderr = io.MultiWriter(os.Stderr, tail)
		if err := c.Start(); err != nil {
			return nil, nil, err
		}
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()
		return c, done, nil
	}

	rebuild := func() {
		proxy.SetBuilding()
		fmt.Println("gin-kit dev: building...")
		output, buildErr := buildProject(rootDir, binPath)
		if buildErr != nil {
			// Keep the previous child running; the overlay is shown until a
			// build succeeds again.
			fmt.Fprintln(os.Stderr, "gin-kit dev: build failed")
			if output != "" {
				fmt.Fprint(os.Stderr, output)
			}
			proxy.SetFailed(output)
			return
		}
		if child != nil {
			stopChild(child, childDone)
			child, childDone = nil, nil
		}
		newChild, newDone, startErr := startChild()
		if startErr != nil {
			proxy.SetFailed("failed to start server: " + startErr.Error())
			fmt.Fprintln(os.Stderr, "gin-kit dev: failed to start server:", startErr)
			return
		}
		child, childDone = newChild, newDone
		if readyErr := waitTCPReady(appAddr, devReadyTimeout, childDone); readyErr != nil {
			if errors.Is(readyErr, errDevChildExited) {
				child, childDone = nil, nil
			}
			proxy.SetFailed(readyErr.Error() + "\n" + tail.Tail())
			fmt.Fprintln(os.Stderr, "gin-kit dev:", readyErr)
			return
		}
		proxy.SetReady(targetURL)
		fmt.Printf("gin-kit dev: ready on http://localhost:%d\n", proxyPort)
	}

	fmt.Printf("gin-kit dev: proxy on http://localhost:%d, application on http://%s\n", proxyPort, appAddr)
	rebuild()

	debounce := newDebouncer(devDebounceDelay)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErr:
			return fmt.Errorf("dev proxy server failed: %w", err)
		case exitErr := <-childDone:
			child, childDone = nil, nil
			message := "process exited"
			if exitErr != nil {
				message = fmt.Sprintf("process exited: %v", exitErr)
			}
			fmt.Fprintln(os.Stderr, "gin-kit dev: "+message+" (save a file to restart)")
			proxy.SetFailed(message + "\n" + tail.Tail())
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() && !devIgnoreDir(filepath.Base(event.Name)) {
					_ = watcher.Add(event.Name)
				}
			}
			if devShouldRebuild(event.Name, uiMode) {
				debounce.Trigger()
			}
		case <-debounce.C():
			rebuild()
		case watcherErr, ok := <-watcher.Errors:
			if ok && watcherErr != nil {
				return fmt.Errorf("file watcher failed: %w", watcherErr)
			}
			return nil
		}
	}
}

// devIgnoreDir reports whether a directory name should be excluded from the
// dev watcher.
func devIgnoreDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "bin", "tmp", "dist":
		return true
	}
	return false
}

// devShouldRebuild reports whether a change to path warrants a rebuild.
func devShouldRebuild(path string, uiMode bool) bool {
	switch filepath.Base(path) {
	case "go.mod", "go.sum", ".env":
		return true
	}
	switch filepath.Ext(path) {
	case ".go":
		return true
	case ".html", ".css", ".js":
		return uiMode
	}
	return false
}

func addDevDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && devIgnoreDir(entry.Name()) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

// pickFreePort asks the kernel for an ephemeral TCP port and releases it.
func pickFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// buildProject compiles ./cmd/server into binPath and returns the combined
// compiler output when the build fails.
func buildProject(rootDir, binPath string) (string, error) {
	c := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	c.Dir = rootDir
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return "", nil
}

// stopChild terminates the child gracefully: SIGTERM, a grace period, then
// SIGKILL. done must be the un-drained channel receiving the child's Wait
// result.
func stopChild(child *exec.Cmd, done <-chan error) {
	if child == nil || child.Process == nil {
		return
	}
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		_ = child.Process.Kill()
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(devStopGrace):
		_ = child.Process.Kill()
		<-done
	}
}

// waitTCPReady polls addr until it accepts a TCP connection, the timeout
// elapses, or the child process exits.
func waitTCPReady(addr string, timeout time.Duration, childExited <-chan error) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case exitErr := <-childExited:
			if exitErr != nil {
				return fmt.Errorf("%w: %v", errDevChildExited, exitErr)
			}
			return errDevChildExited
		case <-time.After(50 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server did not accept connections on %s within %s", addr, timeout)
		}
	}
}

// debouncer coalesces bursts of triggers into a single fire on its channel
// once the burst has been quiet for the configured delay.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	delay time.Duration
	ch    chan struct{}
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{delay: delay, ch: make(chan struct{}, 1)}
}

// Trigger (re)starts the quiet-period timer.
func (d *debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, func() {
		select {
		case d.ch <- struct{}{}:
		default:
		}
	})
}

// C returns the channel that receives one value per coalesced burst.
func (d *debouncer) C() <-chan struct{} { return d.ch }

// tailBuffer is an io.Writer that retains the last max lines written to it.
type tailBuffer struct {
	mu      sync.Mutex
	max     int
	lines   []string
	partial strings.Builder
}

func newTailBuffer(max int) *tailBuffer { return &tailBuffer{max: max} }

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range p {
		if c == '\n' {
			b.lines = append(b.lines, b.partial.String())
			b.partial.Reset()
			if len(b.lines) > b.max {
				b.lines = b.lines[len(b.lines)-b.max:]
			}
			continue
		}
		b.partial.WriteByte(c)
	}
	return len(p), nil
}

// Tail returns the retained lines joined with newlines.
func (b *tailBuffer) Tail() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := append([]string(nil), b.lines...)
	if b.partial.Len() > 0 {
		lines = append(lines, b.partial.String())
	}
	return strings.Join(lines, "\n")
}
