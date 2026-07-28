---
name: gin-kit-development
description: Build and maintain the gin-kit runtime, CLI, project types, database integrations, generated API or UI applications, and documentation.
license: MIT
compatibility: Requires Go 1.26; UI template work also requires Node.js 20+.
---

# gin-kit development

Read `AGENTS.md` before editing. gin-kit provides two deliberate project types:

- `runtime` is the default. It imports the versioned gin-kit runtime and keeps
  only application-owned code in the generated repository.
- `standalone` vendors runtime copies under
  `internal/platform/` for developers who want to inspect or replace every
  layer. The HTTP contract must stay identical across project types.

Keep the generated flow explicit:

```text
router -> handler (DTO in/out) -> service -> repository -> database
```

Use constructors and context-aware interfaces. Transport shapes live in
`internal/dto`; services take request DTOs and return domain values. Put
schema changes in versioned SQL migrations. Go code templates must use
`.tmpl` so they are not compiled as part of the gin-kit repository.

Useful commands:

```bash
go test ./...
go vet ./...
go test -race ./...
test -z "$(gofmt -l cmd runtime internal)"
go run ./cmd/doccheck
gin-kit new <name> --project-type runtime|standalone [--auth --example --docker]
gin-kit run
gin-kit generate resource <Name> --fields "title:string,done:bool"
gin-kit db up
gin-kit explain architecture
```

## Template-change protocol

1. Change the template under `internal/cli/templates/` (scaffold) or
   `internal/cli/generators/` (generate commands). They are separate
   `go:embed` trees.
2. Run `.github/scripts/scaffold-smoke.sh` for the standard matrix cells so
   local checks match CI (it always rebuilds the CLI — a stale binary at
   `/tmp/gin-kit` produces confusing "unknown flag" failures otherwise).
3. If the change touches `tasks_*` templates, also scaffold `--example`
   projects in both project types and both modes and run `go test ./...` in each —
   **the CI matrix does not cover `--example`**.
4. Runtime-development smoke tests may use a local module replacement
   (`--runtime-replace`); user-facing generated projects must pin the
   release version and must not contain a local replacement.

Known traps:

- The Runtime project type emits only an explicit allowlist of shared templates
  (`templateOutputPath` in `internal/cli/new.go`). The allowlist matches the
  file name **before** the `.tmpl` suffix is stripped — renaming a template
  to or from `.tmpl` without updating the allowlist silently drops it from
  Runtime projects.
- Path gates: `auth_`/`/auth/` requires `--auth`, `tasks_` requires
  `--example`, `docker/` requires `--docker`, web/session paths require UI
  mode.
- Templates under `templates/runtime/` overwrite same-named shared files —
  except `migrations/00001_init.sql`, where the shared file wins; add
  runtime-only migrations under new filenames.

## Documentation discipline

Every behavior change updates its documentation in the same PR (matrix in
`AGENTS.md`): new features need a documentation-site page
(`website/src/content/docs/` + sidebar), a CHANGELOG entry, `docs/cli.md`
when the CLI changes, and template AI-guidance updates when the generated
workflow changes. Verify site changes with
`cd website && npm run check && npm run build`.

## Source documentation

Every package-level declaration, struct field, and interface method in
`cmd/`, `runtime/`, and `internal/` needs an English doc comment. Explain the
actual behavior and important defaults, ownership, concurrency, errors, or
security constraints rather than repeating the identifier. Test entry points
are self-describing, but reusable test helpers and fixtures are documented.
Run `go run ./cmd/doccheck`; CI and release checks enforce the same rule.

## Security and contracts

Use Argon2id for passwords, short-lived access tokens, rotating refresh tokens, secure cookies, CSRF protection, and explicit production-secret validation. Never log passwords, tokens, or credentials.

Keep generic lifecycle, HTTP policy, response, validation, and security behavior
inside the runtime. Customization must use explicit options,
interfaces, hooks, middleware, raw Gin access, or a documented module fork.
Avoid reflection-based containers and hidden application dependencies.

The canonical JSON error envelope includes `code`, `message`, `request_id`,
and optional `details.fields`. Validation errors use JSON/form field names
and must never echo submitted values. Malformed JSON is a `400 invalid_json`;
semantic validation is a `422 validation_failed`; oversized bodies are a
`413 body_too_large`. Response DTOs never include credential-like fields.

## Git and pull request workflow

Work on a focused branch and submit changes through a pull request. Use
Conventional Commit-style subjects for branch commits and pull request titles,
then wait for every required check before merging. Preserve GitHub's canonical
merge commit with:

```bash
gh pr merge <number> --merge
```

Do not pass `--subject` or `--body`, use squash/rebase merging, push directly
to `main`, or bypass branch protection without explicit maintainer
authorization. After the merge, fetch `main` and confirm that the commit
subject is `Merge pull request #<number> from <owner>/<branch>` and that it has
two parents. Do not rewrite published history solely to normalize a merge
message.

Release tags are maintainer-only operations: never create or push an
annotated `v0.x.y` tag without explicit authorization for the exact version.
Read `docs/releasing.md` before preparing a release.
