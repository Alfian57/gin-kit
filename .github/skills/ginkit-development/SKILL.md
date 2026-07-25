---
name: ginkit-development
description: Build and maintain the GinKit framework runtime, CLI, project editions, database integrations, generated API or UI applications, and documentation.
license: MIT
compatibility: Requires Go 1.26; UI template work also requires Node.js 20+.
---

# GinKit development

Read `AGENTS.md` before editing. GinKit provides two deliberate editions:

- `framework` is the default. It imports the versioned GinKit runtime and keeps
  only application-owned code in the generated repository.
- `starter` is standalone. It exposes its infrastructure source for developers
  who want to inspect or replace every layer.

Keep the generated flow explicit:

```text
router -> handler -> service -> repository -> database
```

Use constructors and context-aware interfaces. Put schema changes in versioned SQL migrations. Template source files must use `.tmpl` so they are not compiled as part of the GinKit repository.

Useful commands:

```bash
go test ./...
go vet ./...
go test -race ./...
test -z "$(gofmt -l cmd framework internal)"
ginkit new <name> --edition framework
ginkit new <name> --edition starter
ginkit run
ginkit generate resource <name>
ginkit db up
ginkit explain architecture
```

After changing templates, scaffold framework and starter projects in both API
and UI modes, then run `go mod tidy`, `go test ./...`, and `go build ./...` in
each generated project. Framework-development smoke tests may use an explicit
local module replacement; user-facing generated projects must pin the release
version and must not contain a local replacement.

Use `.github/scripts/scaffold-smoke.sh` for generated-project validation so local
checks match CI. Release tags are maintainer-only operations: never create or
push an annotated `v0.x.y` tag without explicit authorization for the exact
version. Read `docs/releasing.md` before preparing a release.

Use Argon2id for passwords, short-lived access tokens, rotating refresh tokens, secure cookies, CSRF protection, and explicit production-secret validation. Never log passwords, tokens, or credentials.

Keep generic lifecycle, HTTP policy, response, validation, and security behavior
inside the framework runtime. Customization must use explicit options,
interfaces, hooks, middleware, raw Gin access, or a documented module fork.
Avoid reflection-based containers and hidden application dependencies.

The canonical JSON error envelope includes `code`, `message`, `request_id`, and
optional `details.fields`. Validation errors use JSON/form field names and must
never echo submitted values. Malformed JSON is a `400 invalid_json`; semantic
validation is a `422 validation_failed`.
