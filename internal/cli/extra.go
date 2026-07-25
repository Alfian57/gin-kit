package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
				"database":     fmt.Sprintf("This project uses %s with %s. Schema changes belong in migrations and are applied with `ginkit db up`.", m.Database, m.ORM),
				"auth":         "Authentication uses Argon2id password hashes, short-lived access tokens, rotating refresh tokens, and protected middleware.",
				"commands":     "Use `ginkit run`, `ginkit build`, `ginkit generate resource`, `ginkit db up`, and `ginkit check`.",
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
		fmt.Println("GinKit doctor")
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
		steps := [][]string{{"gofmt", "-w", "."}, {"go", "test", "./..."}, {"go", "vet", "./..."}}
		for _, step := range steps {
			c := exec.Command(step[0], step[1:]...)
			c.Dir, c.Stdout, c.Stderr = rootDir, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
		}
		return nil
	}}
}

func generateCommand() *cobra.Command {
	generate := &cobra.Command{Use: "generate", Short: "Generate Go project building blocks"}
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
					path := filepath.Join(rootDir, "migrations", fmt.Sprintf("%d_%s.sql", os.Getpid(), snake))
					return os.WriteFile(path, []byte("-- +goose Up\n\n-- +goose Down\n"), 0o644)
				case "resource":
					return generateResource(rootDir, m, name)
				default:
					dir := filepath.Join(rootDir, "internal", k)
					if k == "middleware" {
						dir = filepath.Join(rootDir, "internal", "middleware")
					}
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
					path := filepath.Join(dir, snake+".go")
					if _, err := os.Stat(path); err == nil {
						return fmt.Errorf("%s already exists", path)
					}
					content := fmt.Sprintf("package %s\n\n// %s is a generated %s placeholder. Replace this with project behavior.\ntype %s struct{}\n", k, name, k, name)
					return os.WriteFile(path, []byte(content), 0o644)
				}
			},
		})
	}
	return generate
}

func generateResource(rootDir string, m Manifest, name string) error {
	snake := snakeCase(name)
	files := map[string]string{
		filepath.Join("internal", "domain", snake+".go"):  fmt.Sprintf("package domain\n\n// %s is a generated domain model.\ntype %s struct {\n\tID string `json:\"id\" db:\"id\"`\n\tName string `json:\"name\" db:\"name\"`\n}\n", name, name),
		filepath.Join("internal", "service", snake+".go"): fmt.Sprintf("package service\n\n// %sService contains business rules for %s.\ntype %sService struct{}\n", name, name, name),
		filepath.Join("internal", "handler", snake+".go"): fmt.Sprintf("package handler\n\n// %sHandler exposes the %s use cases over HTTP.\ntype %sHandler struct{}\n", name, name, name),
	}
	for rel, content := range files {
		path := filepath.Join(rootDir, rel)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Generated %s resource skeleton for %s (%s/%s).\\n", name, m.Mode, m.Database, m.ORM)
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
