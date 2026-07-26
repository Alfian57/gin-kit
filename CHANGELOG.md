# Changelog

All notable gin-kit changes are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `gin-kit dev`: a hot-reload development server that builds `./cmd/server`
  to a binary, watches the project, and serves it through a local reverse
  proxy that holds requests while a rebuild is in flight; compile errors
  render as a browser error overlay (plain text for non-HTML clients) until
  the next successful build. `gin-kit run` remains the simple `go run`-based
  runner.
- Opt-in cursor (keyset) pagination in the query builder: setting
  `Options.CursorSort` to an allowed sort switches a list endpoint to cursor
  mode — `page` and `sort` are rejected, the `cursor` parameter carries an
  opaque keyset token tie-broken on `id`, `BuildCursorSQL`/`ApplyCursorGORM`
  fetch one probe row past `per_page`, and `NextCursor` plus `CursorMeta`
  produce `{"next_cursor": ..., "per_page": ...}` metadata. Offset
  pagination remains the default; the starter edition's vendored query
  package gains the same API.
- Authorization policies: the new `framework/authz` package (vendored as
  `internal/platform/authz` in starter projects) provides allowlist-style
  `Decision` values with one stable `403 forbidden` envelope — deny reasons
  are logged and wrapped as error causes, never serialized — enforced in
  handlers via `authz.Authorize` or converted to a mapper-ready error with
  `Decision.Err()`. `gin-kit generate policy <Name>` renders a per-resource
  policy (CanView/CanCreate/CanUpdate/CanDelete) with placeholder rules, a
  table test, and the exact handler wiring snippet in both editions, and
  back-fills the vendored authz package into older starter projects.

- Visual identity: the gin-kit gopher mascot (tool belt included) becomes the
  project logo, with light/dark variants in the site header, a hero
  illustration on the landing page, a README header, a real favicon (the
  previous reference 404'd — `website/public/` did not exist), and a social
  preview image with `og:image`/`twitter:image` metadata (previously the
  site declared `summary_large_image` with no image at all).

- Complete AI-agent workflow: generated projects now render `AGENTS.md` from
  the project manifest (edition, mode, database, ORM, auth/example flags,
  module path) with the full command tree, DTO and error-contract rules, and
  a documentation-discipline section; the generated skill gains YAML
  frontmatter and concrete workflows (add a resource, custom validation
  rule, background work, testing, troubleshooting); Claude, Gemini, Copilot,
  and Cursor adapters carry the most-often-broken rules. The repository
  itself gains a root `CLAUDE.md`, a rewritten `AGENTS.md` with a repository
  map and a change-type documentation matrix, and a PR-template
  documentation checklist. Scaffold tests and the smoke script now verify
  the guidance files are emitted and rendered.

- Documentation: four new site pages — Request and response DTOs, Validation
  (built-in rule catalogue plus custom rules), Seeding and factories, and
  Upgrade notes — plus a complete environment-variable reference on the
  Configuration page, a stable error-code catalogue with an `ErrorMapper`
  example, expanded CLI, deployment, and getting-started guides, and a
  sidebar link to the changelog. `docs/cli.md` now documents `generate dto`,
  and `docs/ai-agents.md` accurately describes where agent adapters live.

### Fixed

- The `--fields` DSL marked nullable string fields (`nickname:string?`) as
  `validate:"required"`, rejecting the very requests the schema allows. Tags
  now follow nullability: nullable strings validate as
  `omitempty,max=255`, nullable text carries no constraint.

- The `httpx` binders (`BindJSON`/`BindQuery`/`BindURI`) validated with the
  package-level `validation.Default` even when the application configured
  `Options.Validator`, so custom rules and messages registered on the
  application validator were silently ignored. The framework now exposes the
  application validator to binders through the request context; resolution
  order is explicit argument, then application validator, then
  `validation.Default`.
- Starter projects discarded binding errors and answered a bare
  `400 validation_failed` with no field details, while the framework edition
  answered `422` with per-field messages. Both editions now share the same
  contract: `422 validation_failed` with `details.fields`, `400 invalid_json`
  for malformed bodies (never echoing the submitted payload), and
  `413 body_too_large` for oversized bodies.

### Changed

- Positioning: gin-kit's identity is now "Everything included, nothing
  hidden." — framework comparisons were removed from package docs, generated
  code, the website, and the repository description. The framing describes
  what gin-kit is (batteries included, explicit wiring, no magic) instead of
  what it resembles.
- Generated services now take `dto.Create<Name>Request` /
  `dto.Update<Name>Request` instead of a service-local `<Name>Input` struct
  with sentinel-error validation; validation happens once, at bind time, with
  `422` field details. `/api/v1/me` returns a `UserResponse`; register and
  login responses are typed `AuthResponse` values with identical wire shapes.
- **Breaking (regenerated starter projects only):** the starter
  `internal/platform/response` package is replaced by
  `internal/platform/httpx` — a standalone copy of the framework envelope and
  binders (`OK`, `Created`, `List`, `NoContent`, `Fail`, `BindJSON`,
  `BindQuery`, `BindURI`) plus `internal/platform/validation`. Error
  responses gain `details` and `request_id` fields. Existing generated
  projects keep working; new scaffolds and newly generated resources use the
  new packages.

- Docs site landing page rebuilt on real Starlight components — fixing the
  card icon that never rendered (an empty MDX expression) — with a
  feature grid covering the current framework, a quickstart, and edition
  cards; the header logo now has light/dark variants (it was invisible in
  dark mode) and the background grid is theme-aware. Repository description
  and topics updated to the framework positioning.

### Added

- A dedicated DTO layer in generated projects: `generate resource` now also
  renders `internal/dto/<name>_dto.go` with `Create<Name>Request` and
  `Update<Name>Request` (validation tags plus a `Normalize()` trimmer) and a
  `<Name>Response` with explicit `New<Name>Response` /
  `New<Name>ResponseList` mappers — no `db`/`gorm` tags, and
  credential-like fields (`password`, `secret`, `token`, `hash`) never appear
  in responses. Services accept request DTOs and return domain values;
  handlers wrap results in response DTOs. A standalone `gin-kit generate dto
  <Name> --fields ...` command renders just the DTO file.
- The auth scaffold ships typed DTOs in both editions (`RegisterRequest`,
  `LoginRequest`, `RefreshRequest`, `UserResponse`, `TokenResponse`,
  `AuthResponse`) replacing ad-hoc `gin.H` payloads and doc-only mirror
  structs — the OpenAPI spec and the wire format now come from the same
  types. The tasks example follows the same layout.

- Thirteen additional built-in validation messages (`gte`, `lte`, `gt`, `lt`,
  `url`, `uuid`, `numeric`, `alphanum`, `datetime`, `eqfield`, `ne`,
  `startswith`, `endswith`) with named parameters in field error details, in
  both the framework `validation` package and the starter copy.

- Auto-generated OpenAPI docs for framework-edition applications — zero
  annotations: `framework/openapi` builds the 3.0.3 spec at runtime from the
  live route table plus typed `Describe` calls that generated code carries
  (schemas reflected from Go structs, validation tags becoming constraints,
  list endpoints documenting filters/sorts/pagination). Swagger UI at
  `/docs`, spec at `/openapi.json`, fully configured via `DOCS_*` env vars
  (default on in development only) including optional basic-auth protection
  and a customizable security scheme; scaffolded auth/tasks/ping routes and
  `generate resource` handlers self-describe, and generated projects include
  a docs test.

- `framework/apptest` v2: PATCH, request options (`WithHeader`, `WithBearer`,
  `WithCookie`), form/multipart/raw bodies, `Meta`/`JSON` decoders, and a
  cookie-jar `Client` with a `CSRFToken` helper for session flows; apps built
  with `apptest.New` now close automatically via `t.Cleanup`. New integration
  helpers run repositories against unique in-memory SQLite databases:
  `OpenSQLite`, `Migrate` (goose), and `Seed`.
- `framework.Application.Close(ctx)`: runs shutdown hooks exactly once
  without serving, for tests and short-lived binaries.
- `framework/browsertest`: Playwright helpers (`Launch`, `NewPage`,
  `StartServer`, `Install`) that skip cleanly when browsers are absent;
  framework-edition UI projects scaffold an `e2e/` browser test, and
  `generate resource` now also emits a repository integration test in
  framework-edition projects.

### Changed

- `apptest.Do` takes request options instead of an `http.Header` parameter
  (breaking within the pre-1.0 test helper API).

- `framework/factory`: generic, ORM-agnostic model factories
  (`factory.Define`, `Make`, `MakeMany`, `Create`, `CreateMany`, deterministic
  `Seeded`) backed by gofakeit. `gin-kit generate factory` emits a per-model
  factory with field-aware fake data, `generate resource` now includes one,
  and starter projects receive a standalone `internal/platform/factory` copy.

### Changed

- Seeders now receive the full `*database.Connection` (SQL plus the project's
  GORM or sqlx handle) instead of `*sql.DB`, so repositories and factories
  work inside seeders; `cmd/seed` opens the connection through the edition's
  database package. Breaking for previously generated projects that
  regenerate seeders.

- Framework-edition generators for infrastructure concepts:
  `gin-kit generate job` (typed queue job with registration and dispatch
  snippets), `generate event` (domain event plus listener for the event bus),
  and `generate mail` (mailable builder with an HTML template). Starter
  projects receive a clear diagnostic since these depend on framework
  packages.

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
  full-coverage generators while keeping explicit wiring and rejecting
  reflection-based dependency injection.

- `framework/session`: encrypted cookie sessions (gin-contrib/sessions) with
  one-shot flash messages and constant-time CSRF protection (form field or
  header, `/api/` exempt by default), wired automatically into UI-mode
  projects in both editions; `SESSION_SECRET` is finally consumed, the tasks
  form carries a CSRF field, and starter projects receive a standalone
  `internal/platform/session` copy.

- Full `--auth` scaffold in both editions, a complete token-API vertical:
  `users` and `refresh_tokens` migrations, domain models, GORM and sqlx
  repositories, an auth service with Argon2id passwords, timing-leveled
  login, and single-use refresh-token rotation, plus JSON endpoints for
  register, login, refresh, logout, and a database-backed `/api/v1/me` —
  with DB-free generated tests.

### Changed

- Framework-edition `api.Register`/`web.Register` signatures for `--auth`
  projects now receive the auth service; the previous sample-only
  `/api/v1/me` handlers were replaced by the full auth handler package.

- `framework/mail`: transactional mailer with a fluent message builder,
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

- `framework/cache`: application cache with in-memory (default) and Redis
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

- Framework-edition projects generated with `--example` no longer register
  the tasks seeder: their tasks example is an in-memory stub without a tasks
  table, so `gin-kit db seed` failed. The seeder stays starter-only.
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
