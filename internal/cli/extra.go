package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

func explainCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <topic>",
		Short: "Explain how the generated project works",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, m, err := projectRoot()
			if err != nil {
				return err
			}
			topics := map[string]string{
				"architecture": "Requests move through router, handler, service, repository, and database. Constructors wire each dependency explicitly.",
				"request-flow": "The router selects a handler. The handler binds and validates input. The service owns business rules. The repository owns persistence.",
				"database":     fmt.Sprintf("This project uses %s with %s. Schema changes belong in migrations and are applied with `gin-kit db up`.", m.Database, m.ORM),
				"auth":         "Authentication uses Argon2id password hashes, short-lived access tokens, rotating refresh tokens, and protected middleware.",
				"commands":     "Use `gin-kit run`, `gin-kit build`, `gin-kit generate resource`, `gin-kit db up`, and `gin-kit check`.",
			}
			text, ok := topics[args[0]]
			if !ok {
				return fmt.Errorf("unknown topic %q (try architecture, request-flow, database, auth, or commands)", args[0])
			}
			fmt.Println(text)
			return nil
		},
	}
}

func doctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local prerequisites", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir, m, err := projectRoot()
		if err != nil {
			return err
		}
		fmt.Println("gin-kit doctor")
		fmt.Println("  project:", rootDir)
		fmt.Println("  module:", m.Module)
		for _, tool := range []string{"go", "git"} {
			if path, e := exec.LookPath(tool); e == nil {
				fmt.Println("  "+tool+":", path)
			} else {
				fmt.Println("  " + tool + ": missing")
			}
		}
		if m.Mode == "ui" {
			for _, tool := range []string{"node", "npm"} {
				if path, e := exec.LookPath(tool); e == nil {
					fmt.Println("  "+tool+":", path)
				} else {
					fmt.Println("  " + tool + ": missing (required for UI assets)")
				}
			}
		}
		if m.Database == "sqlite" {
			fmt.Println("  sqlite: official driver selected; CGO/GCC is required for SQLite builds")
		}
		if m.ProjectType == "runtime" && m.Mode == "ui" {
			fmt.Println("  playwright: optional; install browsers for e2e tests with " +
				"'go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium' (tests skip otherwise)")
		}
		return nil
	}}
}

func checkCommand() *cobra.Command {
	return &cobra.Command{Use: "check", Short: "Run formatting, tests, and static checks", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir, _, err := projectRoot()
		if err != nil {
			return err
		}
		steps := [][]string{{"gofmt", "-l", "."}, {"go", "test", "./..."}, {"go", "vet", "./..."}}
		for _, step := range steps {
			c := exec.Command(step[0], step[1:]...)
			c.Dir, c.Stdout, c.Stderr = rootDir, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				if step[0] == "gofmt" {
					return fmt.Errorf("files need formatting; run gofmt -w . locally")
				}
				return err
			}
		}
		return nil
	}}
}

func writeGeneratedFiles(rootDir string, files map[string][]byte, dryRun bool) error {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
		if _, err := os.Stat(path); err == nil {
			return diagnostic("generated_collision", "generate files", path, fmt.Errorf("%s already exists", path), "Choose a different name or inspect the existing file.")
		} else if !os.IsNotExist(err) {
			return diagnostic("generated_inspection_failed", "inspect generated path", path, err, "Check directory permissions.")
		}
	}
	sort.Strings(paths)
	if dryRun {
		for _, path := range paths {
			fmt.Printf("would create %s\n", path)
		}
		return nil
	}
	staging, err := os.MkdirTemp(rootDir, ".gin-kit-generate-*")
	if err != nil {
		return diagnostic("generation_staging_failed", "stage generated files", rootDir, err, "Check directory permissions and available disk space.")
	}
	defer os.RemoveAll(staging)
	for _, path := range paths {
		content := files[path]
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return diagnostic("generated_path_invalid", "stage generated files", path, err, "Generate files inside the project root.")
		}
		staged := filepath.Join(staging, rel)
		if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
			return diagnostic("generation_directory_failed", "stage generated files", staged, err, "Check directory permissions.")
		}
		if err := os.WriteFile(staged, content, 0o644); err != nil {
			return diagnostic("generation_write_failed", "stage generated files", staged, err, "Check directory permissions and available disk space.")
		}
	}
	// Format staged Go files before publishing so a template bug that renders
	// unparseable code aborts with the project untouched.
	if err := formatGeneratedGo(staging); err != nil {
		return err
	}
	// Every collision is checked before this point. Publishing is intentionally
	// explicit so a failed render cannot leave half-written source files.
	published := make([]string, 0, len(files))
	for _, path := range paths {
		rel, _ := filepath.Rel(rootDir, path)
		staged := filepath.Join(staging, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			for _, created := range published {
				_ = os.Remove(created)
			}
			return diagnostic("generation_directory_failed", "publish generated files", path, err, "Check directory permissions.")
		}
		if err := os.Rename(staged, path); err != nil {
			for _, created := range published {
				_ = os.Remove(created)
			}
			return diagnostic("generation_publish_failed", "publish generated files", path, err, "Remove partial generated files and retry.")
		}
		published = append(published, path)
	}
	return nil
}
