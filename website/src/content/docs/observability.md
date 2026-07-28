---
title: Observability
description: Metrics, request-scoped logging, and profiling.
---

## Prometheus metrics

Enable the scrape endpoint with `METRICS_ENABLED=true` (runtime projects) or
directly through options:

```go
app, err := runtime.New(runtime.Options{
    Metrics: runtime.MetricsOptions{Enabled: true}, // serves GET /metrics
})
```

Every request is recorded as `http_requests_total{method,route,status}`,
`http_request_duration_seconds{method,route}`, and `http_requests_in_flight`.
The `route` label uses the matched pattern (`/tasks/:id`, never `/tasks/123`),
so path parameters cannot explode label cardinality; unmatched requests are
grouped under `unmatched`. Register custom metrics on
`app.Metrics().Registry()`.

## Request-scoped logging

Handlers retrieve a logger already carrying the request ID, method, and path:

```go
httpx.Logger(c).Info("task created", "task_id", task.ID)
```

Inject your base logger through `Options.Logger`; access logs and the
request-scoped logger share it. Without the runtime middleware the accessor
falls back to `slog.Default()`, so handler code stays testable.

## Profiling

`PPROF_ENABLED=true` (or `runtime.PProfOptions{Enabled: true}`) mounts the
standard library profiler under `/debug/pprof`.

Never expose `/metrics` or `/debug/pprof` publicly: keep them behind your
ingress rules or network policy. The pprof endpoints reveal process internals
and can degrade performance under profiling load.
