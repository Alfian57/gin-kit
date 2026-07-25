---
name: ginkit-development
description: Build and maintain GinKit, its embedded Go project templates, generators, database migrations, and generated API or UI projects. Use when changing scaffold behavior, CLI commands, generated architecture, auth, or templates.
license: MIT
compatibility: Requires Go 1.26; UI template work also requires Node.js 20+.
---

# GinKit development

Read `AGENTS.md` before editing. GinKit is a standalone-project scaffolder; generated projects must not import GinKit at runtime.

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
test -z "$(gofmt -l cmd internal)"
ginkit new <name>
ginkit run
ginkit generate resource <name>
ginkit db up
ginkit explain architecture
```

After changing templates, scaffold both an API project and a UI project, then run `go mod tidy`, `go test ./...`, and `go build ./...` in each generated project.

Use `.github/scripts/scaffold-smoke.sh` for generated-project validation so local
checks match CI. Release tags are maintainer-only operations: never create or
push an annotated `v0.x.y` tag without explicit authorization for the exact
version. Read `docs/releasing.md` before preparing a release.

Use Argon2id for passwords, short-lived access tokens, rotating refresh tokens, secure cookies, CSRF protection, and explicit production-secret validation. Never log passwords, tokens, or credentials.
