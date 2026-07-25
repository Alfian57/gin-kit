# CLI Reference

## Project commands

```text
ginkit new <name>
ginkit run
ginkit build
ginkit check
ginkit doctor
ginkit explain <topic>
```

`ginkit new` defaults to `--edition framework`. Use `--edition starter` for the
standalone source-visible edition. Use `--example` to include the runnable
Tasks vertical slice. Generated projects expose the same
domain/service/repository boundaries regardless of whether GORM or sqlx is
selected.

Non-interactive creation requires the module, mode, database, and ORM:

```text
ginkit new ./services/orders --non-interactive \
  --edition framework \
  --module example.com/acme/orders \
  --mode api \
  --database postgres \
  --orm gorm
```

## Generation

```text
ginkit generate handler <name>
ginkit generate service <name>
ginkit generate domain <name>
ginkit generate repository <name>
ginkit generate middleware <name>
ginkit generate migration <name>
ginkit generate resource <name>
```

## Database

```text
ginkit db up
ginkit db down
ginkit db status
```

The command tree intentionally uses Go-native verbs and nouns. It does not
implement Laravel's `make:*` or Artisan conventions.

`ginkit check` is read-only: it reports files that need formatting, then runs
tests and vet. It never rewrites source files.

Generators preflight every output, refuse overwrites, and write transactionally
so a failed generation does not leave a partial resource. Use `--dry-run` to
inspect intended files.

## Generated runtime configuration

Generated projects read configuration from environment variables. Development
uses local defaults where safe. Staging and production fail startup when
database, CORS, or enabled-authentication secrets are missing or invalid.

Rate limiting is enabled by default and exposes separate per-minute settings
for general, authentication, and expensive endpoint classes. Forwarded client
IP headers are trusted only when `TRUSTED_PROXY_CIDRS` is configured.
