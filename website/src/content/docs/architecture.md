---
title: Architecture
description: Understand the boundaries in a generated gin-kit project.
---

gin-kit follows a small, visible flow:

```text
HTTP request → router → handler → service → repository → database
```

Handlers bind and validate input. Services own business decisions. Repositories
own persistence. Domain packages hold models and DTOs. Constructors make each
boundary explicit; gin-kit does not use a reflection-based container or service
locator.

The framework runtime owns generic HTTP policy, recovery, request IDs,
configuration validation, database lifecycle, and graceful shutdown. The
generated project owns routes, business behavior, integrations, and
deployment configuration.
