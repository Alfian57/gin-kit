package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldAPIPreservesSelections(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{
		Version: 1, Project: "sample", Module: "example.com/sample",
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
	dir := t.TempDir()
	m := Manifest{Version: 1, Project: "webapp", Module: "example.com/webapp", Mode: "ui", Database: "sqlite", ORM: "gorm"}
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
