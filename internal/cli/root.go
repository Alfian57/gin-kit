package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Version  int    `yaml:"version"`
	Module   string `yaml:"module"`
	Project  string `yaml:"project"`
	Mode     string `yaml:"mode"`
	Database string `yaml:"database"`
	ORM      string `yaml:"orm"`
	Auth     bool   `yaml:"auth"`
	Example  bool   `yaml:"example"`
	Docker   bool   `yaml:"docker"`
}

var root = &cobra.Command{
	Use:   "ginkit",
	Short: "A learning-first Go project toolkit",
}

func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	root.Version = effectiveVersion()
	root.AddCommand(newCommand(), generateCommand(), runCommand(), buildCommand(), dbCommand(), explainCommand(), doctorCommand(), checkCommand())
}

func projectRoot() (string, Manifest, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", Manifest{}, err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, ".ginkit.yaml"))
		if readErr == nil {
			var m Manifest
			if err := yaml.Unmarshal(data, &m); err != nil {
				return dir, m, err
			}
			return dir, m, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", Manifest{}, errors.New("not inside a GinKit project (.ginkit.yaml not found)")
		}
		dir = parent
	}
}

func promptManifest(name string) (Manifest, error) {
	m := Manifest{Version: 1, Project: name, Mode: "api", Database: "sqlite", ORM: "gorm"}
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Go module path").Description("Example: github.com/you/"+name).Value(&m.Module).Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("module path is required")
				}
				return nil
			}),
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
			huh.NewConfirm().Title("Include core authentication?").Affirmative("Yes").Negative("No").Value(&m.Auth),
			huh.NewConfirm().Title("Include the guided tasks example?").Affirmative("Yes").Negative("No").Value(&m.Example),
			huh.NewConfirm().Title("Include Docker files?").Affirmative("Yes").Negative("No").Value(&m.Docker),
		),
	).Run(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func writeManifest(dir string, m Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".ginkit.yaml"), data, 0o644)
}

func requireTool(name string) error {
	if _, err := os.Stat(name); err == nil {
		return nil
	}
	return nil
}

func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
