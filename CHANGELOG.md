# Changelog

All notable GinKit changes are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

No unreleased changes.

## [0.2.0] - 2026-07-26

### Added

- Secure generated runtime defaults: request IDs, security headers, body limits, trusted proxies, CORS, rate limiting, graceful shutdown, and database readiness.
- Typed environment configuration with production fail-fast validation.
- Runtime GORM and sqlx database adapters across SQLite, PostgreSQL, MySQL, and MariaDB.
- Runnable Tasks CRUD vertical slice for API and UI projects.
- Generated middleware, authentication, password, application, and rate-limit tests.
- Atomic scaffolding, stronger CLI validation, read-only `ginkit check`, AI development skill guidance, and safer Docker defaults.

### Security

- Added Argon2id parameter-aware password verification and refresh-token hashing primitives.
- Added bounded database-container retries to generated-project CI smoke tests.

## [0.1.0] - 2026-07-26

### Added

- Interactive GinKit project scaffolding for API and UI applications.
- SQLite, PostgreSQL, MySQL, and MariaDB selections.
- GORM and sqlx data-access selections.
- Generated authentication primitives, migrations, Docker files, and AI-agent guidance.
