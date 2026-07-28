---
title: Seeding and factories
description: Deterministic sample data for development, tests, and demos.
---

gin-kit pairs an explicit seeder registry with typed model factories, so the
same building blocks fill a development database, drive integration tests,
and power demo environments.

## Model factories

`runtime/factory` (vendored as `internal/platform/factory` in standalone
projects) defines factories as plain generic values — no reflection, no
struct tags:

```go
var Tickets = factory.Define(func(f *factory.F) domain.Ticket {
    return domain.Ticket{
        ID:        f.UUID(),
        Title:     strings.TrimSuffix(f.Sentence(3), "."),
        CreatedAt: f.PastDate().UTC(),
    }
})
```

The `*factory.F` argument wraps [gofakeit](https://github.com/brianvoe/gofakeit)
plus a per-factory sequence (`f.Seq()`, `f.Seqf("user-%d")`) for unique
values.

| Method | What it does |
| --- | --- |
| `Define(build)` | Declare a factory from a build function. |
| `Make(overrides...)` | One in-memory value; overrides mutate the built value. |
| `MakeMany(n, overrides...)` | A slice of n values. |
| `Create(ctx, persist, overrides...)` | Build, then persist through any `func(ctx, *T) error` — a repository `Create` fits directly. |
| `CreateMany(ctx, n, persist, overrides...)` | Persist n values. |
| `Seeded(seed)` | Deterministic output for reproducible tests. |

```go
ticket := factories.Tickets.Make(func(t *domain.Ticket) { t.Title = "Fixed title" })
persisted, err := factories.Tickets.Create(ctx, repo.Create)
```

`generate resource` emits a factory per model into
`internal/database/factories/`; `gin-kit generate factory <Name> --fields ...`
creates a standalone one with field-aware fake data (emails get `f.Email()`,
prices get `f.Price(...)`, string fields ending in `_id` get `f.UUID()`, and
so on).

## Relations

There is no relation combinator on purpose: a `BelongsTo`-style helper would
hide the parent value you almost always need right after creating it.
Compose relations explicitly with overrides — create the parent first, then
point the child's foreign key at it:

```go
user, _ := factories.NewUserFactory().Create(ctx, users.Create)
ticket, _ := factories.NewTicketFactory().Create(ctx, tickets.Create,
    func(t *domain.Ticket) { t.UserId = user.ID })
```

The same override works for in-memory values and batches:

```go
draft := factories.NewTicketFactory().Make(
    func(t *domain.Ticket) { t.UserId = user.ID })

batch, _ := factories.NewTicketFactory().CreateMany(ctx, 5, tickets.Create,
    func(t *domain.Ticket) { t.UserId = user.ID })
```

Generated factories fake `*_id` string fields as UUIDs, so unset foreign
keys at least carry the right shape — but always override them with a real
parent ID before persisting against a foreign-key constraint.

## Seeders

Seeders live in `internal/database/seeders/` as an **explicit registry** — a
plain slice returned by `All()`, run in order by `gin-kit db seed`:

```go
func All() []Seeder {
    return []Seeder{
        {Name: "tickets", Run: SeedTickets},
    }
}
```

Generate one and register it by hand (nothing edits your files):

```bash
gin-kit generate seeder Tickets
```

Each seeder receives the full database connection — `db.SQL` plus the
project's GORM or sqlx handle — so repositories and factories work inside
them. Make seeders idempotent: check for existing rows before inserting, the
way the generated tasks seeder does.

## Database workflow

```bash
gin-kit db up          # migrate to latest
gin-kit db seed        # run the seeder registry
gin-kit db fresh --yes # drop everything, migrate, seed — the full reset
```

`fresh` and `reset` are destructive and refuse to run without `--yes`. See
[CLI and generators](/gin-kit/cli-generators/) for the complete `db` command
tree, and [Testing](/gin-kit/testing/) for using factories with the
in-memory SQLite integration harness.
