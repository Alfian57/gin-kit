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

// templateFS define package-level implementation state.
//
//go:embed templates/* templates/runtime/.env.example
var templateFS embed.FS

// newCommand performs this package operation.
func newCommand() *cobra.Command {
	var nonInteractive bool
	var modulePath, projectType, mode, database, orm, runtimeVersion, runtimeReplace string
	var auth, oauth, example, docker bool
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
			m := Manifest{Version: 3, ProjectType: projectType, Project: name, Module: modulePath, Mode: mode, Database: database, ORM: orm, Auth: auth, OAuth: oauth, Example: example, Docker: docker}
			if !nonInteractive {
				m, err = promptManifest(name)
				if err != nil {
					return err
				}
			} else if m.Module == "" || m.Mode == "" || m.Database == "" || m.ORM == "" {
				return diagnostic("selection_required", "validate project options", target, errors.New("non-interactive mode requires --module, --mode, --database and --orm"), "Provide every required flag or remove --non-interactive.")
			}
			if m.ProjectType == "" {
				m.ProjectType = "runtime"
			}
			if m.ProjectType == "runtime" {
				m.RuntimeVersion = strings.TrimPrefix(runtimeVersion, "v")
				if m.RuntimeVersion == "" {
					m.RuntimeVersion = effectiveVersion()
				}
				if m.RuntimeVersion == "dev" && runtimeReplace == "" {
					return diagnostic("runtime_version_required", "resolve runtime version", target, errors.New("development CLI builds do not identify a released runtime version"), "Pass --runtime-version <version>, or use --runtime-replace <local-repository> while developing gin-kit.")
				}
			} else if runtimeVersion != "" || runtimeReplace != "" {
				return diagnostic("runtime_option_unsupported", "validate project options", target, errors.New("runtime options require the runtime project type"), "Use --project-type runtime, or remove --runtime-version and --runtime-replace.")
			}
			if err := validateManifest(m); err != nil {
				return err
			}
			if _, err := os.Stat(target); err == nil {
				return diagnostic("target_exists", "create project", target, errors.New("target already exists"), "Choose an empty target path or remove the existing directory.")
			} else if !os.IsNotExist(err) {
				return diagnostic("target_unavailable", "inspect project target", target, err, "Check the target permissions and try again.")
			}
			if err := scaffoldWithOptions(target, m, scaffoldOptions{RuntimeReplace: runtimeReplace}); err != nil {
				return err
			}
			fmt.Printf("Created %s (%s project type, %s, %s, %s).\nNext steps:\n  cd %s\n  gin-kit run\n", name, m.ProjectType, m.Mode, m.Database, m.ORM, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "disable prompts")
	cmd.Flags().StringVar(&modulePath, "module", "", "Go module path")
	cmd.Flags().StringVar(&projectType, "project-type", "runtime", "runtime or standalone")
	cmd.Flags().StringVar(&mode, "mode", "", "api or ui")
	cmd.Flags().StringVar(&database, "database", "", "sqlite, postgres, mysql, or mariadb")
	cmd.Flags().StringVar(&orm, "orm", "", "gorm or sqlx")
	cmd.Flags().BoolVar(&auth, "auth", false, "include authentication")
	cmd.Flags().BoolVar(&oauth, "oauth", false, "include Google and GitHub OAuth social login (requires --auth)")
	cmd.Flags().BoolVar(&example, "example", false, "include tasks example")
	cmd.Flags().BoolVar(&docker, "docker", false, "include Docker files")
	cmd.Flags().StringVar(&runtimeVersion, "runtime-version", "", "gin-kit runtime version (defaults to the CLI release)")
	cmd.Flags().StringVar(&runtimeReplace, "runtime-replace", "", "local gin-kit repository override for runtime development")
	return cmd
}

// templateData defines an implementation type used by this package.
type templateData struct {
	Manifest
	// Package store data used by this type.
	Package string
	// RuntimeRequire store data used by this type.
	RuntimeRequire string
	// RuntimeReplace store data used by this type.
	RuntimeReplace string
}

// scaffold performs this package operation.
func scaffold(target string, m Manifest) error {
	return scaffoldWithOptions(target, m, scaffoldOptions{})
}

// scaffoldOptions defines an implementation type used by this package.
type scaffoldOptions struct {
	// RuntimeReplace store data used by this type.
	RuntimeReplace string
}

// scaffoldWithOptions performs this package operation.
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

// scaffoldInto performs this package operation.
func scaffoldInto(target string, m Manifest) error {
	return scaffoldIntoWithOptions(target, m, scaffoldOptions{})
}

// scaffoldIntoWithOptions performs this package operation.
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
	if m.ProjectType == "standalone" {
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
	runtimeRequire := ""
	runtimeReplace := ""
	if m.ProjectType == "runtime" {
		version := strings.TrimPrefix(m.RuntimeVersion, "v")
		if version == "" || version == "dev" {
			version = "0.0.0"
		}
		runtimeRequire = "require github.com/Alfian57/gin-kit v" + version
		if options.RuntimeReplace != "" {
			local, err := filepath.Abs(options.RuntimeReplace)
			if err != nil {
				return nil, diagnostic("runtime_replace_invalid", "resolve runtime override", options.RuntimeReplace, err, "Pass a valid local gin-kit repository path.")
			}
			runtimeReplace = "\nreplace github.com/Alfian57/gin-kit => " + filepath.ToSlash(local)
		}
	}
	data := templateData{Manifest: m, Package: filepath.Base(m.Module), RuntimeRequire: runtimeRequire, RuntimeReplace: runtimeReplace}
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
		if strings.Contains(rel, "internal/platform/session") && m.Mode != "ui" && !m.OAuth {
			return nil
		}
		if strings.HasPrefix(rel, "e2e/") && m.Mode != "ui" {
			return nil
		}
		if (strings.Contains(rel, "auth_") || strings.Contains(rel, "/auth/")) && !m.Auth {
			return nil
		}
		if (strings.Contains(rel, "oauth_") || strings.Contains(rel, "/oauth/")) && !m.OAuth {
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

// formatGeneratedGo performs this package operation.
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

// validateManifest performs this package operation.
func validateManifest(m Manifest) error {
	if m.Version != 3 {
		return diagnostic("manifest_version_invalid", "validate manifest", ".gin-kit.yaml", fmt.Errorf("expected version 3, got %d", m.Version), "Use a version 3 manifest.")
	}
	if strings.TrimSpace(m.Project) == "" || strings.ContainsAny(m.Project, `/\`) || m.Project == "." || m.Project == ".." {
		return diagnostic("project_name_invalid", "validate manifest", m.Project, errors.New("project name must be non-empty and must not contain path separators"), "Use a directory name such as orders-api.")
	}
	if err := checkModulePath(m.Module); err != nil {
		return diagnostic("module_path_invalid", "validate manifest", m.Module, err, "Use a canonical Go module path such as example.com/team/service.")
	}
	if m.ProjectType != "runtime" && m.ProjectType != "standalone" {
		return diagnostic("project_type_invalid", "validate manifest", m.ProjectType, errors.New("project_type must be runtime or standalone"), "Choose --project-type runtime or --project-type standalone.")
	}
	if m.ProjectType == "runtime" && (m.RuntimeVersion == "" || !regexp.MustCompile(`^(dev|[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)$`).MatchString(strings.TrimPrefix(m.RuntimeVersion, "v"))) {
		return diagnostic("runtime_version_invalid", "validate manifest", m.RuntimeVersion, errors.New("runtime version must be semantic"), "Use a version such as 0.3.0.")
	}
	if m.ProjectType == "standalone" && m.RuntimeVersion != "" {
		return diagnostic("runtime_version_unsupported", "validate manifest", m.RuntimeVersion, errors.New("runtime_version is only valid for runtime projects"), "Remove runtime_version from a standalone project manifest.")
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
	if m.OAuth && !m.Auth {
		return diagnostic("oauth_auth_required", "validate manifest", ".gin-kit.yaml", errors.New("oauth requires authentication"), "Enable authentication with --auth before adding --oauth.")
	}
	return nil
}

// checkModulePath performs this package operation.
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

// templateOutputPath performs this package operation.
func templateOutputPath(rel string, m Manifest) (string, bool) {
	const runtimePrefix = "runtime/"
	if strings.HasPrefix(rel, runtimePrefix) {
		if m.ProjectType != "runtime" {
			return "", false
		}
		return strings.TrimPrefix(rel, runtimePrefix), true
	}
	if m.ProjectType == "runtime" {
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
			rel == "migrations/00003_auth_tokens.sql",
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
