package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func runCommand() *cobra.Command {
	return &cobra.Command{Use: "run", Short: "Run the server with hot reload", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir, _, err := projectRoot()
		if err != nil {
			return err
		}
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		defer w.Close()
		if err := addDirs(w, rootDir); err != nil {
			return err
		}
		var child *exec.Cmd
		var childDone chan error
		start := func() {
			if child != nil && child.Process != nil {
				_ = child.Process.Signal(os.Interrupt)
				done := make(chan error, 1)
				go func(old *exec.Cmd) { done <- old.Wait() }(child)
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					_ = child.Process.Kill()
					<-done
				}
			}
			child = exec.Command("go", "run", "./cmd/server")
			child.Dir = rootDir
			child.Stdout, child.Stderr, child.Stdin = os.Stdout, os.Stderr, os.Stdin
			if err := child.Start(); err != nil {
				fmt.Fprintln(os.Stderr, "server:", err)
				child = nil
				return
			}
			childDone = make(chan error, 1)
			go func(current *exec.Cmd, done chan<- error) { done <- current.Wait() }(child, childDone)
		}
		start()
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		var pending bool
		for {
			select {
			case <-cmd.Context().Done():
				if child != nil && child.Process != nil {
					_ = child.Process.Signal(os.Interrupt)
					select {
					case <-childDone:
					case <-time.After(2 * time.Second):
						_ = child.Process.Kill()
						<-childDone
					}
				}
				return nil
			case err := <-childDone:
				if err != nil {
					fmt.Fprintf(os.Stderr, "server stopped: %v (edit a Go file to restart)\n", err)
				}
				child = nil
				childDone = nil
			case event, ok := <-w.Events:
				if !ok {
					return nil
				}
				if event.Has(fsnotify.Create) {
					if info, e := os.Stat(event.Name); e == nil && info.IsDir() {
						_ = w.Add(event.Name)
					}
				}
				if shouldRestart(event.Name) {
					pending = true
				}
			case <-ticker.C:
				if pending {
					pending = false
					start()
				}
			case watcherErr, ok := <-w.Errors:
				if ok && watcherErr != nil {
					return fmt.Errorf("file watcher failed: %w", watcherErr)
				}
				return nil
			}
		}
	}}
}

func buildCommand() *cobra.Command {
	return &cobra.Command{Use: "build", Short: "Build server and migration binaries", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir, _, err := projectRoot()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(rootDir, "bin"), 0o755); err != nil {
			return err
		}
		for _, item := range [][2]string{{"./cmd/server", "./bin/server"}, {"./cmd/migrate", "./bin/migrate"}} {
			c := exec.Command("go", "build", "-trimpath", "-o", item[1], item[0])
			c.Dir, c.Stdout, c.Stderr = rootDir, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
		}
		return nil
	}}
}

func dbCommand() *cobra.Command {
	db := &cobra.Command{Use: "db", Short: "Run database operations"}
	for _, op := range []string{"up", "down", "status"} {
		operation := op
		db.AddCommand(&cobra.Command{Use: operation, RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			c := exec.Command("go", "run", "./cmd/migrate", operation)
			c.Dir, c.Stdout, c.Stderr, c.Stdin = rootDir, os.Stdout, os.Stderr, os.Stdin
			return c.Run()
		}})
	}
	return db
}

func addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && !strings.Contains(path, string(filepath.Separator)+".git") && !strings.Contains(path, "node_modules") && !strings.Contains(path, "bin") {
			return w.Add(path)
		}
		return nil
	})
}

func shouldRestart(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".go" || ext == ".html" || ext == ".yaml" || ext == ".yml" || ext == ".css" || ext == ".js"
}
