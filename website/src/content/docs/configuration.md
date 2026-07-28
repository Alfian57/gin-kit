---
title: Configuration
description: Every environment variable, in one place.
---

Generated projects load environment variables through typed configuration.
Required database credentials, CORS origins, and authentication secrets are
validated before the server starts — malformed values are startup errors, not
runtime surprises.

Runtime applications use the `runtime/config` package:

```go
if err := config.LoadDotenv(".env"); err != nil {
    return nil, err
}
cfg, err := config.Load()
if err != nil {
    return nil, err
}
options := cfg.Options() // then set UI, Database, Readiness, ...
```

`LoadDotenv` reads `KEY=VALUE` pairs and never overrides real environment
variables, so the environment always wins over `.env`. Keep `.env` local and
commit only `.env.example`. Outside development, `DATABASE_URL` and
`CORS_ALLOWED_ORIGINS` are required, and `JWT_SECRET`/`SESSION_SECRET` are
required when the feature that consumes them is enabled.

## Reference

All variables read by `config.Load()` in runtime projects, grouped
by subsystem. Durations use Go syntax (`15s`, `2m`).

### Core

| Variable | Default | Notes |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port. |
| `APP_ENV` | `development` | `development`, `staging`, or `production`; gates safety checks and docs. |
| `DATABASE_URL` | sqlite file (dev) | Required outside development. |
| `TRUSTED_PROXY_CIDRS` | *(empty — trust none)* | Comma-separated CIDRs; forwarded IP headers are honored only from these. |
| `CORS_ALLOWED_ORIGINS` | *(dev default)* | Required outside development. |
| `MAX_BODY_BYTES` | `1048576` | Oversized bodies answer `413 body_too_large`. |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` / `SHUTDOWN_TIMEOUT` | sane defaults | Go durations. |

### Auth and sessions

| Variable | Notes |
| --- | --- |
| `JWT_SECRET` | Required with `--auth`; short secrets fail startup. |
| `SESSION_SECRET` | Required for UI-mode cookie sessions. |

### OAuth social sign-in

These variables are used by projects created with `--auth --oauth`. Set every
credential in one provider row or leave that provider entirely empty. Redirect
targets are local paths only.

| Variable | Default | Notes |
| --- | --- | --- |
| `OAUTH_GOOGLE_CLIENT_ID` / `OAUTH_GOOGLE_CLIENT_SECRET` / `OAUTH_GOOGLE_REDIRECT_URL` | — | Google OAuth credentials and exact registered callback URL. |
| `OAUTH_GITHUB_CLIENT_ID` / `OAUTH_GITHUB_CLIENT_SECRET` / `OAUTH_GITHUB_REDIRECT_URL` | — | GitHub OAuth credentials and exact registered callback URL. |
| `OAUTH_SUCCESS_REDIRECT` | `/` | Relative path after successful sign-in. |
| `OAUTH_FAILURE_REDIRECT` | `/` | Relative path after a failed or unavailable sign-in. |

See [OAuth social sign-in](/gin-kit/oauth/) for the routes and provider
configuration steps.

### Rate limiting

| Variable | Default |
| --- | --- |
| `RATE_LIMIT_ENABLED` | `true` |
| `RATE_LIMIT_PER_MINUTE` / `RATE_LIMIT_BURST` | runtime project type |
| `RATE_LIMIT_GENERAL_PER_MINUTE` / `RATE_LIMIT_AUTH_PER_MINUTE` / `RATE_LIMIT_EXPENSIVE_PER_MINUTE` | standalone project type (per endpoint class) |

### Queue and cache

| Variable | Default | Notes |
| --- | --- | --- |
| `QUEUE_DRIVER` | `sync` | `sync` runs jobs inline; `redis` uses asynq. |
| `QUEUE_CONCURRENCY` | CPU-based | Worker concurrency for the Redis driver. |
| `CACHE_DRIVER` | `memory` | `memory` or `redis`. |
| `REDIS_URL` | — | Shared by queue and cache Redis drivers; adds a `redis` readiness check. |

### Mail

| Variable | Default | Notes |
| --- | --- | --- |
| `MAIL_DRIVER` | `log` | `log` renders the MIME message into logs; `smtp` sends. |
| `MAIL_HOST` / `MAIL_PORT` | — | SMTP endpoint. |
| `MAIL_USERNAME` / `MAIL_PASSWORD` | — | Auth is used when the username is set. |
| `MAIL_ENCRYPTION` | `starttls` | `starttls` (587), `tls` (465), or `none` (25). |
| `MAIL_FROM_ADDRESS` / `MAIL_FROM_NAME` | — | Default sender. |

### File storage

| Variable | Default | Notes |
| --- | --- | --- |
| `STORAGE_DRIVER` | `local` | `local` or `s3`. |
| `STORAGE_LOCAL_ROOT` / `STORAGE_LOCAL_BASE_URL` | — | Path-confined local disk and its public URL prefix. |
| `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` | — | Any S3-compatible service (AWS, MinIO, R2, Spaces). |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | — | Credentials. |
| `S3_USE_SSL` / `S3_USE_PATH_STYLE` | — | Endpoint style toggles. |
| `S3_PUBLIC_BASE_URL` / `S3_PRESIGN_TTL` | — | Public URLs, or presigned URLs with a TTL. |

### API docs (runtime project type)

| Variable | Default | Notes |
| --- | --- | --- |
| `DOCS_ENABLED` | on in development only | Swagger UI + spec routes. |
| `DOCS_PATH` / `DOCS_SPEC_PATH` | `/docs`, `/openapi.json` | Mount points. |
| `DOCS_TITLE` / `DOCS_DESCRIPTION` / `DOCS_VERSION` / `DOCS_SERVERS` | — | Spec metadata. |
| `DOCS_BASIC_AUTH_USERNAME` / `DOCS_BASIC_AUTH_PASSWORD` | — | Protect the docs routes. |

### Devtools dashboard (runtime project type)

| Variable | Default | Notes |
| --- | --- | --- |
| `DEVTOOLS_ENABLED` | on in development only | Enabling it outside development is a startup error. |
| `DEVTOOLS_PATH` | `/_ginkit` | Dashboard mount point. |

### Feature flags (runtime project type)

| Variable | Default | Notes |
| --- | --- | --- |
| `FLAGS` | *(empty)* | Comma-separated boolean flags read by `flags.FromEnv`; surrounding whitespace and empty items are ignored. |

### Observability

| Variable | Default | Notes |
| --- | --- | --- |
| `METRICS_ENABLED` | off | Prometheus `/metrics`. |
| `PPROF_ENABLED` | off | Never expose publicly. |

Standalone projects read the same core, auth/session, and rate-limit
variables through `internal/platform/config` (the queue/cache/mail/storage
subsystems are runtime features).

The runtime refuses unsafe production defaults, applies request timeouts
and body limits, and accepts trusted proxy CIDRs explicitly. Liveness never
depends on a database; readiness may check it with a short timeout. See
[Deployment](/gin-kit/deployment/) for production checklists.
