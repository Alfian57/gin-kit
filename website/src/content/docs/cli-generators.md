---
title: CLI and generators
description: Use the GinKit command line without ceremony.
---

```text
ginkit new <path>
ginkit run
ginkit build
ginkit check
ginkit doctor
ginkit explain <topic>
ginkit generate handler|service|domain|repository|middleware|migration|resource <name>
ginkit db up|down|status
```

Commands use Go-native verbs and nouns. GinKit intentionally does not imitate
Laravel’s `make:*` or Artisan conventions.

Generators preflight output paths, refuse accidental overwrites, render
transactionally, and support `--dry-run`. Errors name the failed phase, stable
code, affected path, and a recovery hint.
