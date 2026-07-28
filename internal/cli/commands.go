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

// runCommand performs this package operation.
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

// buildCommand performs this package operation.
func buildCommand() *cobra.Command {
	return &cobra.Command{Use: "build", Short: "Build server, migration, and seed binaries", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir, _, err := projectRoot()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(rootDir, "bin"), 0o755); err != nil {
			return err
		}
		for _, name := range []string{"server", "migrate", "seed"} {
			if _, err := os.Stat(filepath.Join(rootDir, "cmd", name)); err != nil {
				continue
			}
			c := exec.Command("go", "build", "-trimpath", "-o", "./bin/"+name, "./cmd/"+name)
			c.Dir, c.Stdout, c.Stderr = rootDir, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
		}
		return nil
	}}
}

// runProjectCommand shells a go run command inside the project root.
func runProjectCommand(rootDir string, extraEnv []string, args ...string) error {
	c := exec.Command("go", args...)
	c.Dir, c.Stdout, c.Stderr, c.Stdin = rootDir, os.Stdout, os.Stderr, os.Stdin
	c.Env = append(os.Environ(), extraEnv...)
	return c.Run()
}

// seedUnavailable performs this package operation.
func seedUnavailable(rootDir string) error {
	if _, err := os.Stat(filepath.Join(rootDir, "cmd", "seed")); err != nil {
		return diagnostic("seed_unavailable", "seed database", rootDir,
			fmt.Errorf("cmd/seed does not exist"),
			"This project predates seeding support. Create cmd/seed/main.go and internal/database/seeders (see docs/cli.md), or regenerate the project.")
	}
	return nil
}

// dbCommand performs this package operation.
func dbCommand() *cobra.Command {
	db := &cobra.Command{Use: "db", Short: "Run database operations"}
	for _, item := range []struct{ operation, short string }{
		{"up", "Apply pending migrations"},
		{"down", "Roll back the most recent migration"},
		{"status", "Show migration status"},
		{"redo", "Roll back and re-apply the latest migration"},
		{"reset", "Roll back every migration (destructive)"},
	} {
		operation, short := item.operation, item.short
		db.AddCommand(&cobra.Command{Use: operation, Short: short, RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			if operation == "reset" && !confirmDestructive(cmd) {
				return errDestructiveNotConfirmed
			}
			return runProjectCommand(rootDir, nil, "run", "./cmd/migrate", operation)
		}})
	}
	db.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Create an empty timestamped migration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			path := migrationPath(rootDir, snakeCase(args[0]))
			return writeGeneratedFiles(rootDir, map[string][]byte{path: []byte("-- +goose Up\n\n-- +goose Down\n")}, false)
		},
	})
	db.AddCommand(&cobra.Command{
		Use:   "seed",
		Short: "Run the project's registered seeders",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			if err := seedUnavailable(rootDir); err != nil {
				return err
			}
			return runProjectCommand(rootDir, nil, "run", "./cmd/seed")
		},
	})
	db.AddCommand(&cobra.Command{
		Use:   "fresh",
		Short: "Reset the schema, migrate up, and seed (destructive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			if !confirmDestructive(cmd) {
				return errDestructiveNotConfirmed
			}
			if err := runProjectCommand(rootDir, nil, "run", "./cmd/migrate", "reset"); err != nil {
				return err
			}
			if err := runProjectCommand(rootDir, nil, "run", "./cmd/migrate", "up"); err != nil {
				return err
			}
			if err := seedUnavailable(rootDir); err != nil {
				fmt.Println("skipping seed: cmd/seed does not exist in this project")
				return nil
			}
			return runProjectCommand(rootDir, nil, "run", "./cmd/seed")
		},
	})
	for _, sub := range db.Commands() {
		if sub.Use == "reset" || sub.Use == "fresh" {
			sub.Flags().Bool("yes", false, "confirm the destructive operation")
		}
	}
	return db
}

// errDestructiveNotConfirmed define package-level implementation state.
var errDestructiveNotConfirmed = fmt.Errorf("refusing to run a destructive database operation; re-run with --yes")

// confirmDestructive performs this package operation.
func confirmDestructive(cmd *cobra.Command) bool {
	confirmed, _ := cmd.Flags().GetBool("yes")
	return confirmed
}

// routesCommand performs this package operation.
func routesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "List the application's HTTP routes",
		Long:  "Boots the application (a reachable database is required) and prints its routing table.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			return runProjectCommand(rootDir, []string{"GIN_MODE=release"}, "run", "./cmd/server", "--routes")
		},
	}
}

// addDirs performs this package operation.
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

// shouldRestart performs this package operation.
func shouldRestart(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".go" || ext == ".html" || ext == ".yaml" || ext == ".yml" || ext == ".css" || ext == ".js"
}
