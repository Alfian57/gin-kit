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
		"OAUTH_GOOGLE_CLIENT_ID", "OAUTH_GOOGLE_CLIENT_SECRET", "OAUTH_GOOGLE_REDIRECT_URL",
		"OAUTH_GITHUB_CLIENT_ID", "OAUTH_GITHUB_CLIENT_SECRET", "OAUTH_GITHUB_REDIRECT_URL",
		"OAUTH_SUCCESS_REDIRECT", "OAUTH_FAILURE_REDIRECT",
		"CORS_ALLOWED_ORIGINS", "RATE_LIMIT_ENABLED", "RATE_LIMIT_PER_MINUTE",
		"RATE_LIMIT_BURST", "MAX_BODY_BYTES", "METRICS_ENABLED", "PPROF_ENABLED",
		"CACHE_DRIVER", "REDIS_URL", "QUEUE_DRIVER", "QUEUE_CONCURRENCY",
		"DOCS_ENABLED", "DOCS_PATH", "DOCS_SPEC_PATH", "DOCS_TITLE", "DOCS_VERSION",
		"DOCS_DESCRIPTION", "DOCS_SERVERS", "DOCS_BASIC_AUTH_USERNAME", "DOCS_BASIC_AUTH_PASSWORD",
		"DEVTOOLS_ENABLED", "DEVTOOLS_PATH",
		"MAIL_DRIVER", "MAIL_HOST", "MAIL_PORT", "MAIL_USERNAME", "MAIL_PASSWORD",
		"MAIL_ENCRYPTION", "MAIL_FROM_ADDRESS", "MAIL_FROM_NAME",
		"STORAGE_DRIVER", "STORAGE_LOCAL_ROOT", "STORAGE_LOCAL_BASE_URL",
		"S3_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"S3_USE_SSL", "S3_USE_PATH_STYLE", "S3_PRESIGN_TTL", "S3_PUBLIC_BASE_URL",
		"READ_TIMEOUT", "WRITE_TIMEOUT", "IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT",
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

func TestLoadOAuthConfiguration(t *testing.T) {
	clearEnv(t)
	t.Setenv("OAUTH_GOOGLE_CLIENT_ID", "google-client")
	t.Setenv("OAUTH_GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("OAUTH_GOOGLE_REDIRECT_URL", "https://app.example/auth/oauth/google/callback")
	t.Setenv("OAUTH_SUCCESS_REDIRECT", "/account")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuth.Google.ClientID != "google-client" || cfg.OAuth.SuccessRedirect != "/account" || cfg.OAuth.FailureRedirect != "/" {
		t.Fatalf("OAuth configuration was not loaded: %+v", cfg.OAuth)
	}

	clearEnv(t)
	t.Setenv("OAUTH_GITHUB_CLIENT_ID", "github-client")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "each OAuth provider") {
		t.Fatalf("partial OAuth provider configuration was accepted: %v", err)
	}

	clearEnv(t)
	t.Setenv("OAUTH_SUCCESS_REDIRECT", "https://attacker.example")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "OAUTH_SUCCESS_REDIRECT") {
		t.Fatalf("external OAuth redirect was accepted: %v", err)
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	for _, test := range []struct {
		key   string
		value string
		want  string
	}{
		{"CACHE_DRIVER", "memcached", "CACHE_DRIVER"},
		{"QUEUE_DRIVER", "kafka", "QUEUE_DRIVER"},
		{"QUEUE_CONCURRENCY", "0", "QUEUE_CONCURRENCY"},
		{"MAIL_DRIVER", "pigeon", "MAIL_DRIVER"},
		{"MAIL_ENCRYPTION", "quantum", "MAIL_ENCRYPTION"},
		{"STORAGE_DRIVER", "ftp", "STORAGE_DRIVER"},
		{"S3_PRESIGN_TTL", "soon", "S3_PRESIGN_TTL"},
		{"REDIS_URL", "http://not-redis", "REDIS_URL"},
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
	t.Setenv("METRICS_ENABLED", "true")
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
	if !options.Metrics.Enabled || options.PProf.Enabled {
		t.Fatalf("unexpected observability options: %+v %+v", options.Metrics, options.PProf)
	}
}

func TestLoadCacheConfiguration(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil || cfg.CacheDriver != "memory" {
		t.Fatalf("cache defaults: %+v err=%v", cfg.CacheDriver, err)
	}

	clearEnv(t)
	t.Setenv("CACHE_DRIVER", "redis")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("redis driver without URL accepted: %v", err)
	}

	clearEnv(t)
	t.Setenv("CACHE_DRIVER", "redis")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	options := cfg.Options()
	if options.Cache.Driver != "redis" || options.Cache.RedisURL != "redis://localhost:6379/0" {
		t.Fatalf("cache options not mapped: %+v", options.Cache)
	}
}

func TestMailAndStorageOptionMapping(t *testing.T) {
	clearEnv(t)
	t.Setenv("MAIL_DRIVER", "smtp")
	t.Setenv("MAIL_HOST", "smtp.example.com")
	t.Setenv("MAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("STORAGE_DRIVER", "s3")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_BUCKET", "uploads")
	t.Setenv("S3_USE_PATH_STYLE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	mailOptions := cfg.MailOptions()
	if mailOptions.Driver != "smtp" || mailOptions.Host != "smtp.example.com" ||
		mailOptions.FromAddress != "noreply@example.com" || string(mailOptions.Encryption) != "starttls" {
		t.Fatalf("mail options not mapped: %+v", mailOptions)
	}
	storageOptions := cfg.StorageOptions()
	if storageOptions.Driver != "s3" || storageOptions.S3.Endpoint != "minio:9000" ||
		storageOptions.S3.Bucket != "uploads" || !storageOptions.S3.UsePathStyle ||
		storageOptions.S3.UseSSL == nil || !*storageOptions.S3.UseSSL {
		t.Fatalf("storage options not mapped: %+v", storageOptions)
	}

	clearEnv(t)
	t.Setenv("MAIL_DRIVER", "smtp")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MAIL_HOST") {
		t.Fatalf("smtp without host accepted: %v", err)
	}
}

func TestDocsConfiguration(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil || !cfg.DocsEnabled {
		t.Fatalf("docs should default on in development: enabled=%v err=%v", cfg.DocsEnabled, err)
	}
	options := cfg.Options()
	if !options.Docs.Enabled || options.Docs.Path != "/docs" || options.Docs.SpecPath != "/openapi.json" {
		t.Fatalf("docs options not mapped: %+v", options.Docs)
	}

	clearEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://db/app")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	cfg, err = Load()
	if err != nil || cfg.DocsEnabled {
		t.Fatalf("docs should default off in production: enabled=%v err=%v", cfg.DocsEnabled, err)
	}
	t.Setenv("DOCS_ENABLED", "true")
	t.Setenv("DOCS_BASIC_AUTH_USERNAME", "admin")
	t.Setenv("DOCS_BASIC_AUTH_PASSWORD", "secret")
	t.Setenv("DOCS_SERVERS", "https://api.example.com, https://staging.example.com")
	cfg, err = Load()
	if err != nil || !cfg.DocsEnabled || len(cfg.DocsServers) != 2 {
		t.Fatalf("docs production opt-in failed: %+v err=%v", cfg, err)
	}

	clearEnv(t)
	t.Setenv("DOCS_BASIC_AUTH_USERNAME", "admin")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DOCS_BASIC_AUTH") {
		t.Fatalf("half basic auth accepted: %v", err)
	}

	clearEnv(t)
	t.Setenv("DOCS_PATH", "docs")
	if _, err := Load(); err == nil {
		t.Fatal("relative docs path accepted")
	}

	clearEnv(t)
	t.Setenv("DOCS_SPEC_PATH", "/docs")
	if _, err := Load(); err == nil {
		t.Fatal("identical docs paths accepted")
	}
}

func TestDevToolsConfiguration(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil || !cfg.DevToolsEnabled || cfg.DevToolsPath != "/_ginkit" {
		t.Fatalf("devtools should default on in development: %+v err=%v", cfg, err)
	}
	options := cfg.Options()
	if !options.DevTools.Enabled || options.DevTools.Path != "/_ginkit" {
		t.Fatalf("devtools options not mapped: %+v", options.DevTools)
	}

	clearEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://db/app")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	cfg, err = Load()
	if err != nil || cfg.DevToolsEnabled {
		t.Fatalf("devtools should default off in production: enabled=%v err=%v", cfg.DevToolsEnabled, err)
	}

	t.Setenv("DEVTOOLS_ENABLED", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DEVTOOLS_ENABLED") {
		t.Fatalf("devtools enabled in production accepted: %v", err)
	}

	clearEnv(t)
	t.Setenv("DEVTOOLS_PATH", "_ginkit")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DEVTOOLS_PATH") {
		t.Fatalf("relative devtools path accepted: %v", err)
	}
}

func TestLoadQueueConfiguration(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil || cfg.QueueDriver != "sync" || cfg.QueueConcurrency != 10 {
		t.Fatalf("queue defaults: driver=%q concurrency=%d err=%v", cfg.QueueDriver, cfg.QueueConcurrency, err)
	}

	clearEnv(t)
	t.Setenv("QUEUE_DRIVER", "redis")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("redis queue without URL accepted: %v", err)
	}

	clearEnv(t)
	t.Setenv("QUEUE_DRIVER", "redis")
	t.Setenv("QUEUE_CONCURRENCY", "4")
	t.Setenv("REDIS_URL", "redis://localhost:6379/1")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	options := cfg.Options()
	if options.Queue.Driver != "redis" || options.Queue.Concurrency != 4 ||
		options.Queue.RedisURL != "redis://localhost:6379/1" {
		t.Fatalf("queue options not mapped: %+v", options.Queue)
	}
}
