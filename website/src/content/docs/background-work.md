---
title: Queues and scheduling
description: Background jobs with retries and cron-style scheduled tasks.
---

## Jobs

Every application has a queue — `app.Queue()` is never nil. Handlers are
registered explicitly and payloads are typed JSON:

```go
const TypeWelcomeEmail = "emails.welcome"

type WelcomePayload struct {
    Email string `json:"email"`
}

queue.Register(app.Queue(), TypeWelcomeEmail,
    func(ctx context.Context, payload WelcomePayload) error {
        return mailer.Send(ctx, welcome(payload.Email))
    })

err := queue.Dispatch(ctx, app.Queue(), TypeWelcomeEmail,
    WelcomePayload{Email: email},
    queue.Delay(time.Minute), queue.MaxRetry(5))
```

Scaffold a typed job with `gin-kit generate job <Name>` — it renders the
payload struct, handler, and registration snippet (framework edition only;
`generate event` and `generate mail` do the same for typed events and
mailables).

The default `sync` driver executes jobs inline — perfect for development and
tests. Set `QUEUE_DRIVER=redis` (with `REDIS_URL`) for production: jobs become
persistent asynq tasks with exponential-backoff retries, delayed execution,
named queues, and `QUEUE_CONCURRENCY` worker goroutines. The worker runs as a
supervised runner inside your application process, drains in-flight jobs on
shutdown, and contributes a `redis` readiness check. For dedicated worker
binaries, call `Queue.Start(ctx)` yourself.

## Scheduled tasks

`framework/schedule` wraps robfig/cron with panic recovery and graceful stop:

```go
scheduler := schedule.New(schedule.Options{Logger: app.Logger()})
scheduler.Cron("*/5 * * * *", "sync-inventory", syncInventory)
scheduler.Every(30*time.Second, "heartbeat", heartbeat)
scheduler.Daily("prune-sessions", pruneSessions, schedule.SkipIfRunning())

app.Go("scheduler", scheduler.Run)
```

A failing or panicking job is logged and never stops the scheduler or the
application. `SkipIfRunning` prevents overlapping runs of slow jobs.

The scheduler is single-instance: running N replicas executes every job N
times. Run it in one instance or a dedicated binary when scaling out.

## Runners

Both the queue worker and the scheduler build on `app.Go(name, fn)`: any
long-running goroutine registered this way shares the application's lifecycle
— it receives cancellation on shutdown, and its failure triggers a graceful
shutdown instead of a silent hang.

For tests and short-lived binaries, `app.Close(ctx)` runs shutdown hooks
exactly once without serving HTTP — `apptest.New` calls it automatically via
`t.Cleanup`.
