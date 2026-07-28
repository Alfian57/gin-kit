---
title: Filtering, sorting, and pagination
description: Safe list endpoints with an allowlist-based query builder.
---

The `runtime/query` package (mirrored as `internal/platform/query` in
standalone projects) parses list-endpoint parameters with a bracketed
filter syntax:

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

## Cursor pagination

Offset pagination re-scans every skipped row, so deep pages get slower, and
rows inserted or deleted between requests shift page boundaries. Cursor
(keyset) pagination remembers where the previous page ended instead, which
keeps every page equally fast and stable — prefer it for infinite scroll,
feeds, and large or frequently changing tables. The trade-off: clients can
only walk forward, and there is no total count.

Opt in per endpoint by setting `Options.CursorSort` to one of the allowed
sorts; the direction comes from `DefaultSort` (`"-created_at"` → descending):

```go
q, err := query.Parse(c, query.Options{
    AllowedSorts: []query.Sort{query.SortBy("created_at")},
    DefaultSort:  "-created_at",
    CursorSort:   "created_at",
})
if err != nil {
    httpx.Handle(c, nil, err)
    return
}
```

In cursor mode the request contract changes deliberately:

- `page` and `sort` are **rejected** with `400 invalid_query` — a keyset
  position is only meaningful for one fixed ordering, so arbitrary jumps and
  re-sorting mid-walk would silently skip or repeat rows.
- `per_page` still works (clamped to `MaxPerPage`), and filters apply as
  usual.
- `cursor` carries an opaque token (base64url) holding the last row's sort
  value and id; a malformed token is rejected with `400 invalid_query`.

Rows are always tie-broken on the `id` column (`ORDER BY <sort> DESC, id
DESC`), so rows sharing the same sort value — common with timestamps — never
straddle a page boundary ambiguously.

The query fetches one probe row past `per_page` (`CursorLimit()`) to detect
whether a next page exists; `NextCursor` trims it and encodes the next token.
With sqlx:

```go
statement, args := q.BuildCursorSQL("SELECT id, title, created_at FROM tasks")
var rows []Task
if err := db.SelectContext(c.Request.Context(), &rows, db.Rebind(statement), args...); err != nil {
    httpx.Handle(c, nil, err)
    return
}

rows, next := query.NextCursor(q, rows, func(t Task) (string, string) {
    return t.CreatedAt.UTC().Format(time.RFC3339Nano), t.ID
})
httpx.List(c, rows, q.CursorMeta(next))
```

With GORM, replace the statement building with a one-liner:

```go
err := q.ApplyCursorGORM(db.Model(&Task{})).Find(&rows).Error
```

`q.CursorMeta(next)` produces the cursor-mode metadata — `next_cursor` is
`null` on the last page:

```json
{ "next_cursor": "eyJ2IjoiMjAyNi0w...", "per_page": 25 }
```

Cursor values bind as strings, exactly like `Compare` filter values on
timestamp columns. PostgreSQL and MySQL cast the bound string to the
column's type, so an RFC 3339 UTC timestamp (`Format(time.RFC3339Nano)`)
compares correctly there. SQLite stores timestamps as text and compares them
byte-wise, so serialize the cursor value in the exact format your driver
stores (for `mattn/go-sqlite3`:
`"2006-01-02 15:04:05.999999999-07:00"` in UTC). Whatever the column type,
keep the serialization lossless and monotonic — truncating precision makes
the equality tiebreak miss rows.

## Safety

Column names reach SQL exclusively from the allowlist (validated as bare
identifiers); user values only ever bind as arguments. `Partial` escapes LIKE
wildcards with `ESCAPE '!'`, which behaves identically on MySQL, MariaDB,
PostgreSQL, and SQLite. `Bool()` values bind as native booleans so they compare
correctly against boolean columns everywhere. `In` lists are capped at 100
values.
