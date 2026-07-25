---
title: Configuration
description: Configure an application safely across environments.
---

Generated projects load environment variables through a typed configuration
package. Required database credentials, CORS origins, and authentication
secrets are validated before the server starts.

Keep `.env` local and commit only `.env.example`. The framework refuses unsafe
production defaults, applies request timeouts and body limits, and accepts
trusted proxy CIDRs explicitly. Liveness never depends on a database; readiness
may check it with a short timeout.
