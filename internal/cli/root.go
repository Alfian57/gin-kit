package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Manifest defines an implementation type used by this package.
type Manifest struct {
	// Version store data used by this type.
	Version int `yaml:"version"`
	// ProjectType store data used by this type.
	ProjectType string `yaml:"project_type"`
	// RuntimeVersion store data used by this type.
	RuntimeVersion string `yaml:"runtime_version,omitempty"`
	// Module store data used by this type.
	Module string `yaml:"module"`
	// Project store data used by this type.
	Project string `yaml:"project"`
	// Mode store data used by this type.
	Mode string `yaml:"mode"`
	// Database store data used by this type.
	Database string `yaml:"database"`
	// ORM store data used by this type.
	ORM string `yaml:"orm"`
	// Auth store data used by this type.
	Auth bool `yaml:"auth"`
	// OAuth store data used by this type.
	OAuth bool `yaml:"oauth"`
	// Example store data used by this type.
	Example bool `yaml:"example"`
	// Docker store data used by this type.
	Docker bool `yaml:"docker"`
}

// root define package-level implementation state.
var root = &cobra.Command{
	Use:   "gin-kit",
	Short: "An opinionated application framework built on Gin",
}

// Execute performs this package operation.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// init initializes package-level implementation state.
func init() {
	root.Version = effectiveVersion()
	root.AddCommand(newCommand(), generateCommand(), runCommand(), devCommand(), buildCommand(), dbCommand(), routesCommand(), upgradeCommand(), explainCommand(), doctorCommand(), checkCommand())
}

// projectRoot performs this package operation.
func projectRoot() (string, Manifest, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", Manifest{}, err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, ".gin-kit.yaml"))
		if readErr == nil {
			var m Manifest
			decoder := yaml.NewDecoder(bytes.NewReader(data))
			decoder.KnownFields(true)
			if err := decoder.Decode(&m); err != nil {
				return dir, m, err
			}
			if m.Version != 3 {
				return dir, m, diagnostic(
					"manifest_version_unsupported",
					"read project manifest",
					filepath.Join(dir, ".gin-kit.yaml"),
					fmt.Errorf("manifest version %d is not supported", m.Version),
					"Create a new project with this gin-kit release; only version 3 manifests are supported.",
				)
			}
			if err := validateManifest(m); err != nil {
				return dir, m, err
			}
			return dir, m, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", Manifest{}, errors.New("not inside a gin-kit project (.gin-kit.yaml not found)")
		}
		dir = parent
	}
}

// promptManifest performs this package operation.
func promptManifest(name string) (Manifest, error) {
	m := newInteractiveManifest(name)
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Go module path").Description("Example: github.com/you/"+name).Value(&m.Module).Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("module path is required")
				}
				return nil
			}),
			huh.NewSelect[string]().Title("Project type").Options(
				huh.NewOption("Runtime (recommended)", "runtime"),
				huh.NewOption("Standalone (includes runtime source)", "standalone"),
			).Value(&m.ProjectType),
			huh.NewSelect[string]().Title("Application mode").Options(
				huh.NewOption("API (JSON REST)", "api"), huh.NewOption("UI (HTML + HTMX)", "ui"),
			).Value(&m.Mode),
			huh.NewSelect[string]().Title("Database").Options(
				huh.NewOption("SQLite", "sqlite"), huh.NewOption("PostgreSQL", "postgres"),
				huh.NewOption("MySQL", "mysql"), huh.NewOption("MariaDB", "mariadb"),
			).Value(&m.Database),
			huh.NewSelect[string]().Title("Data access").Options(
				huh.NewOption("GORM", "gorm"), huh.NewOption("sqlx", "sqlx"),
			).Value(&m.ORM),
		),
		huh.NewGroup(
			newFeatureConfirm("Include core authentication?", &m.Auth),
			newFeatureConfirm("Include OAuth social login?", &m.OAuth),
			newFeatureConfirm("Include the guided tasks example?", &m.Example),
			newFeatureConfirm("Include Docker files?", &m.Docker),
		),
	).Run(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// newInteractiveManifest performs this package operation.
func newInteractiveManifest(name string) Manifest {
	return Manifest{
		Version:     3,
		ProjectType: "runtime",
		Project:     name,
		Mode:        "api",
		Database:    "sqlite",
		ORM:         "gorm",
		Auth:        true,
		Example:     true,
		Docker:      true,
	}
}

// newFeatureConfirm performs this package operation.
func newFeatureConfirm(title string, value *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		WithButtonAlignment(lipgloss.Left).
		Value(value)
}

// writeManifest performs this package operation.
func writeManifest(dir string, m Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".gin-kit.yaml"), data, 0o644)
}

// platform performs this package operation.
func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
