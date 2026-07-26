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
		"internal/platform/query/query.go",
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
	if !strings.Contains(string(appSource), "jobs.Register(") {
		t.Fatalf("framework app.go does not register jobs:\n%s", appSource)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "jobs", "ping.go")); err != nil {
		t.Fatalf("framework scaffold missing jobs example: %v", err)
	}
	envExample, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envExample), "READ_TIMEOUT") ||
		strings.Contains(string(envExample), "RATE_LIMIT_GENERAL_PER_MINUTE") {
		t.Fatalf("framework .env.example should use the framework variable set:\n%s", envExample)
	}
}

func TestComposeRedisServiceIsFrameworkOnly(t *testing.T) {
	frameworkDir := filepath.Join(t.TempDir(), "fw")
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "fw", Module: "example.com/fw",
		Mode: "api", Database: "postgres", ORM: "gorm", Docker: true,
	}
	if err := scaffold(frameworkDir, m); err != nil {
		t.Fatal(err)
	}
	compose, err := os.ReadFile(filepath.Join(frameworkDir, "docker", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), "redis:") {
		t.Fatalf("framework compose missing redis service:\n%s", compose)
	}

	starterDir := filepath.Join(t.TempDir(), "st")
	m.Edition = "starter"
	m.FrameworkVersion = ""
	m.Project = "st"
	m.Module = "example.com/st"
	if err := scaffold(starterDir, m); err != nil {
		t.Fatal(err)
	}
	compose, err = os.ReadFile(filepath.Join(starterDir, "docker", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(compose), "redis:") {
		t.Fatalf("starter compose unexpectedly gained redis:\n%s", compose)
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
	if !strings.Contains(string(appSource), "auth.New(") ||
		!strings.Contains(string(appSource), "service.NewAuthService(") {
		t.Fatalf("framework app.go does not wire authentication:\n%s", appSource)
	}
	for _, rel := range []string{
		"internal/domain/auth_user.go",
		"internal/repository/auth_repository.go",
		"internal/service/auth_service.go",
		"internal/service/auth_service_test.go",
		"internal/handler/auth/auth_handlers.go",
		"internal/handler/auth/auth_handlers_test.go",
		"migrations/00002_auth_init.sql",
	} {
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
	for _, rel := range []string{"internal/handler/auth/auth_handlers.go", "migrations/00002_auth_init.sql"} {
		if _, err := os.Stat(filepath.Join(plain, rel)); err == nil {
			t.Fatalf("auth scaffold %s generated without --auth", rel)
		}
	}
}

func TestPlatformSessionIsStarterUIOnly(t *testing.T) {
	ui := filepath.Join(t.TempDir(), "webapp")
	m := Manifest{Version: 2, Edition: "starter", Project: "webapp", Module: "example.com/webapp", Mode: "ui", Database: "sqlite", ORM: "gorm"}
	if err := scaffold(ui, m); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"internal/platform/session/session.go",
		"internal/platform/session/csrf.go",
		"internal/platform/session/flash.go",
	} {
		if _, err := os.Stat(filepath.Join(ui, rel)); err != nil {
			t.Fatalf("starter UI missing %s: %v", rel, err)
		}
	}
	goMod, err := os.ReadFile(filepath.Join(ui, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "gin-contrib/sessions") {
		t.Fatalf("starter UI go.mod missing sessions dependency:\n%s", goMod)
	}

	apiDir := filepath.Join(t.TempDir(), "apionly")
	m.Mode = "api"
	m.ORM = "sqlx"
	m.Project = "apionly"
	m.Module = "example.com/apionly"
	if err := scaffold(apiDir, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(apiDir, "internal", "platform", "session")); err == nil {
		t.Fatal("platform session generated for api mode")
	}
}

func TestStarterScaffoldWiresAuth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secure")
	m := Manifest{
		Version: 2, Edition: "starter",
		Project: "secure", Module: "example.com/secure",
		Mode: "api", Database: "sqlite", ORM: "sqlx", Auth: true,
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"internal/domain/auth_user.go",
		"internal/repository/auth_repository.go",
		"internal/service/auth_service.go",
		"internal/handler/auth/auth_handlers.go",
		"migrations/00003_auth_init.sql",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("starter auth scaffold missing %s: %v", rel, err)
		}
	}
	healthSource, err := os.ReadFile(filepath.Join(dir, "internal", "handler", "api", "health.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(healthSource), "authhandler.Register(") {
		t.Fatalf("starter API register does not wire auth:\n%s", healthSource)
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
