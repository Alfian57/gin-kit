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

type Manifest struct {
	Version        int    `yaml:"version"`
	ProjectType    string `yaml:"project_type"`
	RuntimeVersion string `yaml:"runtime_version,omitempty"`
	Module         string `yaml:"module"`
	Project        string `yaml:"project"`
	Mode           string `yaml:"mode"`
	Database       string `yaml:"database"`
	ORM            string `yaml:"orm"`
	Auth           bool   `yaml:"auth"`
	OAuth          bool   `yaml:"oauth"`
	Example        bool   `yaml:"example"`
	Docker         bool   `yaml:"docker"`
}

var root = &cobra.Command{
	Use:   "gin-kit",
	Short: "An opinionated application framework built on Gin",
}

func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	root.Version = effectiveVersion()
	root.AddCommand(newCommand(), generateCommand(), runCommand(), devCommand(), buildCommand(), dbCommand(), routesCommand(), upgradeCommand(), explainCommand(), doctorCommand(), checkCommand())
}

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

func newFeatureConfirm(title string, value *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		WithButtonAlignment(lipgloss.Left).
		Value(value)
}

func writeManifest(dir string, m Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".gin-kit.yaml"), data, 0o644)
}

func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
