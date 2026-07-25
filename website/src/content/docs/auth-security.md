---
title: Authentication and security
description: Start with practical security defaults.
---

Enable authentication during project creation to receive JWT and refresh-token
primitives, Argon2id password hashing, request IDs, security headers, body
limits, restrictive CORS, trusted proxy handling, and endpoint-class rate
limiting.

The primitives are intentionally not a complete policy engine. Add your own
user store, claims, authorization checks, token rotation policy, and audit
events in application code. Secrets belong in the environment or a secret
manager, never in generated source.
