package config

import (
	"strings"
	"testing"
	"time"
)

// clearEnv blanks every variable Load reads so host environments cannot leak
// into table cases.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "APP_ENV", "DATABASE_URL", "JWT_SECRET", "TRUSTED_PROXY_CIDRS",
		"CORS_ALLOWED_ORIGINS", "RATE_LIMIT_ENABLED", "RATE_LIMIT_PER_MINUTE",
		"RATE_LIMIT_BURST", "MAX_BODY_BYTES", "READ_TIMEOUT", "WRITE_TIMEOUT",
		"IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":8080" || cfg.Environment != "development" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReadTimeout != 10*time.Second || cfg.WriteTimeout != 30*time.Second ||
		cfg.IdleTimeout != 60*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected timeout defaults: %+v", cfg)
	}
	if !cfg.RateLimitEnabled || cfg.RateLimitPerMinute != 60 || cfg.RateLimitBurst != 0 {
		t.Fatalf("unexpected rate limit defaults: %+v", cfg)
	}
	if cfg.MaxBodyBytes != 1<<20 || !cfg.IsDevelopment() {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadParsesAndNormalizes(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("TRUSTED_PROXY_CIDRS", " 10.0.0.0/8 , 192.168.0.0/16 ,")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("RATE_LIMIT_ENABLED", "no")
	t.Setenv("READ_TIMEOUT", "2s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":9090" {
		t.Fatalf("PORT not normalized: %q", cfg.Address)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 || cfg.TrustedProxyCIDRs[1] != "192.168.0.0/16" {
		t.Fatalf("CSV parsing failed: %v", cfg.TrustedProxyCIDRs)
	}
	if cfg.RateLimitEnabled || cfg.ReadTimeout != 2*time.Second {
		t.Fatalf("parsed values wrong: %+v", cfg)
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	for _, test := range []struct {
		key   string
		value string
		want  string
	}{
		{"RATE_LIMIT_ENABLED", "maybe", "RATE_LIMIT_ENABLED"},
		{"RATE_LIMIT_PER_MINUTE", "sixty", "RATE_LIMIT_PER_MINUTE"},
		{"MAX_BODY_BYTES", "-1", "MAX_BODY_BYTES"},
		{"READ_TIMEOUT", "10", "READ_TIMEOUT"},
		{"SHUTDOWN_TIMEOUT", "-2s", "SHUTDOWN_TIMEOUT"},
	} {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %s error, got %v", test.want, err)
			}
		})
	}
}

func TestLoadFailsFastOutsideDevelopment(t *testing.T) {
	for _, test := range []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"missing database", map[string]string{"APP_ENV": "production"}, "DATABASE_URL"},
		{"missing cors", map[string]string{
			"APP_ENV": "production", "DATABASE_URL": "postgres://db/app",
		}, "CORS_ALLOWED_ORIGINS"},
		{"complete production", map[string]string{
			"APP_ENV": "production", "DATABASE_URL": "postgres://db/app",
			"CORS_ALLOWED_ORIGINS": "https://app.example.com",
		}, ""},
		{"development needs neither", map[string]string{"APP_ENV": "development"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearEnv(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			_, err := Load()
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %s error, got %v", test.wantErr, err)
			}
		})
	}
}

func TestOptionsMapsAllFields(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://db/app")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	t.Setenv("RATE_LIMIT_PER_MINUTE", "120")
	t.Setenv("RATE_LIMIT_BURST", "20")
	t.Setenv("MAX_BODY_BYTES", "2048")
	t.Setenv("WRITE_TIMEOUT", "45s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	options := cfg.Options()
	if options.Environment != "staging" || options.HTTP.Address != ":8080" {
		t.Fatalf("unexpected options: %+v", options)
	}
	if options.HTTP.WriteTimeout != 45*time.Second || options.HTTP.MaxBodyBytes != 2048 {
		t.Fatalf("unexpected HTTP options: %+v", options.HTTP)
	}
	if len(options.HTTP.TrustedProxies) != 1 || len(options.HTTP.CORSOrigins) != 1 {
		t.Fatalf("unexpected proxy/CORS options: %+v", options.HTTP)
	}
	if !options.HTTP.RateLimit.Enabled || options.HTTP.RateLimit.RequestsPerMinute != 120 ||
		options.HTTP.RateLimit.Burst != 20 {
		t.Fatalf("unexpected rate limit options: %+v", options.HTTP.RateLimit)
	}
}
