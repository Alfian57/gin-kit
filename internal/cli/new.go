package cli

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed templates/* templates/framework/.env.example
var templateFS embed.FS

func newCommand() *cobra.Command {
	var nonInteractive bool
	var modulePath, edition, mode, database, orm, frameworkVersion, frameworkReplace string
	var auth, example, docker bool
	cmd := &cobra.Command{
		Use:   "new <project>",
		Short: "Create a new gin-kit project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := filepath.Abs(filepath.Clean(args[0]))
			if err != nil {
				return diagnostic("target_invalid", "resolve project target", args[0], err, "Choose a valid directory path.")
			}
			name := filepath.Base(target)
			if name == "." || name == string(filepath.Separator) || name == "" {
				return diagnostic("target_invalid", "resolve project target", args[0], errors.New("target must name a project directory"), "Choose a path such as ./my-service.")
			}
			m := Manifest{Version: 2, Edition: edition, Project: name, Module: modulePath, Mode: mode, Database: database, ORM: orm, Auth: auth, Example: example, Docker: docker}
			if !nonInteractive {
				m, err = promptManifest(name)
				if err != nil {
					return err
				}
			} else if m.Module == "" || m.Mode == "" || m.Database == "" || m.ORM == "" {
				return diagnostic("selection_required", "validate project options", target, errors.New("non-interactive mode requires --module, --mode, --database and --orm"), "Provide every required flag or remove --non-interactive.")
			}
			if m.Edition == "" {
				m.Edition = "framework"
			}
			if m.Edition == "framework" {
				m.FrameworkVersion = strings.TrimPrefix(frameworkVersion, "v")
				if m.FrameworkVersion == "" {
					m.FrameworkVersion = effectiveVersion()
				}
				if m.FrameworkVersion == "dev" && frameworkReplace == "" {
					return diagnostic("framework_version_required", "resolve framework version", target, errors.New("development CLI builds do not identify a released framework version"), "Pass --framework-version <version>, or use --framework-replace <local-repository> while developing gin-kit.")
				}
			}
			if err := validateManifest(m); err != nil {
				return err
			}
			if _, err := os.Stat(target); err == nil {
				return diagnostic("target_exists", "create project", target, errors.New("target already exists"), "Choose an empty target path or remove the existing directory.")
			} else if !os.IsNotExist(err) {
				return diagnostic("target_unavailable", "inspect project target", target, err, "Check the target permissions and try again.")
			}
			if err := scaffoldWithOptions(target, m, scaffoldOptions{FrameworkReplace: frameworkReplace}); err != nil {
				return err
			}
			fmt.Printf("Created %s (%s edition, %s, %s, %s).\nNext steps:\n  cd %s\n  gin-kit run\n", name, m.Edition, m.Mode, m.Database, m.ORM, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "disable prompts")
	cmd.Flags().StringVar(&modulePath, "module", "", "Go module path")
	cmd.Flags().StringVar(&edition, "edition", "framework", "framework or starter")
	cmd.Flags().StringVar(&mode, "mode", "", "api or ui")
	cmd.Flags().StringVar(&database, "database", "", "sqlite, postgres, mysql, or mariadb")
	cmd.Flags().StringVar(&orm, "orm", "", "gorm or sqlx")
	cmd.Flags().BoolVar(&auth, "auth", false, "include authentication")
	cmd.Flags().BoolVar(&example, "example", false, "include tasks example")
	cmd.Flags().BoolVar(&docker, "docker", false, "include Docker files")
	cmd.Flags().StringVar(&frameworkVersion, "framework-version", "", "gin-kit framework version (defaults to the CLI release)")
	cmd.Flags().StringVar(&frameworkReplace, "framework-replace", "", "local gin-kit repository override for framework development")
	return cmd
}

type templateData struct {
	Manifest
	Package          string
	FrameworkRequire string
	FrameworkReplace string
}

func scaffold(target string, m Manifest) error {
	return scaffoldWithOptions(target, m, scaffoldOptions{})
}

type scaffoldOptions struct {
	FrameworkReplace string
}

func scaffoldWithOptions(target string, m Manifest, options scaffoldOptions) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return diagnostic("target_parent_failed", "create target parent", parent, err, "Check directory permissions and available disk space.")
	}
	staging, err := os.MkdirTemp(parent, ".gin-kit-scaffold-*")
	if err != nil {
		return diagnostic("staging_failed", "create scaffold staging directory", parent, err, "Check directory permissions and available disk space.")
	}
	if err := scaffoldIntoWithOptions(staging, m, options); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		_ = os.RemoveAll(staging)
		return diagnostic("publish_failed", "publish scaffold", target, err, "Ensure the target does not exist and is on the same filesystem.")
	}
	return nil
}

func scaffoldInto(target string, m Manifest) error {
	return scaffoldIntoWithOptions(target, m, scaffoldOptions{})
}

func scaffoldIntoWithOptions(target string, m Manifest, options scaffoldOptions) error {
	if m.Module == "" {
		m.Module = "example.com/" + m.Project
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	files, err := renderScaffoldTree(m, options, nil)
	if err != nil {
		return err
	}
	rels := make([]string, 0, len(files))
	for rel := range files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		out := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, files[rel], 0o644); err != nil {
			return err
		}
	}
	if err := writeManifest(target, m); err != nil {
		return err
	}
	if err := formatGeneratedGo(target); err != nil {
		return err
	}
	if m.Edition == "starter" {
		// Record the post-gofmt checksums of the vendored platform files so
		// gin-kit upgrade can tell local edits from stale vendored copies.
		if err := writeBaselineFromDisk(target); err != nil {
			return err
		}
	}
	return nil
}

// renderScaffoldTree renders every embedded template the manifest selects, in
// memory. Keys are project-relative slash paths. filter, when non-nil, limits
// the output.
func renderScaffoldTree(m Manifest, options scaffoldOptions, filter func(rel string) bool) (map[string][]byte, error) {
	if m.Module == "" {
		m.Module = "example.com/" + m.Project
	}
	frameworkRequire := ""
	frameworkReplace := ""
	if m.Edition == "framework" {
		version := strings.TrimPrefix(m.FrameworkVersion, "v")
		if version == "" || version == "dev" {
			version = "0.0.0"
		}
		frameworkRequire = "require github.com/Alfian57/gin-kit v" + version
		if options.FrameworkReplace != "" {
			local, err := filepath.Abs(options.FrameworkReplace)
			if err != nil {
				return nil, diagnostic("framework_replace_invalid", "resolve framework override", options.FrameworkReplace, err, "Pass a valid local gin-kit repository path.")
			}
			frameworkReplace = "\nreplace github.com/Alfian57/gin-kit => " + filepath.ToSlash(local)
		}
	}
	data := templateData{Manifest: m, Package: filepath.Base(m.Module), FrameworkRequire: frameworkRequire, FrameworkReplace: frameworkReplace}
	files := map[string][]byte{}
	err := fs.WalkDir(templateFS, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, include := templateOutputPath(strings.TrimPrefix(path, "templates/"), m)
		if !include {
			return nil
		}
		if strings.HasSuffix(rel, ".tmpl") {
			rel = strings.TrimSuffix(rel, ".tmpl")
		}
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
		if strings.Contains(rel, "internal/platform/session") && m.Mode != "ui" {
			return nil
		}
		if strings.HasPrefix(rel, "e2e/") && m.Mode != "ui" {
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
		if filter != nil && !filter(rel) {
			return nil
		}
		raw, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tmpl") {
			t, err := template.New(rel).Parse(string(raw))
			if err != nil {
				return err
			}
			var rendered bytes.Buffer
			if err := t.Execute(&rendered, data); err != nil {
				return err
			}
			files[rel] = rendered.Bytes()
			return nil
		}
		files[rel] = raw
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func formatGeneratedGo(root string) error {
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(path) == ".go" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	}); err != nil {
		return diagnostic("format_inspection_failed", "format generated Go", root, err, "Inspect the staged scaffold and try again.")
	}
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"-w"}, files...)
	command := exec.Command("gofmt", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return diagnostic("format_failed", "format generated Go", root, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))), "Install Go and retry the scaffold.")
	}
	return nil
}

func validateManifest(m Manifest) error {
	if m.Version != 2 {
		return diagnostic("manifest_version_invalid", "validate manifest", ".gin-kit.yaml", fmt.Errorf("expected version 2, got %d", m.Version), "Use a version 2 manifest.")
	}
	if strings.TrimSpace(m.Project) == "" || strings.ContainsAny(m.Project, `/\`) || m.Project == "." || m.Project == ".." {
		return diagnostic("project_name_invalid", "validate manifest", m.Project, errors.New("project name must be non-empty and must not contain path separators"), "Use a directory name such as orders-api.")
	}
	if err := checkModulePath(m.Module); err != nil {
		return diagnostic("module_path_invalid", "validate manifest", m.Module, err, "Use a canonical Go module path such as example.com/team/service.")
	}
	if m.Edition != "framework" && m.Edition != "starter" {
		return diagnostic("edition_invalid", "validate manifest", m.Edition, errors.New("edition must be framework or starter"), "Choose --edition framework or --edition starter.")
	}
	if m.Edition == "framework" && (m.FrameworkVersion == "" || !regexp.MustCompile(`^(dev|[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)$`).MatchString(strings.TrimPrefix(m.FrameworkVersion, "v"))) {
		return diagnostic("framework_version_invalid", "validate manifest", m.FrameworkVersion, errors.New("framework version must be semantic"), "Use a version such as 0.3.0.")
	}
	if m.Mode != "api" && m.Mode != "ui" {
		return fmt.Errorf("invalid mode %q: use api or ui", m.Mode)
	}
	switch m.Database {
	case "sqlite", "postgres", "mysql", "mariadb":
	default:
		return fmt.Errorf("invalid database %q", m.Database)
	}
	if m.ORM != "gorm" && m.ORM != "sqlx" {
		return fmt.Errorf("invalid ORM %q: use gorm or sqlx", m.ORM)
	}
	return nil
}

func checkModulePath(path string) error {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsAny(path, " \t\r\n") ||
		strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") {
		return errors.New("module path must be a slash-separated path without whitespace")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]*(/[A-Za-z0-9][A-Za-z0-9._~-]*)*$`).MatchString(path) {
		return errors.New("module path contains an invalid path element")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
			return errors.New("module path elements cannot be relative or hidden")
		}
	}
	return nil
}

func templateOutputPath(rel string, m Manifest) (string, bool) {
	const frameworkPrefix = "framework/"
	if strings.HasPrefix(rel, frameworkPrefix) {
		if m.Edition != "framework" {
			return "", false
		}
		return strings.TrimPrefix(rel, frameworkPrefix), true
	}
	if m.Edition == "framework" {
		switch {
		case rel == ".gitignore",
			rel == "AGENTS.md.tmpl",
			rel == "CLAUDE.md",
			rel == "GEMINI.md",
			rel == ".github/copilot-instructions.md",
			rel == ".github/skills/gin-kit-development/SKILL.md.tmpl",
			rel == ".cursor/rules/gin-kit.mdc",
			rel == "cmd/migrate/main.go.tmpl",
			rel == "internal/database/seeders/seeders.go.tmpl",
			rel == "package.json.tmpl",
			rel == "migrations/00001_init.sql",
			rel == "docker/docker-compose.yml.tmpl",
			strings.HasPrefix(rel, "web/assets/"),
			strings.HasPrefix(rel, "web/src/"),
			strings.HasPrefix(rel, "web/templates/"):
			return rel, true
		default:
			return "", false
		}
	}
	return rel, true
}
