---
title: Architecture
description: Understand the boundaries in a generated gin-kit project.
---

gin-kit follows a small, visible flow:

```text
HTTP request → router → handler → service → repository → database
```

Handlers bind and validate request DTOs from `internal/dto`, pass them to
services, and wrap the returned domain values in response DTOs. Services own
business decisions: they accept request DTOs and return domain values, never
HTTP types. Repositories own persistence. `internal/domain` holds models and
repository interfaces; `internal/dto` decides what enters and leaves the API —
requests carry validation tags, responses exclude credential-like fields.
Constructors make each boundary explicit; gin-kit does not use a
reflection-based container or service locator.

The runtime owns generic HTTP policy, recovery, request IDs,
configuration validation, database lifecycle, and graceful shutdown. The
generated project owns routes, business behavior, integrations, and
deployment configuration.
