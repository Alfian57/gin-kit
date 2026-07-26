---
title: Deployment
description: Ship a generated gin-kit service confidently.
---

Run `gin-kit build` to produce the application binary. Generated Dockerfiles
use a non-root runtime image and a health check. Set production configuration
through your platform's secret store and expose `/health/live` and
`/health/ready` to your orchestrator — liveness never depends on external
services, readiness may check the database and Redis with short timeouts.

## Production checklist

- **Secrets**: `JWT_SECRET` (with `--auth`) and `SESSION_SECRET` (UI mode)
  must be long and random; short secrets fail startup. `DATABASE_URL` and
  `CORS_ALLOWED_ORIGINS` are required outside development.
- **Docs off**: the auto-generated API docs default to development-only. Keep
  `DOCS_ENABLED` unset in production, or set basic-auth credentials if you
  need them reachable.
- **Proxies**: set `TRUSTED_PROXY_CIDRS` to your load balancer's CIDRs —
  forwarded client IPs are ignored otherwise, which is correct but makes rate
  limiting per-LB instead of per-client.
- **Observability**: enable `METRICS_ENABLED` for Prometheus scraping; never
  expose `PPROF_ENABLED` publicly.
- **Migrations**: keep `gin-kit db up` (or the `cmd/migrate` binary) as a
  separate, reviewed deployment step — the server never auto-migrates.

## Background work at scale

With `QUEUE_DRIVER=redis`, job handlers run inside the application process —
the same binary serves HTTP and drains its queues, supervised by the
application lifecycle. Run more replicas to add workers, and tune
`QUEUE_CONCURRENCY` per instance.

The scheduler is **single-instance**: every replica runs every cron job. If
you scale beyond one replica, either keep scheduled work idempotent or run a
dedicated scheduler replica.

## Docker Compose profiles

Framework-edition compose files ship optional services behind profiles:
`docker compose --profile mail up` starts Mailpit for a local inbox;
`--profile storage` starts MinIO for S3-compatible storage. Neither runs by
default.

CI should run `gin-kit check`, tests, vet, and a generated-project smoke
test. Build the same module and Go version locally and in CI.
