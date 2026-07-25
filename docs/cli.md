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

The command tree intentionally uses Go-native verbs and nouns. It does not implement Laravel's `make:*` or Artisan conventions.

## Generated runtime configuration

Generated projects read configuration from environment variables. Development
uses local defaults where safe. Staging and production fail startup when
database, CORS, or enabled-authentication secrets are missing or invalid.

Rate limiting is enabled by default and exposes separate per-minute settings
for general, authentication, and expensive endpoint classes. Forwarded client
IP headers are trusted only when `TRUSTED_PROXY_CIDRS` is configured.
