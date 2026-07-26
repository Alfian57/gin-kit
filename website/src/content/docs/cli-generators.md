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
gin-kit build             # build a production binary
gin-kit check             # read-only: report gofmt drift, run tests and vet
gin-kit doctor            # diagnose toolchain, manifest, and database issues
gin-kit routes            # boot the app and print the sorted routing table
gin-kit explain <topic>   # architecture | request-flow | database | auth | commands
```

`new` defaults to `--edition framework`; use `--edition starter` for the
standalone source-visible edition, `--auth` for the authentication vertical,
`--example` for the tasks example, and `--docker` for compose files.
Non-interactive creation requires `--module`, `--mode`, `--database`, and
`--orm`.

`routes` boots the application, so it needs a reachable database (SQLite
always works) and valid required secrets.

## Generators

```text
gin-kit generate resource <Name> --fields "..." [--table name]
gin-kit generate domain <Name> [--fields ...]
gin-kit generate dto <Name> [--fields ...]
gin-kit generate repository <Name> [--fields ...]
gin-kit generate handler <Name>
gin-kit generate service <Name>
gin-kit generate middleware <Name>
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

`generate dto` renders just the DTO file for an existing model.
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
