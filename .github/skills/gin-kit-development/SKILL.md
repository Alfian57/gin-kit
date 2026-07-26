---
name: gin-kit-development
description: Build and maintain the gin-kit framework runtime, CLI, project editions, database integrations, generated API or UI applications, and documentation.
license: MIT
compatibility: Requires Go 1.26; UI template work also requires Node.js 20+.
---

# gin-kit development

Read `AGENTS.md` before editing. gin-kit provides two deliberate editions:

- `framework` is the default. It imports the versioned gin-kit runtime and keeps
  only application-owned code in the generated repository.
- `starter` is standalone. It vendors runtime copies under
  `internal/platform/` for developers who want to inspect or replace every
  layer. The HTTP contract must stay identical across editions.

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
test -z "$(gofmt -l cmd framework internal)"
gin-kit new <name> --edition framework|starter [--auth --example --docker]
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
   projects in both editions and both modes and run `go test ./...` in each —
   **the CI matrix does not cover `--example`**.
4. Framework-development smoke tests may use a local module replacement
   (`--framework-replace`); user-facing generated projects must pin the
   release version and must not contain a local replacement.

Known traps:

- The framework edition emits only an explicit allowlist of shared templates
  (`templateOutputPath` in `internal/cli/new.go`). The allowlist matches the
  file name **before** the `.tmpl` suffix is stripped — renaming a template
  to or from `.tmpl` without updating the allowlist silently drops it from
  framework-edition projects.
- Path gates: `auth_`/`/auth/` requires `--auth`, `tasks_` requires
  `--example`, `docker/` requires `--docker`, web/session paths require UI
  mode.
- Templates under `templates/framework/` overwrite same-named shared files —
  except `migrations/00001_init.sql`, where the shared file wins; add
  framework-only migrations under new filenames.

## Documentation discipline

Every behavior change updates its documentation in the same PR (matrix in
`AGENTS.md`): new features need a documentation-site page
(`website/src/content/docs/` + sidebar), a CHANGELOG entry, `docs/cli.md`
when the CLI changes, and template AI-guidance updates when the generated
workflow changes. Verify site changes with
`cd website && npm run check && npm run build`.

## Security and contracts

Use Argon2id for passwords, short-lived access tokens, rotating refresh tokens, secure cookies, CSRF protection, and explicit production-secret validation. Never log passwords, tokens, or credentials.

Keep generic lifecycle, HTTP policy, response, validation, and security behavior
inside the framework runtime. Customization must use explicit options,
interfaces, hooks, middleware, raw Gin access, or a documented module fork.
Avoid reflection-based containers and hidden application dependencies.

The canonical JSON error envelope includes `code`, `message`, `request_id`,
and optional `details.fields`. Validation errors use JSON/form field names
and must never echo submitted values. Malformed JSON is a `400 invalid_json`;
semantic validation is a `422 validation_failed`; oversized bodies are a
`413 body_too_large`. Response DTOs never include credential-like fields.

Release tags are maintainer-only operations: never create or push an
annotated `v0.x.y` tag without explicit authorization for the exact version.
Read `docs/releasing.md` before preparing a release.
