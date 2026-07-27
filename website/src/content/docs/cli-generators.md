---
title: CLI and generators
description: The complete gin-kit command reference.
---

The `gin-kit` CLI creates projects, generates building blocks, manages the
database, and inspects the running application. Routes are never registered
automatically — every generator prints the exact wiring snippet to paste, so
application wiring stays explicit and reviewable.

## Project commands

```text
gin-kit new <path>        # create a project (interactive, or --non-interactive)
gin-kit run               # run the server with .env loaded
gin-kit dev               # hot-reload dev server behind a holding proxy
gin-kit build             # build a production binary
gin-kit check             # read-only: report gofmt drift, run tests and vet
gin-kit doctor            # diagnose toolchain, manifest, and database issues
gin-kit routes            # boot the app and print the sorted routing table
gin-kit upgrade           # starter: update vendored internal/platform code
gin-kit explain <topic>   # architecture | request-flow | database | auth | commands
```

`new` defaults to `--edition framework`; use `--edition starter` for the
standalone source-visible edition, `--auth` for the authentication vertical,
`--example` for the tasks example, and `--docker` for compose files.
Non-interactive creation requires `--module`, `--mode`, `--database`, and
`--orm`.

`routes` boots the application, so it needs a reachable database (SQLite
always works) and valid required secrets.

`dev` rebuilds the server binary on every change, holds requests behind its
proxy while a rebuild is in flight, and renders compile errors as a browser
overlay until the next successful build; `run` remains the simple runner.

## Generators

```text
gin-kit generate resource <Name> --fields "..." [--table name] [--soft-delete]
gin-kit generate domain <Name> [--fields ...]
gin-kit generate dto <Name> [--fields ...]
gin-kit generate repository <Name> [--fields ...]
gin-kit generate handler <Name>
gin-kit generate service <Name>
gin-kit generate middleware <Name>
gin-kit generate policy <Name>
gin-kit generate factory <Name> [--fields ...]
gin-kit generate seeder <Name>
gin-kit generate migration <name>
gin-kit generate job <Name>      # framework edition only
gin-kit generate event <Name>    # framework edition only
gin-kit generate mail <Name>     # framework edition only
```

`generate resource` is the flagship: it renders a working vertical slice —
domain model, [request/response DTOs](/gin-kit/dto/), GORM/sqlx repository
(following the project manifest), service that accepts DTOs and returns
domain values, HTTP handler with allowlist-based
filtering/sorting/pagination, tests with in-memory fakes, and a goose
migration with dialect-mapped column types — then prints the exact wiring
snippet to paste into `internal/app/app.go`.

`--soft-delete` opts the resource into explicit soft deletion: the domain
model gains a `DeletedAt *time.Time` field (hidden from JSON responses),
every repository query filters on a visible `deleted_at IS NULL` condition —
no implicit ORM scoping — and `Delete` becomes an `UPDATE` that stamps
`deleted_at` instead of removing the row (`404` on already-deleted rows).
The migration adds the nullable `deleted_at` column with an index, and
`deleted_at` joins `id`, `created_at`, and `updated_at` as a reserved field
name. Only `generate resource` accepts the flag — the standalone `domain`,
`repository`, and `dto` generators always render the plain variants, so
regenerate soft-deleting pieces through the resource generator.

`generate dto` renders just the DTO file for an existing model.
`generate policy` renders an [authorization policy](/gin-kit/authorization/)
in `internal/policy` — per-action decision methods with placeholder rules
and a table test — and works in both editions; starter projects get the
vendored `internal/platform/authz` package back-filled when it is missing.
`generate factory` creates a [model factory](/gin-kit/seeding-factories/)
with field-aware fake data; `generate seeder` a registry-based seeder.
`generate job`, `generate event`, and `generate mail` scaffold
[background jobs](/gin-kit/background-work/), typed events, and
[mailables](/gin-kit/mail-storage/) — they rely on framework packages, so
they are framework-edition only.

### The `--fields` grammar

Comma-separated `name:type` pairs; a trailing `?` makes a field nullable.
`id`, `created_at`, and `updated_at` are always generated.

| Type | Go type | Column | Filter | Validation |
| --- | --- | --- | --- | --- |
| `string` | `string` | `VARCHAR(255)` | partial match | `required,max=255` (`omitempty,max=255` if nullable) |
| `text` | `string` | `TEXT` | partial match | `required` (none if nullable) |
| `int` / `int64` | `int` / `int64` | `INTEGER` / `BIGINT` | comparison | — |
| `float64` (alias `float`) | `float64` | dialect-mapped | comparison | — |
| `bool` | `bool` | `BOOLEAN` | exact boolean | — |
| `time` (aliases `datetime`, `timestamp`) | `time.Time` | `TIMESTAMP` | comparison | — |

Field names containing `password`, `secret`, `token`, or `hash` are treated
as credentials and excluded from the generated response DTO. `--table`
overrides the derived table name.

### Generator safety

Generators preflight output paths, refuse accidental overwrites, gofmt in a
staging directory, and publish transactionally — a failed render leaves the
project untouched. Use `--dry-run` to inspect intended files. Errors name the
failed phase, a stable code, the affected path, and a recovery hint.

## Upgrading starter projects

Starter projects vendor their runtime under `internal/platform/`, so
`gin-kit upgrade` is how they receive platform fixes from newer CLI
releases. It re-renders the current CLI's platform templates for the
project's manifest, then reports each file as `up-to-date`, `outdated`
(stale vendored copy, safe to update), `modified` (local edits, kept),
`differs` (no baseline to verify against), `missing`, or `unmanaged`
(on-disk files the templates no longer render — never touched).

`--diff` shows unified diffs, `--apply` writes the safe updates, and
`--apply --force` also overwrites modified files. The `.gin-kit.sum`
checksum baseline distinguishes your edits from stale vendored code — see
[Upgrade notes](/gin-kit/upgrading/) for details. Framework-edition projects
upgrade the versioned module instead:
`go get github.com/Alfian57/gin-kit@vX.Y.Z`.

## Database commands

```text
gin-kit db up             # migrate to the latest version
gin-kit db down           # roll back one migration
gin-kit db status         # migration status table
gin-kit db redo           # down then up for the last migration
gin-kit db create <name>  # create an empty timestamped migration
gin-kit db seed           # run the seeder registry
gin-kit db reset --yes    # roll back everything (destructive)
gin-kit db fresh --yes    # reset, migrate up, then seed (destructive)
```

`reset` and `fresh` refuse to run without `--yes`. Server, migrate, and seed
binaries all load `.env` first; the real environment always wins. See
[Seeding and factories](/gin-kit/seeding-factories/) for the seeder registry.

## Explain topics

`gin-kit explain` answers questions offline: `architecture` (the layer flow),
`request-flow` (who binds, who decides, who persists), `database` (your
selected database/ORM and migration workflow), `auth` (the token model), and
`commands` (the daily loop).
