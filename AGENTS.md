# AGENTS.md

## Project overview

GinKit is an opinionated Go framework built on Gin. The repository contains a
versioned runtime, a project CLI, a thin framework edition, and a standalone
starter edition. Application code follows the explicit router → handler →
service → repository → database flow.

## Commands

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd framework internal)"
gofmt -w cmd framework internal
go run ./cmd/ginkit new /tmp/ginkit-sample --non-interactive --edition starter --module example.com/sample --mode api --database sqlite --orm gorm
cd website && npm ci && npm run check && npm run build
```

## Engineering rules

- Keep source, README files, CLI messages, and generated documentation in English.
- Do not introduce Laravel-like `make:*` commands, reflection-based dependency
  injection, service locators, or implicit application wiring.
- Framework-edition projects import the versioned GinKit runtime. Keep their
  application routes, handlers, services, domains, repositories, migrations,
  and configuration visible and editable.
- Starter-edition projects remain standalone and buildable without GinKit
  installed.
- Keep the public API small and explicit. Prefer options, interfaces, hooks,
  raw Gin access, and standard Go types as customization boundaries.
- Preserve the canonical API response envelope and detailed validation field
  errors. Never expose stack traces, credentials, tokens, database errors, or
  submitted secret values in HTTP responses.
- Template files use `.tmpl` so they are not compiled as part of the GinKit repository.
- Any template change requires framework and starter API/UI scaffold validation.
- Keep migrations versioned SQL; do not add production AutoMigrate behavior.
- Add tests for new CLI behavior and generated output.

## CI/CD and releases

- Treat the CI workflow as the source of truth for required checks.
- Run the generated-project smoke script when changing templates:
  `.github/scripts/scaffold-smoke.sh`.
- Documentation changes must pass the Astro/Starlight check and static build.
- Do not create, move, or push release tags unless a maintainer explicitly authorizes the exact version.
- Releases use annotated `v0.x.y` tags created from `main`; the release workflow builds binaries and publishes checksums and provenance.
- Never add long-lived release credentials to the repository. Release provenance uses GitHub Actions OIDC.
- Read `docs/releasing.md` before preparing a release or asking an AI agent to push one.

## AI workflow

Read `.github/skills/ginkit-development/SKILL.md` when changing the scaffold, generators, or generated runtime.
