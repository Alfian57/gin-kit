package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDotenv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDotenvParsesSupportedSyntax(t *testing.T) {
	for _, key := range []string{"DOTENV_A", "DOTENV_B", "DOTENV_C", "DOTENV_D", "DOTENV_E"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	path := writeDotenv(t, strings.Join([]string{
		"# comment line",
		"",
		"DOTENV_A=plain",
		"export DOTENV_B=exported",
		`DOTENV_C="double quoted"`,
		"DOTENV_D='single quoted'",
		"DOTENV_E=key=value\r",
	}, "\n"))
	if err := LoadDotenv(path); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"DOTENV_A": "plain",
		"DOTENV_B": "exported",
		"DOTENV_C": "double quoted",
		"DOTENV_D": "single quoted",
		"DOTENV_E": "key=value",
	} {
		if got := os.Getenv(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLoadDotenvNeverOverridesEnvironment(t *testing.T) {
	t.Setenv("DOTENV_PRESET", "from-environment")
	path := writeDotenv(t, "DOTENV_PRESET=from-file\n")
	if err := LoadDotenv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DOTENV_PRESET"); got != "from-environment" {
		t.Fatalf("environment was overridden: %q", got)
	}
}

func TestLoadDotenvMissingFileIsNotAnError(t *testing.T) {
	if err := LoadDotenv(filepath.Join(t.TempDir(), "absent.env")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDotenvRejectsMalformedLines(t *testing.T) {
	for _, content := range []string{"NOT A PAIR\n", "=value\n"} {
		if err := LoadDotenv(writeDotenv(t, content)); err == nil {
			t.Fatalf("expected error for %q", content)
		}
	}
}
