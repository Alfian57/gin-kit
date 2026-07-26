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
gin-kit generate resource <Name> --fields "title:string,done:bool,price:float64"
gin-kit generate factory <Name> --fields "email:string,age:int"
gin-kit generate domain|repository|handler|service|middleware|seeder|migration <Name>
gin-kit db up|down|status
```

`generate resource` is the flagship: it renders a working vertical slice —
domain model, GORM/sqlx repository (following the project manifest), service
with typed inputs, HTTP handler with allowlist-based filtering/sorting/
pagination, tests with in-memory fakes, and a goose migration with
dialect-mapped column types — then prints the exact wiring snippet to paste
into `internal/app/app.go`. Field types: `string`, `text`, `int`, `int64`,
`float64`, `bool`, `time`, with `?` for nullable.

Generators preflight output paths, refuse accidental overwrites, render
transactionally, and support `--dry-run`. Errors name the failed phase, stable
code, affected path, and a recovery hint.
