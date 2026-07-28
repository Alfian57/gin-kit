---
title: Caching and events
description: A pluggable cache and a typed in-process event bus.
---

## Cache

Every application has a cache — `app.Cache()` is never nil. The default is an
in-memory store; set `CACHE_DRIVER=redis` and `REDIS_URL` to share it across
instances. A `redis` readiness check and shutdown cleanup are wired
automatically.

```go
store := app.Cache()
_ = store.Set(ctx, "greeting", "hello", 5*time.Minute)
value, ok, _ := store.Get(ctx, "greeting")
count, _ := store.Increment(ctx, "hits", 1)
```

`cache.Remember` memoizes expensive lookups with JSON encoding:

```go
profile, err := cache.Remember(ctx, app.Cache(), "profile:"+id, time.Minute,
    func(ctx context.Context) (Profile, error) {
        return repository.FindProfile(ctx, id)
    })
```

A cached value that no longer decodes into the requested type is treated as a
miss and recomputed, so shape changes self-heal. The in-memory driver is
unbounded and per-process — use Redis for production fleets.

## Events

`runtime/events` is a dependency-free, typed, in-process event bus.
Subscriptions are explicit — no scanning or reflection-based wiring:

```go
bus := events.NewBus()

unsubscribe := events.On(bus, func(ctx context.Context, event UserRegistered) error {
    return mailer.Send(ctx, welcomeMail(event.Email))
})

err := events.Emit(ctx, bus, UserRegistered{Email: email})
```

Dispatch is synchronous and in registration order; handler errors are joined
and panics become errors without stopping later handlers. For asynchronous
handling, subscribe a handler that dispatches a queue job and returns
immediately.
