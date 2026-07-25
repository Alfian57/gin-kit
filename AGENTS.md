# AGENTS.md

## Project overview

GinKit is a Go CLI that embeds templates and scaffolds standalone Gin projects. The canonical generated-project flow is router → handler → service → repository → database.

## Commands

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal)"
gofmt -w cmd internal
go run ./cmd/ginkit new /tmp/ginkit-sample --non-interactive --module example.com/sample --mode api --database sqlite --orm gorm
```

## Engineering rules

- Keep source, README files, CLI messages, and generated documentation in English.
- Do not introduce Laravel-like `make:*` commands, hidden dependency injection, or a runtime framework dependency for generated projects.
- Generated projects must remain buildable without GinKit installed.
- Template files use `.tmpl` so they are not compiled as part of the GinKit repository.
- Any template change requires API and UI scaffold validation.
- Keep migrations versioned SQL; do not add production AutoMigrate behavior.
- Add tests for new CLI behavior and generated output.

## CI/CD and releases

- Treat the CI workflow as the source of truth for required checks.
- Run the generated-project smoke script when changing templates:
  `.github/scripts/scaffold-smoke.sh`.
- Do not create, move, or push release tags unless a maintainer explicitly authorizes the exact version.
- Releases use annotated `v0.x.y` tags created from `main`; the release workflow builds binaries and publishes checksums and provenance.
- Never add long-lived release credentials to the repository. Release provenance uses GitHub Actions OIDC.
- Read `docs/releasing.md` before preparing a release or asking an AI agent to push one.

## AI workflow

Read `.github/skills/ginkit-development/SKILL.md` when changing the scaffold, generators, or generated runtime.
