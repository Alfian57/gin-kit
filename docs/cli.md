# CLI Reference

## Project commands

```text
gin-kit new <name>
gin-kit run
gin-kit build
gin-kit check
gin-kit doctor
gin-kit explain <topic>
```

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
gin-kit generate repository <Name> [--fields ...]
gin-kit generate handler <Name>
gin-kit generate service <Name>
gin-kit generate middleware <Name>
gin-kit generate seeder <Name>
gin-kit generate factory <Name> --fields "email:string,age:int"
gin-kit generate migration <name>
gin-kit generate job <Name>      # framework edition only
gin-kit generate event <Name>    # framework edition only
gin-kit generate mail <Name>     # framework edition only
```

`generate resource` renders a complete vertical slice from real,
manifest-aware templates: domain model with a repository interface, a GORM or
sqlx repository (per the project manifest), a service with typed inputs and
validation, an HTTP handler with query filtering/sorting/pagination, tests
with in-memory repository fakes, and a timestamped goose migration with
dialect-mapped column types.

The `--fields` grammar is `name:type` pairs separated by commas, with types
`string`, `text`, `int`, `int64`, `float64`, `bool`, and `time` (aliases:
`float`, `datetime`, `timestamp`); a trailing `?` makes a field nullable.
`id`, `created_at`, and `updated_at` are always generated. String fields get
partial-match filters, booleans exact boolean filters, and numeric/time
fields comparison filters.

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
