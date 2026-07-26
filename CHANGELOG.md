# Changelog

All notable gin-kit changes are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
