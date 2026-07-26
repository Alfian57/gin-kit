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
gin-kit generate handler <name>
gin-kit generate service <name>
gin-kit generate domain <name>
gin-kit generate repository <name>
gin-kit generate middleware <name>
gin-kit generate migration <name>
gin-kit generate resource <name>
```

## Database

```text
gin-kit db up
gin-kit db down
gin-kit db status
```

The command tree intentionally uses Go-native verbs and nouns. It does not
implement Laravel's `make:*` or Artisan conventions.

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
`MAX_BODY_BYTES`, and `READ/WRITE/IDLE/SHUTDOWN_TIMEOUT` Go durations.

Starter-edition rate limiting is enabled by default and exposes separate
per-minute settings for general, authentication, and expensive endpoint
classes. In both editions, forwarded client IP headers are trusted only when
`TRUSTED_PROXY_CIDRS` is configured.
