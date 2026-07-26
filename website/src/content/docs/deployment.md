---
title: Deployment
description: Ship a generated gin-kit service confidently.
---

Run `gin-kit build` to produce the application binary. Generated Dockerfiles use
a non-root runtime image and a health check. Set production configuration
through your platform’s secret store and expose `/health/live` and
`/health/ready` to your orchestrator.

CI should run `gin-kit check`, tests, vet, and a generated-project smoke test.
Build the same module and Go version locally and in CI; keep migrations as a
separate, reviewed deployment step.
