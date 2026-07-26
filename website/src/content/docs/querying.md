---
title: Filtering, sorting, and pagination
description: Safe list endpoints with an allowlist-based query builder.
---

The `framework/query` package (mirrored as `internal/platform/query` in
starter-edition projects) parses list-endpoint parameters in the style of
Spatie's query builder:

```
GET /tasks?filter[completed]=true&filter[title]=report
          &filter[created_at][gte]=2026-01-01
          &sort=-created_at,title&page=2&per_page=25
```

Everything is allowlist-based — the endpoint declares what it accepts:

```go
result, err := query.Parse(c, query.Options{
    AllowedFilters: []query.Filter{
        query.Exact("completed").Bool(),      // filter[completed]=true
        query.Partial("title"),               // filter[title]=report (escaped LIKE)
        query.In("priority"),                 // filter[priority]=1,2,3
        query.Compare("created_at"),          // filter[created_at][gte]=...
        query.Exact("state").Column("status"),// public name -> column mapping
    },
    AllowedSorts: []query.Sort{query.SortBy("created_at"), query.SortBy("title")},
    DefaultSort:  "-created_at",
})
if err != nil {
    httpx.Handle(c, nil, err) // 400 invalid_query with the offending keys
    return
}
```

Unknown filters or sorts, unsupported operators, and malformed pagination are
rejected with `400 invalid_query`; `per_page` is clamped to `MaxPerPage`
(default 100).

## Applying the query

With GORM:

```go
total, err := result.CountGORM(db.Model(&Task{}))
var tasks []Task
err = result.ApplyGORM(db.Model(&Task{})).Find(&tasks).Error
httpx.List(c, tasks, result.Meta(total))
```

With sqlx or database/sql (always `?` placeholders — `Rebind` adapts them):

```go
statement, args := result.BuildSQL("SELECT id, title FROM tasks")
err := db.SelectContext(ctx, &tasks, db.Rebind(statement), args...)
```

`result.Meta(total)` produces the standard pagination metadata
(`page`, `per_page`, `total`, `total_pages`) for `httpx.List`.

## Safety

Column names reach SQL exclusively from the allowlist (validated as bare
identifiers); user values only ever bind as arguments. `Partial` escapes LIKE
wildcards with `ESCAPE '!'`, which behaves identically on MySQL, MariaDB,
PostgreSQL, and SQLite. `Bool()` values bind as native booleans so they compare
correctly against boolean columns everywhere. `In` lists are capped at 100
values.
