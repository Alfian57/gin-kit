---
title: CLI and generators
description: Use the gin-kit command line without ceremony.
---

```text
gin-kit new <path>
gin-kit run
gin-kit build
gin-kit check
gin-kit doctor
gin-kit explain <topic>
gin-kit generate handler|service|domain|repository|middleware|migration|resource <name>
gin-kit db up|down|status
```

Commands use Go-native verbs and nouns. gin-kit intentionally does not imitate
Laravel’s `make:*` or Artisan conventions.

Generators preflight output paths, refuse accidental overwrites, render
transactionally, and support `--dry-run`. Errors name the failed phase, stable
code, affected path, and a recovery hint.
