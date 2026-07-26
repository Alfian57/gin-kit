---
title: Database and ORM
description: Choose SQL and persistence boundaries.
---

## Seeding and factories

Seeders are an explicit registry in `internal/database/seeders`, run with
`gin-kit db seed`. Each seeder receives the full database connection —
`db.SQL` plus the project's GORM or sqlx handle — so repositories work inside
them. Pair them with model factories (`internal/database/factories`,
gofakeit-backed) to seed realistic data:

```go
func SeedUsers(ctx context.Context, db *database.Connection) error {
    repo := repository.NewUserRepository(db)
    _, err := factories.NewUserFactory().CreateMany(ctx, 25, repo.Create)
    return err
}
```

Factories also work in tests: `Make` builds in-memory values, `Seeded(42)`
makes runs deterministic, and overrides tweak fields per case.

gin-kit supports MySQL, MariaDB, PostgreSQL, and SQLite. Choose GORM or sqlx
when creating a project:

```bash
gin-kit new ./billing --module example.com/acme/billing \
  --database postgres --orm sqlx
```

The database package exposes the selected `*sql.DB`, `*gorm.DB`, or `*sqlx.DB`
so you can use the ORM where it helps and raw SQL where it is clearer. Goose
migrations are versioned SQL files. gin-kit never runs production `AutoMigrate`.
