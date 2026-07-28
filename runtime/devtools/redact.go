package devtools

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

// ConfigEntry is one environment variable in the devtools config report.
type ConfigEntry struct {
	// Key store data used by this type.
	Key string `json:"key"`
	// Value store data used by this type.
	Value string `json:"value"`
	// Redacted store data used by this type.
	Redacted bool `json:"redacted"`
}

// configKeys is the static allowlist of documented gin-kit environment
// variables. The config report never reads anything outside this list, so
// unrelated process environment (cloud credentials, tokens, ...) can never
// leak into the dashboard.
var configKeys = []string{
	"APP_ENV",
	"PORT",
	"DATABASE_URL",
	"JWT_SECRET",
	"SESSION_SECRET",
	"TRUSTED_PROXY_CIDRS",
	"CORS_ALLOWED_ORIGINS",
	"RATE_LIMIT_ENABLED",
	"RATE_LIMIT_PER_MINUTE",
	"RATE_LIMIT_BURST",
	"MAX_BODY_BYTES",
	"CACHE_DRIVER",
	"QUEUE_DRIVER",
	"QUEUE_CONCURRENCY",
	"REDIS_URL",
	"METRICS_ENABLED",
	"PPROF_ENABLED",
	"DOCS_ENABLED",
	"DOCS_PATH",
	"DOCS_SPEC_PATH",
	"DOCS_TITLE",
	"DOCS_VERSION",
	"DOCS_DESCRIPTION",
	"DOCS_SERVERS",
	"DOCS_BASIC_AUTH_USERNAME",
	"DOCS_BASIC_AUTH_PASSWORD",
	"DEVTOOLS_ENABLED",
	"DEVTOOLS_PATH",
	"MAIL_DRIVER",
	"MAIL_HOST",
	"MAIL_PORT",
	"MAIL_USERNAME",
	"MAIL_PASSWORD",
	"MAIL_ENCRYPTION",
	"MAIL_FROM_ADDRESS",
	"MAIL_FROM_NAME",
	"STORAGE_DRIVER",
	"STORAGE_LOCAL_ROOT",
	"STORAGE_LOCAL_BASE_URL",
	"S3_ENDPOINT",
	"S3_REGION",
	"S3_BUCKET",
	"S3_ACCESS_KEY",
	"S3_SECRET_KEY",
	"S3_USE_SSL",
	"S3_USE_PATH_STYLE",
	"S3_PRESIGN_TTL",
	"S3_PUBLIC_BASE_URL",
	"READ_TIMEOUT",
	"WRITE_TIMEOUT",
	"IDLE_TIMEOUT",
	"SHUTDOWN_TIMEOUT",
}

// secretKeyPattern define package-level implementation state.
var secretKeyPattern = regexp.MustCompile(`(?i)(SECRET|PASSWORD|KEY|TOKEN)`)

// configReport reads the allowlisted environment variables, redacting
// secret-like values entirely and scrubbing passwords out of URL userinfo.
func configReport() []ConfigEntry {
	report := make([]ConfigEntry, 0, len(configKeys))
	for _, key := range configKeys {
		value := os.Getenv(key)
		switch {
		case value == "":
			report = append(report, ConfigEntry{Key: key})
		case redactKey(key):
			report = append(report, ConfigEntry{Key: key, Value: "[redacted]", Redacted: true})
		default:
			report = append(report, ConfigEntry{Key: key, Value: scrubURL(key, value)})
		}
	}
	return report
}

// redactKey reports whether the variable's value must never be shown.
func redactKey(key string) bool { return secretKeyPattern.MatchString(key) }

// scrubURL replaces the userinfo password of URL-valued variables with ***
// so connection strings stay readable without leaking credentials.
func scrubURL(key, value string) string {
	if !strings.HasSuffix(key, "_URL") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}
	if _, hasPassword := parsed.User.Password(); !hasPassword {
		return value
	}
	// url.URL.String percent-encodes '*' in userinfo, so serialize with an
	// unreserved placeholder and swap in the literal asterisks afterwards.
	parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	return strings.Replace(parsed.String(), ":xxxxx@", ":***@", 1)
}
