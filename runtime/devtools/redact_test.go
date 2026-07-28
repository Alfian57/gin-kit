package devtools

import (
	"testing"
)

// findEntry performs this package operation.
func findEntry(t *testing.T, report []ConfigEntry, key string) ConfigEntry {
	t.Helper()
	for _, entry := range report {
		if entry.Key == key {
			return entry
		}
	}
	t.Fatalf("key %s missing from the config report", key)
	return ConfigEntry{}
}

func TestConfigReportRedactsSecretKeys(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("MAIL_PASSWORD", "hunter2")
	t.Setenv("S3_ACCESS_KEY", "AKIAEXAMPLE")
	t.Setenv("DOCS_BASIC_AUTH_PASSWORD", "docspass")
	report := configReport()
	for _, key := range []string{"JWT_SECRET", "MAIL_PASSWORD", "S3_ACCESS_KEY", "DOCS_BASIC_AUTH_PASSWORD"} {
		entry := findEntry(t, report, key)
		if !entry.Redacted || entry.Value != "[redacted]" {
			t.Fatalf("%s not redacted: %+v", key, entry)
		}
	}
}

func TestConfigReportScrubsURLPasswords(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:supersecret@db.internal:5432/app?sslmode=disable")
	entry := findEntry(t, configReport(), "DATABASE_URL")
	if entry.Redacted {
		t.Fatalf("DATABASE_URL should be scrubbed, not redacted: %+v", entry)
	}
	if entry.Value != "postgres://app:***@db.internal:5432/app?sslmode=disable" {
		t.Fatalf("password not scrubbed: %q", entry.Value)
	}
}

func TestConfigReportLeavesPlainValuesUntouched(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "8080")
	report := configReport()
	if entry := findEntry(t, report, "APP_ENV"); entry.Value != "development" || entry.Redacted {
		t.Fatalf("APP_ENV altered: %+v", entry)
	}
	if entry := findEntry(t, report, "PORT"); entry.Value != "8080" || entry.Redacted {
		t.Fatalf("PORT altered: %+v", entry)
	}
}
