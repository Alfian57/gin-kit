---
title: Authentication and security
description: Start with practical security defaults.
---

Enable authentication during project creation (`--auth`) to receive JWT and
refresh-token primitives, Argon2id password hashing, and a Bearer-protected
sample route in both editions.

Framework-edition projects protect routes with the framework middleware:

```go
protected := router.Group("/api/v1", auth.RequireAuth(tokens))
protected.GET("/me", func(c *gin.Context) {
    httpx.OK(c, gin.H{"user_id": auth.UserID(c)})
})
```

`auth.RequireAuth` rejects missing or invalid Bearer tokens with the canonical
`401` envelope (`missing_token` / `invalid_token`) and stores the verified
claims; read them with `auth.ClaimsFromContext(c)` or `auth.UserID(c)`.

Client addresses are spoof-resistant by default: `X-Forwarded-For` is honored
only from proxies listed in `HTTPOptions.TrustedProxies` — set
`TRUSTED_PROXY_CIDRS` in generated projects — and no proxy is trusted until you
do so.

The primitives are intentionally not a complete policy engine. Add your own
user store, claims, authorization checks, token rotation policy, and audit
events in application code. Secrets belong in the environment or a secret
manager, never in generated source.
