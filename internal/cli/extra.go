package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

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

func generateCommand() *cobra.Command {
	var dryRun bool
	generate := &cobra.Command{Use: "generate", Short: "Generate Go project building blocks"}
	generate.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show generated files without writing them")
	for _, kind := range []string{"handler", "service", "domain", "repository", "middleware", "migration", "resource"} {
		k := kind
		generate.AddCommand(&cobra.Command{
			Use:   k + " <name>",
			Short: "Generate a " + k,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				rootDir, m, err := projectRoot()
				if err != nil {
					return err
				}
				name := args[0]
				snake := snakeCase(name)
				switch k {
				case "migration":
					stamp := time.Now().UTC().Format("20060102150405")
					path := filepath.Join(rootDir, "migrations", fmt.Sprintf("%s_%s.sql", stamp, snake))
					for i := 1; ; i++ {
						if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
							break
						}
						path = filepath.Join(rootDir, "migrations", fmt.Sprintf("%s_%s_%d.sql", stamp, snake, i))
					}
					return writeGeneratedFiles(rootDir, map[string][]byte{path: []byte("-- +goose Up\n\n-- +goose Down\n")}, dryRun)
				case "resource":
					return generateResource(rootDir, m, name, dryRun)
				default:
					dir := filepath.Join(rootDir, "internal", k)
					if k == "middleware" {
						dir = filepath.Join(rootDir, "internal", "middleware")
					}
					path := filepath.Join(dir, snake+".go")
					content := fmt.Sprintf("package %s\n\n// %s is a generated %s placeholder. Replace this with project behavior.\ntype %s struct{}\n", k, name, k, name)
					return writeGeneratedFiles(rootDir, map[string][]byte{path: []byte(content)}, dryRun)
				}
			},
		})
	}
	return generate
}

func generateResource(rootDir string, m Manifest, name string, dryRun bool) error {
	snake := snakeCase(name)
	files := map[string]string{
		filepath.Join("internal", "domain", snake+".go"):  fmt.Sprintf("package domain\n\n// %s is a generated domain model.\ntype %s struct {\n\tID string `json:\"id\" db:\"id\"`\n\tName string `json:\"name\" db:\"name\"`\n}\n", name, name),
		filepath.Join("internal", "service", snake+".go"): fmt.Sprintf("package service\n\n// %sService contains business rules for %s.\ntype %sService struct{}\n", name, name, name),
		filepath.Join("internal", "handler", snake+".go"): fmt.Sprintf("package handler\n\n// %sHandler exposes the %s use cases over HTTP.\ntype %sHandler struct{}\n", name, name, name),
	}
	absolute := make(map[string][]byte, len(files))
	for rel, content := range files {
		path := filepath.Join(rootDir, rel)
		absolute[path] = []byte(content)
	}
	if err := writeGeneratedFiles(rootDir, absolute, dryRun); err != nil {
		return err
	}
	if !dryRun {
		fmt.Printf("Generated %s resource skeleton for %s (%s/%s).\n", name, m.Mode, m.Database, m.ORM)
	}
	return nil
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

func snakeCase(value string) string {
	var out []rune
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}
