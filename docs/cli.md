# CLI Reference

## Project commands

```text
gin-kit new <name>
gin-kit run
gin-kit dev [--port]
gin-kit build
gin-kit check
gin-kit upgrade [--diff] [--apply] [--force]   # starter edition only
gin-kit doctor
gin-kit explain <topic>
```

`gin-kit dev` is the hot-reload development server: it builds `./cmd/server`
to a binary, watches the project, and rebuilds on change. A local reverse
proxy on `--port` (default 8080) holds incoming requests while a rebuild is in
flight instead of dropping them, and compile errors render as a browser
overlay (plain text for non-HTML clients) until the next successful build. The
application itself listens on `--app-port` (default: an automatically chosen
free port, passed to the server as `PORT`). `gin-kit run` remains the simple
`go run`-based runner.

`gin-kit new` defaults to `--edition framework`. Use `--edition starter` for the
standalone source-visible edition. Use `--example` to include the runnable
Tasks vertical slice. Generated projects expose the same
domain/service/repository boundaries regardless of whether GORM or sqlx is
selected.

Non-interactive creation requires the module, mode, database, and ORM:

```text
gin-kit new ./services/orders --non-interactive \
  --edition framework \
  --module example.com/acme/orders \
  --mode api \
  --database postgres \
  --orm gorm
```

## Generation

```text
gin-kit generate resource <Name> --fields "title:string,done:bool" [--table name]
gin-kit generate domain <Name> [--fields ...]
gin-kit generate dto <Name> [--fields ...]
gin-kit generate repository <Name> [--fields ...]
gin-kit generate handler <Name>
gin-kit generate service <Name>
gin-kit generate middleware <Name>
gin-kit generate policy <Name>
gin-kit generate seeder <Name>
gin-kit generate factory <Name> --fields "email:string,age:int"
gin-kit generate migration <name>
gin-kit generate job <Name>      # framework edition only
gin-kit generate event <Name>    # framework edition only
gin-kit generate mail <Name>     # framework edition only
```

`generate resource` renders a complete vertical slice from real,
manifest-aware templates: domain model with a repository interface, request
and response DTOs in `internal/dto`, a GORM or sqlx repository (per the
project manifest), a service that accepts DTOs and returns domain values, an
HTTP handler with query filtering/sorting/pagination, tests with in-memory
repository fakes, and a timestamped goose migration with dialect-mapped
column types.

`generate dto` renders just the DTO file for an existing domain model:
`Create<Name>Request` and `Update<Name>Request` with validation tags and a
`Normalize()` trimmer, plus `<Name>Response` with explicit mappers.
Credential-like fields (`password`, `secret`, `token`, `hash`) are excluded
from the response type.

`generate policy` renders an authorization policy in `internal/policy`:
`CanView`/`CanCreate`/`CanUpdate`/`CanDelete` methods returning
`authz.Decision` values (deny reasons are logged, never serialized) plus a
table test. It works in both editions; starter projects missing the vendored
`internal/platform/authz` package get it back-filled automatically.

The `--fields` grammar is `name:type` pairs separated by commas, with types
`string`, `text`, `int`, `int64`, `float64`, `bool`, and `time` (aliases:
`float`, `datetime`, `timestamp`); a trailing `?` makes a field nullable.
`id`, `created_at`, and `updated_at` are always generated. String fields get
partial-match filters, booleans exact boolean filters, and numeric/time
fields comparison filters. Validation tags follow nullability: required
strings validate as `required,max=255`, nullable strings as
`omitempty,max=255`.

## Database

```text
gin-kit db up
gin-kit db down
gin-kit db status
gin-kit db redo
gin-kit db reset --yes
gin-kit db create <name>
gin-kit db seed
gin-kit db fresh --yes
```

Seeders live in `internal/database/seeders` as an explicit registry — generate
one with `gin-kit generate seeder <Name>` and add it to `All()` by hand.
Seeders receive the full database connection (`db.SQL` plus the project's
GORM or sqlx handle), so repositories and model factories work inside them.
Factories live in `internal/database/factories`; `generate resource` emits
one automatically, and `generate factory` creates standalone ones with
field-aware fake data (gofakeit). `db
fresh` resets the schema, migrates up, and seeds; `reset` and `fresh` are
destructive and require `--yes`. Server, migrate, and seed binaries all load
`.env` (the real environment always wins).

## Upgrading starter projects

Starter projects vendor their runtime under `internal/platform/`, so bugfixes
in newer gin-kit releases do not reach them through `go get`. `gin-kit
upgrade` closes that gap: it re-renders the **current CLI's** platform
templates for the project's manifest (mode, ORM, and feature gates all
apply), compares them with the files on disk, and reports one status per
file:

| Status | Meaning | On `--apply` |
| --- | --- | --- |
| `up-to-date` | disk matches the render | untouched |
| `outdated` | disk matches the recorded baseline, not the render — a stale vendored copy | updated |
| `modified` | disk matches neither render nor baseline — local edits | skipped unless `--force` |
| `differs` | disk differs and no baseline entry exists — unverifiable | skipped unless `--force` |
| `missing` | the rendered file is absent on disk | created |
| `unmanaged` | an on-disk `internal/platform` file the current templates do not render | never touched |

Without flags the command only reports (exit code 0). `--diff` prints a
unified diff for every file that differs, `--apply` writes the safe updates
transactionally (staged first, then published), and `--apply --force` also
overwrites `modified` and `differs` files. Go files are compared after
`gofmt` normalization, so formatting drift between Go versions never counts
as a change.

The baseline lives in `.gin-kit.sum` — sorted `sha256  path` lines recording
the checksum each platform file had when the CLI last wrote it. New starter
scaffolds create it automatically; projects created before it existed start
with every changed file reported as `differs`, and the first
`upgrade --apply` bootstraps baseline entries for all files that match the
render. Commit the file: it is what lets a future upgrade distinguish your
edits from stale vendored code.

Framework-edition projects do not vendor the runtime, so `gin-kit upgrade`
refuses to run there (`upgrade_edition_unsupported`); upgrade the versioned
module instead: `go get github.com/Alfian57/gin-kit@vX.Y.Z && go mod tidy`.

## Routes

`gin-kit routes` boots the application (a reachable database is required —
SQLite always works) and prints the sorted routing table with handlers.

The command tree uses Go-native verbs and nouns, and every building block a
project needs has a generator. Routes are never registered automatically:
each generator prints the exact wiring snippet to paste, keeping application
wiring explicit and reviewable.

`gin-kit check` is read-only: it reports files that need formatting, then runs
tests and vet. It never rewrites source files.

Generators preflight every output, refuse overwrites, and write transactionally
so a failed generation does not leave a partial resource. Use `--dry-run` to
inspect intended files.

## Generated runtime configuration

Generated projects read configuration from environment variables, and load a
local `.env` file at startup without overriding the real environment.
Development uses local defaults where safe. Staging and production fail startup
when database, CORS, or enabled-authentication secrets are missing or invalid,
and malformed values are always startup errors.

Framework-edition projects use `framework/config` with `PORT`, `APP_ENV`,
`DATABASE_URL`, `JWT_SECRET`, `TRUSTED_PROXY_CIDRS`, `CORS_ALLOWED_ORIGINS`,
`RATE_LIMIT_ENABLED`, `RATE_LIMIT_PER_MINUTE`, `RATE_LIMIT_BURST`,
`MAX_BODY_BYTES`, `METRICS_ENABLED`, `PPROF_ENABLED`, and
`READ/WRITE/IDLE/SHUTDOWN_TIMEOUT` Go durations.

Starter-edition rate limiting is enabled by default and exposes separate
per-minute settings for general, authentication, and expensive endpoint
classes. In both editions, forwarded client IP headers are trusted only when
`TRUSTED_PROXY_CIDRS` is configured.
