# Changelog

All notable gin-kit changes are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Database seeding: generated projects ship `cmd/seed` with an explicit
  seeder registry (`internal/database/seeders`), `gin-kit generate seeder`,
  and `gin-kit db seed`; `--example` projects seed sample tasks idempotently.
- `gin-kit db create|redo|reset|fresh` (destructive operations require
  `--yes`) and `gin-kit routes`, which boots the app and prints the sorted
  routing table (`cmd/server --routes`).

### Fixed

- Framework-edition `cmd/migrate` now loads `.env` like the server; starter
  projects load `.env` in the server, migrate, and seed binaries (a copied
  dotenv parser in `internal/platform/config`), and the starter migrate
  binary gains the same SQLite DSN fallback as the framework edition.

- Template-based, manifest-aware generators: `gin-kit generate resource`
  renders a complete vertical slice (domain model + repository interface,
  GORM or sqlx repository, service with typed inputs and validation, HTTP
  handler with allowlist filtering/sorting/pagination, tests with in-memory
  fakes, and a dialect-mapped goose migration) driven by a `--fields` DSL
  (`string`, `text`, `int`, `int64`, `float64`, `bool`, `time`, `?` for
  nullable), then prints the exact wiring snippet — explicit wiring, no file
  editing. `domain`, `repository`, `handler`, `service`, and `middleware`
  generators now render real, compilable code in the correct packages.

### Changed

- `snakeCase` naming is acronym- and dash-aware (`APIKey` → `api_key`);
  generated Go is gofmt-ed in staging before publishing, so a bad render
  aborts with the project untouched. Generator policy docs now embrace
  Laravel-style generators while keeping explicit wiring and rejecting
  reflection-based dependency injection.

- `framework/session`: encrypted cookie sessions (gin-contrib/sessions) with
  one-shot flash messages and constant-time CSRF protection (form field or
  header, `/api/` exempt by default), wired automatically into UI-mode
  projects in both editions; `SESSION_SECRET` is finally consumed, the tasks
  form carries a CSRF field, and starter projects receive a standalone
  `internal/platform/session` copy.

- Full `--auth` scaffold in both editions (Laravel Breeze-API equivalent):
  `users` and `refresh_tokens` migrations, domain models, GORM and sqlx
  repositories, an auth service with Argon2id passwords, timing-leveled
  login, and single-use refresh-token rotation, plus JSON endpoints for
  register, login, refresh, logout, and a database-backed `/api/v1/me` —
  with DB-free generated tests.

### Changed

- Framework-edition `api.Register`/`web.Register` signatures for `--auth`
  projects now receive the auth service; the previous sample-only
  `/api/v1/me` handlers were replaced by the full auth handler package.

- `framework/mail`: Laravel-style mailer with a fluent message builder,
  html/template rendering, an SMTP driver (wneessen/go-mail with
  none/tls/starttls), and a development log driver that renders the full MIME
  message; configured through `MAIL_*` variables and `config.MailOptions()`.
  Framework compose scaffolds an opt-in Mailpit service
  (`docker compose --profile mail up`).
- `framework/storage`: disk abstraction with a path-confined, atomic local
  driver and an S3-compatible driver (minio-go; AWS S3, MinIO, R2, Spaces)
  with presigned or public URLs, a gin `SaveUpload` multipart helper, and
  `STORAGE_*`/`S3_*` configuration via `config.StorageOptions()`; compose
  scaffolds an opt-in MinIO service (`--profile storage`).

- `framework/queue`: explicit background jobs with typed `Register`/`Dispatch`,
  an inline sync driver for development, and a Redis (asynq) driver with
  delays, retries, named queues, worker concurrency, and graceful drain,
  supervised by the application lifecycle; configured via
  `QUEUE_DRIVER`/`QUEUE_CONCURRENCY`/`REDIS_URL`. `app.Queue()` is always
  available, and framework-edition projects scaffold an `internal/jobs`
  example.
- `framework/schedule`: cron scheduling on robfig/cron with
  `Cron`/`Every`/`Daily`/`Hourly`, per-job panic recovery, optional
  skip-if-still-running, and graceful stop as an application runner
  (single-instance in this version).

- `framework/cache`: Laravel-style cache with in-memory (default) and Redis
  drivers behind one interface, a JSON `Remember` helper, key prefixes,
  increments and TTLs; configured with `CACHE_DRIVER`/`REDIS_URL`, with an
  automatic `redis` readiness check and shutdown-managed cleanup.
  `app.Cache()` is always available.
- `framework/events`: dependency-free in-process typed event bus (`On`/`Emit`)
  with synchronous ordered dispatch, joined handler errors, panic recovery,
  and unsubscribe.
- Framework-edition Docker Compose now includes a Redis service
  (`GIN_KIT_REDIS_PORT` override supported).

- Supervised background runners: `Application.Go(name, runner)` runs
  long-lived goroutines under `Run` with shared cancellation; a runner error
  or panic triggers graceful HTTP shutdown, runners drain before shutdown
  hooks, and hooks keep their LIFO order.

- `framework/query`: allowlist-based filtering (exact, partial, in,
  gte/lte/gt/lt), sorting, and pagination for list endpoints with GORM and
  parameterized-SQL appliers, portable LIKE escaping, native boolean binding,
  and standard pagination metadata. Starter-edition projects receive a
  standalone `internal/platform/query` copy.
- The Tasks examples in both editions now demonstrate query filtering,
  sorting, and pagination end to end.

- `httpx.BindQuery` and `httpx.BindURI` generic binders with the same
  validation behavior and error envelopes as `BindJSON` (`invalid_query` /
  `invalid_path`).
- `framework/apptest` test helpers: build an application, perform JSON
  requests, and decode envelope responses without recorder boilerplate.

- Opt-in Prometheus metrics endpoint (`MetricsOptions` / `METRICS_ENABLED`)
  with request count, duration histogram, and in-flight gauge labeled by route
  pattern, plus a registry accessor for custom metrics.
- Request-scoped structured logging: `httpx.Logger(c)` returns an slog logger
  carrying the request ID, method, and path.
- Opt-in pprof endpoints under `/debug/pprof` (`PProfOptions` /
  `PPROF_ENABLED`).

- `framework/config`: typed environment loading with fail-fast validation,
  production requirements, Go-duration timeouts, and dependency-free `.env`
  support where the real environment always wins.
- Framework-edition applications now apply timeouts, CORS origins, rate
  limiting, body limits, and trusted proxies from the environment through
  `framework/config`, and ship a framework-specific `.env.example`.

- `framework/auth` Gin middleware: `RequireAuth`, `ClaimsFromContext`, and
  `UserID` with canonical `401` envelopes and `WWW-Authenticate` headers.
- `HTTPOptions.TrustedProxies` for explicit forwarded-header trust.
- Framework-edition `--auth` scaffolding: JWT manager wiring from `JWT_SECRET`,
  a Bearer-protected `/api/v1/me` sample route, and generated tests for both
  API and UI modes.

### Security

- Framework applications no longer trust `X-Forwarded-For` from any peer by
  default; list your proxies in `HTTPOptions.TrustedProxies`
  (`TRUSTED_PROXY_CIDRS` in generated projects) to restore forwarded client
  addresses. Rate limiting and access logs now key on non-spoofable client
  IPs.

### Fixed

- Generated framework README instructed copying `.env.example` to `.env`
  although nothing loaded the file; framework applications now load it.
- Framework-edition projects received the starter's `.env.example` with
  variables their runtime never read.
- `--auth` was silently ignored for framework-edition projects.
- Framework-edition UI projects registered the Tasks example route only when
  HTML templates were absent.
- Documentation described a functional-options API and middleware order that
  never existed; it now shows the real `framework.Options` API and order.

### Changed

- **Breaking:** renamed the project from GinKit to gin-kit with no migration
  path: module path `github.com/Alfian57/ginkit` → `github.com/Alfian57/gin-kit`,
  CLI binary `ginkit` → `gin-kit`, project manifest `.ginkit.yaml` →
  `.gin-kit.yaml`, default JWT issuer `ginkit` → `gin-kit`, generated health
  table `ginkit_health` → `gin_kit_health`, and CI smoke-test environment
  variables `GINKIT_*` → `GIN_KIT_*`. Releases `v0.2.0` and earlier remain
  importable only through the old module path.

### Added

- Opinionated framework runtime on Gin with lifecycle, security, rate limiting,
  SQL/GORM/sqlx adapters, authentication, password hashing, and extension
  hooks.
- Detailed field-level validation errors and a canonical response envelope.
- Framework and standalone starter editions with module-aware scaffolding,
  transactional generators, diagnostics, and local runtime replacement support.
- English Astro/Starlight documentation deployed through GitHub Pages.

## [0.2.0] - 2026-07-26

### Added

- Secure generated runtime defaults: request IDs, security headers, body limits, trusted proxies, CORS, rate limiting, graceful shutdown, and database readiness.
- Typed environment configuration with production fail-fast validation.
- Runtime GORM and sqlx database adapters across SQLite, PostgreSQL, MySQL, and MariaDB.
- Runnable Tasks CRUD vertical slice for API and UI projects.
- Generated middleware, authentication, password, application, and rate-limit tests.
- Atomic scaffolding, stronger CLI validation, read-only `gin-kit check`, AI development skill guidance, and safer Docker defaults.

### Security

- Added Argon2id parameter-aware password verification and refresh-token hashing primitives.
- Added bounded database-container retries to generated-project CI smoke tests.

## [0.1.0] - 2026-07-26

### Added

- Interactive gin-kit project scaffolding for API and UI applications.
- SQLite, PostgreSQL, MySQL, and MariaDB selections.
- GORM and sqlx data-access selections.
- Generated authentication primitives, migrations, Docker files, and AI-agent guidance.
