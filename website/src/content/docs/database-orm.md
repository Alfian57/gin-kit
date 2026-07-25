---
title: Database and ORM
description: Choose SQL and persistence boundaries.
---

GinKit supports MySQL, MariaDB, PostgreSQL, and SQLite. Choose GORM or sqlx
when creating a project:

```bash
ginkit new ./billing --module example.com/acme/billing \
  --database postgres --orm sqlx
```

The database package exposes the selected `*sql.DB`, `*gorm.DB`, or `*sqlx.DB`
so you can use the ORM where it helps and raw SQL where it is clearer. Goose
migrations are versioned SQL files. GinKit never runs production `AutoMigrate`.
