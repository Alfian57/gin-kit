package cli

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed templates/*
var templateFS embed.FS

func newCommand() *cobra.Command {
	var nonInteractive bool
	var module, mode, database, orm string
	var auth, example, docker bool
	cmd := &cobra.Command{
		Use:   "new <project>",
		Short: "Create a new GinKit project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			m := Manifest{Version: 1, Project: name, Module: module, Mode: mode, Database: database, ORM: orm, Auth: auth, Example: example, Docker: docker}
			if !nonInteractive {
				var err error
				m, err = promptManifest(name)
				if err != nil {
					return err
				}
			} else if m.Module == "" || m.Mode == "" || m.Database == "" || m.ORM == "" {
				return errors.New("non-interactive mode requires --module, --mode, --database and --orm")
			}
			target := filepath.Clean(name)
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("target %s already exists", target)
			}
			if err := scaffold(target, m); err != nil {
				return err
			}
			fmt.Printf("Created %s (%s, %s, %s).\nNext steps:\n  cd %s\n  ginkit run\n", name, m.Mode, m.Database, m.ORM, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "disable prompts")
	cmd.Flags().StringVar(&module, "module", "", "Go module path")
	cmd.Flags().StringVar(&mode, "mode", "", "api or ui")
	cmd.Flags().StringVar(&database, "database", "", "sqlite, postgres, mysql, or mariadb")
	cmd.Flags().StringVar(&orm, "orm", "", "gorm or sqlx")
	cmd.Flags().BoolVar(&auth, "auth", false, "include authentication")
	cmd.Flags().BoolVar(&example, "example", false, "include tasks example")
	cmd.Flags().BoolVar(&docker, "docker", false, "include Docker files")
	return cmd
}

type templateData struct {
	Manifest
	Package string
}

func scaffold(target string, m Manifest) error {
	if m.Module == "" {
		m.Module = "example.com/" + m.Project
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	data := templateData{Manifest: m, Package: filepath.Base(m.Module)}
	err := fs.WalkDir(templateFS, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "templates/")
		if strings.HasSuffix(rel, ".tmpl") {
			rel = strings.TrimSuffix(rel, ".tmpl")
		}
		out := filepath.Join(target, rel)
		if strings.Contains(rel, "internal/handler/api") && m.Mode != "api" {
			return nil
		}
		if strings.HasPrefix(rel, "api/") && m.Mode != "api" {
			return nil
		}
		if strings.Contains(rel, "internal/handler/web") && m.Mode != "ui" {
			return nil
		}
		if strings.Contains(rel, "web/") && m.Mode != "ui" {
			return nil
		}
		if rel == "package.json" && m.Mode != "ui" {
			return nil
		}
		if (strings.Contains(rel, "auth_") || strings.Contains(rel, "/auth/")) && !m.Auth {
			return nil
		}
		if strings.Contains(rel, "tasks_") && !m.Example {
			return nil
		}
		if strings.Contains(rel, "docker/") && !m.Docker {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		raw, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(raw)
		if strings.HasSuffix(path, ".tmpl") {
			t, err := template.New(rel).Parse(content)
			if err != nil {
				return err
			}
			f, err := os.Create(out)
			if err != nil {
				return err
			}
			defer f.Close()
			return t.Execute(f, data)
		}
		return os.WriteFile(out, raw, 0o644)
	})
	if err != nil {
		return err
	}
	return writeManifest(target, m)
}
