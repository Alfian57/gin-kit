package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldAPIPreservesSelections(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sample")
	m := Manifest{
		Version: 2, Edition: "starter", Project: "sample", Module: "example.com/sample",
		Mode: "api", Database: "postgres", ORM: "sqlx", Auth: false, Example: false, Docker: true,
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"go.mod", "cmd/server/main.go", "cmd/migrate/main.go", "internal/handler/api/health.go", "api/openapi.yaml", "docker/Dockerfile"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "web/templates/index.html")); !os.IsNotExist(err) {
		t.Fatal("UI files should not be scaffolded for API mode")
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "example.com/sample") || !strings.Contains(string(data), "github.com/jmoiron/sqlx") {
		t.Fatalf("generated go.mod did not preserve module or ORM:\n%s", data)
	}
	for _, rel := range []string{
		"internal/platform/config/config.go",
		"internal/platform/database/database.go",
		"internal/middleware/security.go",
		"internal/middleware/ratelimit.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected runtime foundation %s: %v", rel, err)
		}
	}
}

func TestScaffoldUIHasEnglishLandingPage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "webapp")
	m := Manifest{Version: 2, Edition: "starter", Project: "webapp", Module: "example.com/webapp", Mode: "ui", Database: "sqlite", ORM: "gorm"}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "web/templates/index.html"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, phrase := range []string{"Build the path", "Understand every step", "Request journey"} {
		if !strings.Contains(content, phrase) {
			t.Fatalf("landing page missing %q", phrase)
		}
	}
}

func TestValidateManifestRejectsInvalidSelections(t *testing.T) {
	base := Manifest{Version: 2, Edition: "starter", Project: "sample", Module: "example.com/sample", Mode: "api", Database: "sqlite", ORM: "gorm"}
	for name, mutate := range map[string]func(*Manifest){
		"mode":         func(m *Manifest) { m.Mode = "desktop" },
		"database":     func(m *Manifest) { m.Database = "redis" },
		"orm":          func(m *Manifest) { m.ORM = "raw" },
		"project path": func(m *Manifest) { m.Project = "../escape" },
	} {
		t.Run(name, func(t *testing.T) {
			m := base
			mutate(&m)
			if err := validateManifest(m); err == nil {
				t.Fatal("expected invalid manifest to be rejected")
			}
		})
	}
}

func TestValidateManifestRejectsVersionOneWithMigrationHint(t *testing.T) {
	err := validateManifest(Manifest{Version: 1, Project: "sample", Module: "example.com/sample", Edition: "starter", Mode: "api", Database: "sqlite", ORM: "gorm"})
	if err == nil {
		t.Fatal("expected manifest version one to be rejected")
	}
}

func TestFrameworkScaffoldIsThinAndPinsRuntime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "services", "orders")
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "orders", Module: "example.com/acme/orders",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "module example.com/acme/orders") ||
		!strings.Contains(string(data), "github.com/Alfian57/gin-kit v0.3.0") {
		t.Fatalf("framework go.mod did not preserve module/runtime:\n%s", data)
	}
	for _, rel := range []string{"internal/app/app.go", "cmd/server/main.go"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("framework edition missing %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"internal/platform/config/config.go", "internal/middleware/security.go"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Fatalf("framework edition copied generic core %s", rel)
		}
	}
}

func TestFrameworkReplaceAddsLocalOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	m := Manifest{Version: 2, Edition: "framework", FrameworkVersion: "0.3.0", Project: "app", Module: "example.com/app", Mode: "api", Database: "sqlite", ORM: "gorm"}
	if err := scaffoldWithOptions(dir, m, scaffoldOptions{FrameworkReplace: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "replace github.com/Alfian57/gin-kit =>") {
		t.Fatalf("expected local framework replace:\n%s", data)
	}
}

func TestFrameworkScaffoldUsesTypedConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg")
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "cfg", Module: "example.com/cfg",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	appSource, err := os.ReadFile(filepath.Join(dir, "internal", "app", "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appSource), "config.LoadDotenv(") ||
		!strings.Contains(string(appSource), "config.Load()") {
		t.Fatalf("framework app.go does not use typed configuration:\n%s", appSource)
	}
	envExample, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envExample), "READ_TIMEOUT") ||
		strings.Contains(string(envExample), "SESSION_SECRET") {
		t.Fatalf("framework .env.example should use the framework variable set:\n%s", envExample)
	}
}

func TestFrameworkScaffoldWiresAuth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secure")
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "secure", Module: "example.com/secure",
		Mode: "api", Database: "sqlite", ORM: "gorm", Auth: true,
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	appSource, err := os.ReadFile(filepath.Join(dir, "internal", "app", "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appSource), "auth.New(") {
		t.Fatalf("framework app.go does not wire authentication:\n%s", appSource)
	}
	for _, rel := range []string{"internal/handler/api/auth_me.go", "internal/handler/api/auth_me_test.go"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("framework auth scaffold missing %s: %v", rel, err)
		}
	}

	plain := filepath.Join(t.TempDir(), "plain")
	m.Auth = false
	m.Project = "plain"
	m.Module = "example.com/plain"
	if err := scaffold(plain, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(plain, "internal", "handler", "api", "auth_me.go")); err == nil {
		t.Fatal("auth scaffold generated without --auth")
	}
}

func TestFrameworkUIIncludesAssetTooling(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "portal")
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "portal", Module: "example.com/portal",
		Mode: "ui", Database: "sqlite", ORM: "gorm",
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"package.json", "web/src/input.css", "web/templates/index.html"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("framework UI edition missing %s: %v", rel, err)
		}
	}
}
