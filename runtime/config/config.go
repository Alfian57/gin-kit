// Package config loads and validates environment configuration for gin-kit
// runtime applications and converts it into runtime options.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Alfian57/gin-kit/runtime"
	"github.com/Alfian57/gin-kit/runtime/mail"
	"github.com/Alfian57/gin-kit/runtime/storage"
	"github.com/Alfian57/gin-kit/runtime/whatsapp"
)

// Config is the environment-shaped application configuration. Code-level
// choices such as UI mode, database dialect, and readiness checks belong on
// the runtime.Options value produced by Options.
type Config struct {
	Address               string        // PORT, "8080" or ":8080"
	Environment           string        // APP_ENV, defaults to development
	DatabaseURL           string        // DATABASE_URL
	JWTSecret             []byte        // JWT_SECRET, length is enforced by auth.New
	SessionSecret         []byte        // SESSION_SECRET, length is enforced by session.Middleware
	OAuth                 OAuthConfig   // OAUTH_* provider credentials and browser redirects
	TrustedProxyCIDRs     []string      // TRUSTED_PROXY_CIDRS, comma separated
	CORSAllowedOrigins    []string      // CORS_ALLOWED_ORIGINS, comma separated
	RateLimitEnabled      bool          // RATE_LIMIT_ENABLED, defaults to true
	RateLimitPerMinute    int           // RATE_LIMIT_PER_MINUTE, defaults to 60
	RateLimitBurst        int           // RATE_LIMIT_BURST, 0 lets the runtime derive it
	MaxBodyBytes          int64         // MAX_BODY_BYTES, defaults to 1 MiB
	CacheDriver           string        // CACHE_DRIVER, "memory" (default) or "redis"
	QueueDriver           string        // QUEUE_DRIVER, "sync" (default) or "redis"
	QueueConcurrency      int           // QUEUE_CONCURRENCY, defaults to 10
	RedisURL              string        // REDIS_URL, e.g. redis://localhost:6379/0
	MetricsEnabled        bool          // METRICS_ENABLED, defaults to false
	PProfEnabled          bool          // PPROF_ENABLED, defaults to false; never expose publicly
	DocsEnabled           bool          // DOCS_ENABLED, defaults to true only in development
	DocsPath              string        // DOCS_PATH, defaults to /docs
	DocsSpecPath          string        // DOCS_SPEC_PATH, defaults to /openapi.json
	DocsTitle             string        // DOCS_TITLE, empty lets the application inject its name
	DocsVersion           string        // DOCS_VERSION, defaults to 0.1.0
	DocsDescription       string        // DOCS_DESCRIPTION
	DocsServers           []string      // DOCS_SERVERS, comma separated
	DocsBasicAuthUser     string        // DOCS_BASIC_AUTH_USERNAME
	DocsBasicAuthPass     string        // DOCS_BASIC_AUTH_PASSWORD
	DevToolsEnabled       bool          // DEVTOOLS_ENABLED, defaults to true only in development; refused elsewhere
	DevToolsPath          string        // DEVTOOLS_PATH, defaults to /_ginkit
	MailDriver            string        // MAIL_DRIVER, "log" (default) or "smtp"
	MailHost              string        // MAIL_HOST
	MailPort              int           // MAIL_PORT, defaults per encryption
	MailUsername          string        // MAIL_USERNAME
	MailPassword          string        // MAIL_PASSWORD
	MailEncryption        string        // MAIL_ENCRYPTION, none|tls|starttls (default starttls)
	MailFromAddress       string        // MAIL_FROM_ADDRESS
	MailFromName          string        // MAIL_FROM_NAME
	WhatsAppDriver        string        // WHATSAPP_DRIVER, "log" (default) or "cloud"
	WhatsAppAccessToken   string        // WHATSAPP_ACCESS_TOKEN
	WhatsAppPhoneNumberID string        // WHATSAPP_PHONE_NUMBER_ID
	WhatsAppAPIVersion    string        // WHATSAPP_API_VERSION, for example v25.0
	WhatsAppTimeout       time.Duration // WHATSAPP_TIMEOUT, defaults to 15s
	StorageDriver         string        // STORAGE_DRIVER, "local" (default) or "s3"
	StorageLocalRoot      string        // STORAGE_LOCAL_ROOT, defaults to ./storage
	StorageLocalURL       string        // STORAGE_LOCAL_BASE_URL
	S3Endpoint            string        // S3_ENDPOINT, host[:port] without scheme
	S3Region              string        // S3_REGION
	S3Bucket              string        // S3_BUCKET
	S3AccessKey           string        // S3_ACCESS_KEY
	S3SecretKey           string        // S3_SECRET_KEY
	S3UseSSL              bool          // S3_USE_SSL, defaults to true
	S3UsePathStyle        bool          // S3_USE_PATH_STYLE, defaults to false (MinIO needs true)
	S3PresignTTL          time.Duration // S3_PRESIGN_TTL, defaults to 15m
	S3PublicBaseURL       string        // S3_PUBLIC_BASE_URL
	ReadTimeout           time.Duration // READ_TIMEOUT, Go duration syntax such as 10s
	WriteTimeout          time.Duration // WRITE_TIMEOUT
	IdleTimeout           time.Duration // IDLE_TIMEOUT
	ShutdownTimeout       time.Duration // SHUTDOWN_TIMEOUT
}

// OAuthProviderConfig holds one provider's client credentials and exact
// registered callback URL.
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OAuthConfig holds the generated application's supported social providers.
type OAuthConfig struct {
	Google          OAuthProviderConfig
	GitHub          OAuthProviderConfig
	SuccessRedirect string
	FailureRedirect string
}

// Load reads the process environment, failing fast on malformed values and on
// unsafe defaults outside development. Unset variables use safe defaults.
func Load() (Config, error) {
	cfg := Config{
		Address:       normalizeAddress(stringValue("PORT", "8080")),
		Environment:   stringValue("APP_ENV", "development"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     []byte(os.Getenv("JWT_SECRET")),
		SessionSecret: []byte(os.Getenv("SESSION_SECRET")),
		OAuth: OAuthConfig{
			Google:          OAuthProviderConfig{ClientID: os.Getenv("OAUTH_GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("OAUTH_GOOGLE_CLIENT_SECRET"), RedirectURL: os.Getenv("OAUTH_GOOGLE_REDIRECT_URL")},
			GitHub:          OAuthProviderConfig{ClientID: os.Getenv("OAUTH_GITHUB_CLIENT_ID"), ClientSecret: os.Getenv("OAUTH_GITHUB_CLIENT_SECRET"), RedirectURL: os.Getenv("OAUTH_GITHUB_REDIRECT_URL")},
			SuccessRedirect: stringValue("OAUTH_SUCCESS_REDIRECT", "/"),
			FailureRedirect: stringValue("OAUTH_FAILURE_REDIRECT", "/"),
		},
		TrustedProxyCIDRs:     splitCSV(os.Getenv("TRUSTED_PROXY_CIDRS")),
		CORSAllowedOrigins:    splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		CacheDriver:           stringValue("CACHE_DRIVER", "memory"),
		QueueDriver:           stringValue("QUEUE_DRIVER", "sync"),
		RedisURL:              os.Getenv("REDIS_URL"),
		MailDriver:            stringValue("MAIL_DRIVER", "log"),
		MailHost:              os.Getenv("MAIL_HOST"),
		MailUsername:          os.Getenv("MAIL_USERNAME"),
		MailPassword:          os.Getenv("MAIL_PASSWORD"),
		MailEncryption:        stringValue("MAIL_ENCRYPTION", "starttls"),
		MailFromAddress:       os.Getenv("MAIL_FROM_ADDRESS"),
		MailFromName:          os.Getenv("MAIL_FROM_NAME"),
		WhatsAppDriver:        stringValue("WHATSAPP_DRIVER", "log"),
		WhatsAppAccessToken:   os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
		WhatsAppAPIVersion:    os.Getenv("WHATSAPP_API_VERSION"),
		DocsPath:              stringValue("DOCS_PATH", "/docs"),
		DocsSpecPath:          stringValue("DOCS_SPEC_PATH", "/openapi.json"),
		DocsTitle:             os.Getenv("DOCS_TITLE"),
		DocsVersion:           stringValue("DOCS_VERSION", "0.1.0"),
		DocsDescription:       os.Getenv("DOCS_DESCRIPTION"),
		DocsServers:           splitCSV(os.Getenv("DOCS_SERVERS")),
		DocsBasicAuthUser:     os.Getenv("DOCS_BASIC_AUTH_USERNAME"),
		DocsBasicAuthPass:     os.Getenv("DOCS_BASIC_AUTH_PASSWORD"),
		DevToolsPath:          stringValue("DEVTOOLS_PATH", "/_ginkit"),
		StorageDriver:         stringValue("STORAGE_DRIVER", "local"),
		StorageLocalRoot:      stringValue("STORAGE_LOCAL_ROOT", "./storage"),
		StorageLocalURL:       os.Getenv("STORAGE_LOCAL_BASE_URL"),
		S3Endpoint:            os.Getenv("S3_ENDPOINT"),
		S3Region:              os.Getenv("S3_REGION"),
		S3Bucket:              os.Getenv("S3_BUCKET"),
		S3AccessKey:           os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:           os.Getenv("S3_SECRET_KEY"),
		S3PublicBaseURL:       os.Getenv("S3_PUBLIC_BASE_URL"),
	}
	var err error
	if cfg.RateLimitEnabled, err = boolValue("RATE_LIMIT_ENABLED", true); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitPerMinute, err = intValue("RATE_LIMIT_PER_MINUTE", 60); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitBurst, err = intValue("RATE_LIMIT_BURST", 0); err != nil {
		return Config{}, err
	}
	maxBody, err := intValue("MAX_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxBodyBytes = int64(maxBody)
	if cfg.QueueConcurrency, err = intValue("QUEUE_CONCURRENCY", 10); err != nil {
		return Config{}, err
	}
	if cfg.MetricsEnabled, err = boolValue("METRICS_ENABLED", false); err != nil {
		return Config{}, err
	}
	if cfg.PProfEnabled, err = boolValue("PPROF_ENABLED", false); err != nil {
		return Config{}, err
	}
	if cfg.DocsEnabled, err = boolValue("DOCS_ENABLED", cfg.Environment == "development"); err != nil {
		return Config{}, err
	}
	if cfg.DevToolsEnabled, err = boolValue("DEVTOOLS_ENABLED", cfg.Environment == "development"); err != nil {
		return Config{}, err
	}
	if cfg.MailPort, err = intValue("MAIL_PORT", 0); err != nil {
		return Config{}, err
	}
	if cfg.S3UseSSL, err = boolValue("S3_USE_SSL", true); err != nil {
		return Config{}, err
	}
	if cfg.S3UsePathStyle, err = boolValue("S3_USE_PATH_STYLE", false); err != nil {
		return Config{}, err
	}
	if cfg.S3PresignTTL, err = durationValue("S3_PRESIGN_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.WhatsAppTimeout, err = durationValue("WHATSAPP_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = durationValue("READ_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = durationValue("WRITE_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = durationValue("IDLE_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationValue("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.Environment != "development" {
		if c.DatabaseURL == "" {
			return errors.New("DATABASE_URL is required outside development")
		}
		if len(c.CORSAllowedOrigins) == 0 {
			return errors.New("CORS_ALLOWED_ORIGINS is required outside development")
		}
	}
	if c.DatabaseURL != "" {
		if _, err := url.Parse(c.DatabaseURL); err != nil {
			return fmt.Errorf("invalid DATABASE_URL: %w", err)
		}
	}
	if c.MaxBodyBytes <= 0 {
		return errors.New("MAX_BODY_BYTES must be positive")
	}
	if err := c.OAuth.Validate(); err != nil {
		return err
	}
	if c.RateLimitEnabled && c.RateLimitPerMinute <= 0 {
		return errors.New("RATE_LIMIT_PER_MINUTE must be positive when rate limiting is enabled")
	}
	if c.RateLimitBurst < 0 {
		return errors.New("RATE_LIMIT_BURST must not be negative")
	}
	switch c.CacheDriver {
	case "memory":
	case "redis":
		if c.RedisURL == "" {
			return errors.New("REDIS_URL is required when CACHE_DRIVER is redis")
		}
	default:
		return fmt.Errorf("CACHE_DRIVER must be memory or redis, got %q", c.CacheDriver)
	}
	switch c.QueueDriver {
	case "sync":
	case "redis":
		if c.RedisURL == "" {
			return errors.New("REDIS_URL is required when QUEUE_DRIVER is redis")
		}
	default:
		return fmt.Errorf("QUEUE_DRIVER must be sync or redis, got %q", c.QueueDriver)
	}
	if c.QueueConcurrency < 1 {
		return errors.New("QUEUE_CONCURRENCY must be positive")
	}
	switch c.MailDriver {
	case "log":
	case "smtp":
		if c.MailHost == "" || c.MailFromAddress == "" {
			return errors.New("MAIL_HOST and MAIL_FROM_ADDRESS are required when MAIL_DRIVER is smtp")
		}
	default:
		return fmt.Errorf("MAIL_DRIVER must be log or smtp, got %q", c.MailDriver)
	}
	switch c.MailEncryption {
	case "none", "tls", "starttls":
	default:
		return fmt.Errorf("MAIL_ENCRYPTION must be none, tls, or starttls, got %q", c.MailEncryption)
	}
	if _, err := whatsapp.New(c.WhatsAppOptions()); err != nil {
		return fmt.Errorf("WHATSAPP_DRIVER: %w", err)
	}
	switch c.StorageDriver {
	case "local", "s3":
	default:
		return fmt.Errorf("STORAGE_DRIVER must be local or s3, got %q", c.StorageDriver)
	}
	if (c.DocsBasicAuthUser == "") != (c.DocsBasicAuthPass == "") {
		return errors.New("DOCS_BASIC_AUTH_USERNAME and DOCS_BASIC_AUTH_PASSWORD must be set together")
	}
	if !strings.HasPrefix(c.DocsPath, "/") || !strings.HasPrefix(c.DocsSpecPath, "/") {
		return errors.New("DOCS_PATH and DOCS_SPEC_PATH must start with /")
	}
	if c.DocsPath == c.DocsSpecPath {
		return errors.New("DOCS_PATH and DOCS_SPEC_PATH must differ")
	}
	if c.DevToolsEnabled && c.Environment != "development" {
		return errors.New("DEVTOOLS_ENABLED must not be set outside development; the dashboard exposes requests, mail, and configuration")
	}
	if !strings.HasPrefix(c.DevToolsPath, "/") {
		return errors.New("DEVTOOLS_PATH must start with /")
	}
	if c.RedisURL != "" {
		parsed, err := url.Parse(c.RedisURL)
		if err != nil {
			return fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		switch parsed.Scheme {
		case "redis", "rediss", "unix":
		default:
			return fmt.Errorf("REDIS_URL must use the redis, rediss, or unix scheme, got %q", parsed.Scheme)
		}
	}
	for _, timeout := range []struct {
		name  string
		value time.Duration
	}{
		{"READ_TIMEOUT", c.ReadTimeout},
		{"WRITE_TIMEOUT", c.WriteTimeout},
		{"IDLE_TIMEOUT", c.IdleTimeout},
		{"SHUTDOWN_TIMEOUT", c.ShutdownTimeout},
	} {
		if timeout.value <= 0 {
			return fmt.Errorf("%s must be positive", timeout.name)
		}
	}
	return nil
}

// Validate rejects partial provider credentials and unsafe browser redirects.
func (c OAuthConfig) Validate() error {
	for _, provider := range []OAuthProviderConfig{c.Google, c.GitHub} {
		populated := provider.ClientID != "" || provider.ClientSecret != "" || provider.RedirectURL != ""
		if populated && (provider.ClientID == "" || provider.ClientSecret == "" || provider.RedirectURL == "") {
			return errors.New("each OAuth provider requires client ID, client secret, and redirect URL")
		}
	}
	for _, path := range []string{c.SuccessRedirect, c.FailureRedirect} {
		if !safeRedirectPath(path) {
			return errors.New("OAUTH_SUCCESS_REDIRECT and OAUTH_FAILURE_REDIRECT must be relative paths")
		}
	}
	return nil
}

func safeRedirectPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "\r\n")
}

func (c Config) IsDevelopment() bool { return c.Environment == "development" }

// Options converts the configuration into runtime options. Set code-level
// fields (UI, Database, Validator, Readiness, ...) on the result before
// calling runtime.New.
func (c Config) Options() runtime.Options {
	return runtime.Options{
		Environment: c.Environment,
		HTTP: runtime.HTTPOptions{
			Address:         c.Address,
			ReadTimeout:     c.ReadTimeout,
			WriteTimeout:    c.WriteTimeout,
			IdleTimeout:     c.IdleTimeout,
			ShutdownTimeout: c.ShutdownTimeout,
			MaxBodyBytes:    c.MaxBodyBytes,
			CORSOrigins:     c.CORSAllowedOrigins,
			TrustedProxies:  c.TrustedProxyCIDRs,
			RateLimit: runtime.RateLimitOptions{
				Enabled:           c.RateLimitEnabled,
				RequestsPerMinute: c.RateLimitPerMinute,
				Burst:             c.RateLimitBurst,
			},
		},
		Metrics: runtime.MetricsOptions{Enabled: c.MetricsEnabled},
		PProf:   runtime.PProfOptions{Enabled: c.PProfEnabled},
		Docs: runtime.DocsOptions{
			Enabled:           c.DocsEnabled,
			Path:              c.DocsPath,
			SpecPath:          c.DocsSpecPath,
			Title:             c.DocsTitle,
			Version:           c.DocsVersion,
			Description:       c.DocsDescription,
			Servers:           c.DocsServers,
			BasicAuthUsername: c.DocsBasicAuthUser,
			BasicAuthPassword: c.DocsBasicAuthPass,
		},
		DevTools: runtime.DevToolsOptions{
			Enabled: c.DevToolsEnabled,
			Path:    c.DevToolsPath,
		},
		Cache: runtime.CacheOptions{Driver: c.CacheDriver, RedisURL: c.RedisURL},
		Queue: runtime.QueueOptions{
			Driver:      c.QueueDriver,
			RedisURL:    c.RedisURL,
			Concurrency: c.QueueConcurrency,
		},
	}
}

// MailOptions converts the mail configuration for mail.New. The mailer is
// constructed by application code, not by runtime.New.
func (c Config) MailOptions() mail.Options {
	return mail.Options{
		Driver:      c.MailDriver,
		Host:        c.MailHost,
		Port:        c.MailPort,
		Username:    c.MailUsername,
		Password:    c.MailPassword,
		Encryption:  mail.Encryption(c.MailEncryption),
		FromAddress: c.MailFromAddress,
		FromName:    c.MailFromName,
	}
}

// WhatsAppOptions converts WhatsApp environment configuration for
// whatsapp.New. The client is constructed by application code, not runtime.New.
func (c Config) WhatsAppOptions() whatsapp.Options {
	return whatsapp.Options{
		Driver:        c.WhatsAppDriver,
		AccessToken:   c.WhatsAppAccessToken,
		PhoneNumberID: c.WhatsAppPhoneNumberID,
		APIVersion:    c.WhatsAppAPIVersion,
		Timeout:       c.WhatsAppTimeout,
	}
}

// StorageOptions converts the storage configuration for storage.New. The
// disk is constructed by application code, not by runtime.New.
func (c Config) StorageOptions() storage.Options {
	useSSL := c.S3UseSSL
	return storage.Options{
		Driver: c.StorageDriver,
		Local: storage.LocalOptions{
			Root:    c.StorageLocalRoot,
			BaseURL: c.StorageLocalURL,
		},
		S3: storage.S3Options{
			Endpoint:      c.S3Endpoint,
			Region:        c.S3Region,
			Bucket:        c.S3Bucket,
			AccessKey:     c.S3AccessKey,
			SecretKey:     c.S3SecretKey,
			UseSSL:        &useSSL,
			UsePathStyle:  c.S3UsePathStyle,
			PresignTTL:    c.S3PresignTTL,
			PublicBaseURL: c.S3PublicBaseURL,
		},
	}
}

func stringValue(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func normalizeAddress(port string) string {
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

func splitCSV(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func boolValue(key string, fallback bool) (bool, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch raw {
	case "":
		return fallback, nil
	case "yes":
		return true, nil
	case "no":
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q", key, raw)
	}
	return value, nil
}

func intValue(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return value, nil
}

func durationValue(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration such as 10s, got %q", key, raw)
	}
	return value, nil
}
