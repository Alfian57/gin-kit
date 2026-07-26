# Contributing to gin-kit

Thank you for helping improve gin-kit. Contributions may target the framework
runtime, CLI, generated editions, documentation, or delivery pipeline.

## Development setup

Requirements:

- Go 1.26 or newer;
- Git;
- Node.js 20+ when working on the UI template or documentation site;
- a C compiler when running SQLite integration tests.

Useful commands:

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd framework internal)"
go run ./cmd/gin-kit new /tmp/gin-kit-check --non-interactive --edition starter --module example.com/check --mode api --database sqlite --orm gorm
cd website && npm ci && npm run check && npm run build
```

The pull request workflow validates the runtime, documentation, and generated
framework/starter projects. Template changes must pass the complete
database/ORM matrix on `main`, including migration smoke tests for PostgreSQL,
MySQL, MariaDB, and SQLite.

## Pull requests

1. Explain the user-facing behavior and why it belongs in gin-kit.
2. Add or update tests.
3. Update English documentation and generated-project instructions when behavior changes.
4. Run the relevant generated-project matrix before requesting review.
5. Keep commits focused. A separate CLA is not required.

Pull requests must pass the Go quality, generated-project smoke, release snapshot, vulnerability scan, dependency review, and CodeQL checks before merge.

## Template changes

The template tree is embedded into the CLI. Any template change must be checked
with framework and starter projects in API and UI modes. Changes to database or
ORM templates should be tested against every supported database and ORM
combination.

## New generators and databases

New generators must:

- use the `gin-kit generate ...` command tree;
- refuse accidental overwrites;
- produce formatted Go;
- include tests and an English explanation;
- preserve the router → handler → service → repository → database flow.

New database support must include connection setup, migration dialect, Docker documentation, health checks, and CI coverage.

## Releases

gin-kit uses SemVer tags in the `v0.x.y` series. Only maintainers may create annotated release tags. Read [docs/releasing.md](docs/releasing.md) for the complete release and verification procedure.
