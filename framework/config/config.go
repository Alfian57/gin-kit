// Package config loads and validates environment configuration for gin-kit
// framework applications and converts it into framework options.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Alfian57/gin-kit/framework"
)

// Config is the environment-shaped application configuration. Code-level
// choices such as UI mode, database dialect, and readiness checks belong on
// the framework.Options value produced by Options.
type Config struct {
	Address            string        // PORT, "8080" or ":8080"
	Environment        string        // APP_ENV, defaults to development
	DatabaseURL        string        // DATABASE_URL
	JWTSecret          []byte        // JWT_SECRET, length is enforced by auth.New
	TrustedProxyCIDRs  []string      // TRUSTED_PROXY_CIDRS, comma separated
	CORSAllowedOrigins []string      // CORS_ALLOWED_ORIGINS, comma separated
	RateLimitEnabled   bool          // RATE_LIMIT_ENABLED, defaults to true
	RateLimitPerMinute int           // RATE_LIMIT_PER_MINUTE, defaults to 60
	RateLimitBurst     int           // RATE_LIMIT_BURST, 0 lets the framework derive it
	MaxBodyBytes       int64         // MAX_BODY_BYTES, defaults to 1 MiB
	CacheDriver        string        // CACHE_DRIVER, "memory" (default) or "redis"
	QueueDriver        string        // QUEUE_DRIVER, "sync" (default) or "redis"
	QueueConcurrency   int           // QUEUE_CONCURRENCY, defaults to 10
	RedisURL           string        // REDIS_URL, e.g. redis://localhost:6379/0
	MetricsEnabled     bool          // METRICS_ENABLED, defaults to false
	PProfEnabled       bool          // PPROF_ENABLED, defaults to false; never expose publicly
	ReadTimeout        time.Duration // READ_TIMEOUT, Go duration syntax such as 10s
	WriteTimeout       time.Duration // WRITE_TIMEOUT
	IdleTimeout        time.Duration // IDLE_TIMEOUT
	ShutdownTimeout    time.Duration // SHUTDOWN_TIMEOUT
}

// Load reads the process environment, failing fast on malformed values and on
// unsafe defaults outside development. Unset variables use safe defaults.
func Load() (Config, error) {
	cfg := Config{
		Address:            normalizeAddress(stringValue("PORT", "8080")),
		Environment:        stringValue("APP_ENV", "development"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          []byte(os.Getenv("JWT_SECRET")),
		TrustedProxyCIDRs:  splitCSV(os.Getenv("TRUSTED_PROXY_CIDRS")),
		CORSAllowedOrigins: splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		CacheDriver:        stringValue("CACHE_DRIVER", "memory"),
		QueueDriver:        stringValue("QUEUE_DRIVER", "sync"),
		RedisURL:           os.Getenv("REDIS_URL"),
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

func (c Config) IsDevelopment() bool { return c.Environment == "development" }

// Options converts the configuration into framework options. Set code-level
// fields (UI, Database, Validator, Readiness, ...) on the result before
// calling framework.New.
func (c Config) Options() framework.Options {
	return framework.Options{
		Environment: c.Environment,
		HTTP: framework.HTTPOptions{
			Address:         c.Address,
			ReadTimeout:     c.ReadTimeout,
			WriteTimeout:    c.WriteTimeout,
			IdleTimeout:     c.IdleTimeout,
			ShutdownTimeout: c.ShutdownTimeout,
			MaxBodyBytes:    c.MaxBodyBytes,
			CORSOrigins:     c.CORSAllowedOrigins,
			TrustedProxies:  c.TrustedProxyCIDRs,
			RateLimit: framework.RateLimitOptions{
				Enabled:           c.RateLimitEnabled,
				RequestsPerMinute: c.RateLimitPerMinute,
				Burst:             c.RateLimitBurst,
			},
		},
		Metrics: framework.MetricsOptions{Enabled: c.MetricsEnabled},
		PProf:   framework.PProfOptions{Enabled: c.PProfEnabled},
		Cache:   framework.CacheOptions{Driver: c.CacheDriver, RedisURL: c.RedisURL},
		Queue: framework.QueueOptions{
			Driver:      c.QueueDriver,
			RedisURL:    c.RedisURL,
			Concurrency: c.QueueConcurrency,
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
