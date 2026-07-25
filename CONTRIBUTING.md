# Contributing to GinKit

Thank you for helping make Go more approachable. GinKit is intentionally open to contributions that improve the generated code, the learning experience, or the reliability of the CLI.

## Development setup

Requirements:

- Go 1.26 or newer;
- Git;
- Node.js 20+ only when working on the UI template;
- a C compiler when running SQLite integration tests.

Useful commands:

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal)"
go run ./cmd/ginkit new /tmp/ginkit-check --non-interactive --module example.com/check --mode api --database sqlite --orm gorm
```

The pull request workflow also validates generated API and UI projects. Template changes must pass the complete database/ORM matrix on `main`, including migration smoke tests for PostgreSQL, MySQL, MariaDB, and SQLite.

## Pull requests

1. Explain the user-facing behavior and why it belongs in GinKit.
2. Add or update tests.
3. Update English documentation and generated-project instructions when behavior changes.
4. Run the relevant generated-project matrix before requesting review.
5. Keep commits focused. A separate CLA is not required.

Pull requests must pass the Go quality, generated-project smoke, release snapshot, vulnerability scan, dependency review, and CodeQL checks before merge.

## Template changes

The template tree is embedded into the CLI. Any template change must be checked by scaffolding at least one API project and one UI project. Changes to database or ORM templates should be tested against every supported database and ORM combination.

## New generators and databases

New generators must:

- use the `ginkit generate ...` command tree;
- refuse accidental overwrites;
- produce formatted Go;
- include tests and an English explanation;
- preserve the router → handler → service → repository → database flow.

New database support must include connection setup, migration dialect, Docker documentation, health checks, and CI coverage.

## Releases

GinKit uses SemVer tags in the `v0.x.y` series. Only maintainers may create annotated release tags. Read [docs/releasing.md](docs/releasing.md) for the complete release and verification procedure.
