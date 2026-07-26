---
title: Configuration
description: Configure an application safely across environments.
---

Generated projects load environment variables through typed configuration.
Required database credentials, CORS origins, and authentication secrets are
validated before the server starts.

Framework-edition applications use the `framework/config` package:

```go
if err := config.LoadDotenv(".env"); err != nil {
    return nil, err
}
cfg, err := config.Load()
if err != nil {
    return nil, err
}
options := cfg.Options() // then set UI, Database, Readiness, ...
```

`LoadDotenv` reads `KEY=VALUE` pairs and never overrides real environment
variables, so the environment always wins over `.env`. `Load` reads `PORT`,
`APP_ENV`, `DATABASE_URL`, `JWT_SECRET`, `TRUSTED_PROXY_CIDRS`,
`CORS_ALLOWED_ORIGINS`, `RATE_LIMIT_*`, `MAX_BODY_BYTES`, and the
`READ/WRITE/IDLE/SHUTDOWN_TIMEOUT` durations. Malformed values are startup
errors, and outside development `DATABASE_URL` and `CORS_ALLOWED_ORIGINS` are
required.

Keep `.env` local and commit only `.env.example`. The framework refuses unsafe
production defaults, applies request timeouts and body limits, and accepts
trusted proxy CIDRs explicitly. Liveness never depends on a database; readiness
may check it with a short timeout.
