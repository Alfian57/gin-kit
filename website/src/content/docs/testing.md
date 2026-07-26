---
title: Testing
description: Unit, integration, and browser testing with batteries included.
---

## Unit tests with apptest

`framework/apptest` removes recorder boilerplate. Applications built with
`apptest.New` close automatically when the test finishes:

```go
app := apptest.New(t, framework.Options{})
app.Router().POST("/tasks", createTask)

var task Task
app.POST("/tasks", newTask, apptest.WithBearer(token)).
    RequireStatus(http.StatusCreated).
    Data(&task)

var meta query.Meta
app.GET("/tasks").RequireStatus(http.StatusOK).Meta(&meta)
```

Bodies adapt to their type: structs are JSON, `apptest.Form` is
form-urlencoded, `apptest.NewMultipart().Field(...).File(...)` is multipart,
and `apptest.Raw` is sent verbatim.

## Session and CSRF flows

`app.Client()` carries cookies across requests, so UI flows work end to end:

```go
client := app.Client()
token := client.CSRFToken("/tasks")      // GET, session established, token extracted
client.POST("/tasks", apptest.Form{"_csrf": {token}, "title": {"Hello"}}).
    RequireStatus(http.StatusSeeOther)
```

## Integration tests

Run repositories against a real in-memory SQLite database — no containers:

```go
conn := apptest.OpenSQLite(t, database.GORM)          // unique DB per test
apptest.Migrate(t, conn, database.SQLite, "migrations") // goose migrations
apptest.Seed(t, conn, seeders.SeedTasks)

repo := repository.NewTicketRepository(conn)
ticket, err := factories.NewTicketFactory().Create(ctx, repo.Create)
```

`gin-kit generate resource` emits a repository integration test automatically
in framework-edition projects, alongside a model factory with realistic fake
data (`Make`, `MakeMany`, deterministic `Seeded`).

## Browser tests

Framework-edition UI projects scaffold `e2e/browser_test.go` on Playwright.
Install the driver and Chromium once:

```sh
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium
```

```go
browser := browsertest.Launch(t)                 // skips when browsers absent
base := browsertest.StartServer(t, application)  // real listener on 127.0.0.1:0
page := browser.NewPage(t)
page.Goto(base + "/tasks")
```

Tests skip cleanly when Playwright is not installed (or `PLAYWRIGHT_SKIP` is
set), so `go test ./...` stays green on any machine and in CI.
