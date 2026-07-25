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
