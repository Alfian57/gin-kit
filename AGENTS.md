# AGENTS.md

## Project overview

gin-kit is an opinionated Go framework built on Gin — "Everything included,
nothing hidden." The repository contains a versioned runtime (`runtime/`),
a project CLI (`cmd/gin-kit`, `internal/cli`), scaffold templates for a thin
Runtime project type and a Standalone project type
(`internal/cli/templates/`), code generators (`internal/cli/generators/`),
and the documentation site (`website/`). Application code follows the
explicit router → handler → service → repository → database flow, with
transport DTOs in `internal/dto`.

## Repository map

- `runtime/` — the versioned runtime: `httpx` (envelope + binders),
  `validation`, `query`, `config`, `database`, `auth`, `password`, `session`,
  `queue`, `schedule`, `cache`, `events`, `mail`, `storage`, `factory`,
  `openapi`, `metrics`, `apptest`, `browsertest`, plus `app.go` lifecycle.
- `internal/cli/templates/` — scaffold for `gin-kit new`. Files under
  `templates/runtime/` render only for the runtime project type; shared files
  render for both, except that the runtime project type emits only an explicit
  allowlist (`templateOutputPath` in `internal/cli/new.go`). Path gates:
  `auth_`/`/auth/` requires `--auth`, `tasks_` requires `--example`,
  `docker/` requires `--docker`, web/session paths require `--mode ui`.
- `internal/cli/generators/` — a separate `go:embed` tree for
  `gin-kit generate`; `resource/shared|runtime|standalone` split mirrors the
  project types.
- `website/src/content/docs/` — the documentation site (GitHub Pages).
- `docs/` — repo-level references (CLI, architecture, releasing, AI agents).

## Commands

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd runtime internal)"
gofmt -w cmd runtime internal
go run ./cmd/gin-kit new /tmp/gin-kit-sample --non-interactive --project-type standalone --module example.com/sample --mode api --database sqlite --orm gorm
cd website && npm ci && npm run check && npm run build
```

## Engineering rules

- Keep source, README files, CLI messages, and generated documentation in English.
- Generators must produce explicit, wired-by-hand code. Do not introduce
  reflection-based dependency injection, service locators, or implicit
  application wiring; generators print exact wiring snippets instead of
  editing existing files.
- Runtime projects import the versioned gin-kit runtime. Keep their
  application routes, handlers, services, domains, repositories, migrations,
  and configuration visible and editable.
- Standalone projects remain standalone and buildable without gin-kit
  installed; they vendor runtime copies under `internal/platform/`. The HTTP
  contract must stay identical across project types.
- Keep the public API small and explicit. Prefer options, interfaces, hooks,
  raw Gin access, and standard Go types as customization boundaries.
- Preserve the canonical API response envelope and detailed validation field
  errors (`422 validation_failed` + `details.fields`, `400 invalid_json`,
  `413 body_too_large`). Never expose stack traces, credentials, tokens,
  database errors, or submitted secret values in HTTP responses.
- Generated code keeps transport shapes in `internal/dto`: handlers bind
  request DTOs, services take DTOs and return domain values, responses go
  through explicit `New<Name>Response` mappers, and credential-like fields
  (`password`, `secret`, `token`, `hash`) never appear in response types.
- Runtime handlers self-describe for OpenAPI: route changes must
  update the corresponding `Describe` call (standalone: `api/openapi.yaml`).
- Go code templates use `.tmpl` so they are not compiled as part of this
  repository. Agent guidance templates may be `.tmpl` (rendered) or static —
  check the file. When renaming any template to or from `.tmpl`, update the
  runtime allowlist in `templateOutputPath`; it matches the file
  name **before** the suffix is stripped.
- Any template change requires Runtime and Standalone API/UI scaffold
  validation (`.github/scripts/scaffold-smoke.sh`). Changes touching
  `tasks_*` templates also require local `--example` scaffolds in both
  project types — the CI matrix does not cover `--example`.
- Keep migrations versioned SQL; do not add production AutoMigrate behavior.
  Runtime compose overwrites shared files, but shared `migrations/00001`
  overwrites runtime — runtime-only migrations need new filenames.
- Add tests for new CLI behavior and generated output.

## Documentation discipline

Every behavior change updates the documents it affects, **in the same pull
request**. A feature is not done until its documentation exists.

New features must land with:

1. A page or section on the documentation site
   (`website/src/content/docs/` — this deploys to GitHub Pages), registered
   in the `astro.config.mjs` sidebar when it is a new page.
2. A `CHANGELOG.md` entry under `[Unreleased]`.
3. `docs/cli.md` updates when the CLI surface changes.
4. Template AI-guidance updates (`internal/cli/templates/AGENTS.md*` and
   friends) when the generated-project workflow changes.

Change-type matrix — update at minimum:

| Change | Required documentation |
| --- | --- |
| New runtime package or option | Guide page on the site + `configuration.md` env vars + CHANGELOG |
| New or changed generator / CLI command | `cli-generators.md` + `docs/cli.md` + template `AGENTS.md*` + CHANGELOG + smoke |
| New/changed env variable | `configuration.md` reference table + the subsystem page |
| New stable error code | `error-handling.md` catalogue |
| Breaking change (API, generated layout, helper signatures) | `upgrading.md` + CHANGELOG under **Changed/Breaking** |
| Endpoint added/changed in templates | Runtime `Describe` call + Standalone `api/openapi.yaml` |
| Validation rule/message added | `validation.md` catalogue |
| Repo workflow / agent guidance change | root `AGENTS.md`, `CONTRIBUTING.md`, skill files |

## CI/CD and releases

- Treat the CI workflow as the source of truth for required checks.
- Run the generated-project smoke script when changing templates:
  `.github/scripts/scaffold-smoke.sh`.
- Documentation changes must pass the Astro/Starlight check and static build.
- Do not create, move, or push release tags unless a maintainer explicitly authorizes the exact version.
- Releases use annotated `v0.x.y` tags created from `main`; the release workflow builds binaries and publishes checksums and provenance.
- Never add long-lived release credentials to the repository. Release provenance uses GitHub Actions OIDC.
- Read `docs/releasing.md` before preparing a release or asking an AI agent to push one.

## Git and pull request workflow

- Work on a focused branch and submit every change through a pull request.
  Never push commits directly to `main`.
- Use concise Conventional Commit-style subjects for branch commits and pull
  request titles, such as `feat:`, `fix:`, `docs:`, or `chore:`.
- Merge only after all required CI and security checks pass. Do not bypass
  branch protection with `--admin` unless a maintainer explicitly authorizes
  that exact bypass.
- Preserve the repository's merge-commit history. Use
  `gh pr merge <number> --merge` without `--subject` or `--body`; do not use
  squash or rebase merges.
- Let GitHub create the canonical merge message:
  `Merge pull request #<number> from <owner>/<branch>`, with the pull request
  title in the commit body. Do not replace it with a Conventional Commit
  subject.
- After merging, fetch `main` and verify that the new commit has two parents
  and the canonical GitHub subject. Do not rewrite published `main` history or
  move release tags merely to normalize an older commit message.

## AI workflow

Read `.github/skills/gin-kit-development/SKILL.md` when changing the
scaffold, generators, or generated runtime. `docs/ai-agents.md` describes the
agent files that ship with generated projects.
