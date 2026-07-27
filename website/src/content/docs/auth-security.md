---
title: Authentication and security
description: Start with practical security defaults.
---

Enable authentication during project creation (`--auth`) to receive a
complete, working authentication vertical in both editions: `users` and
`refresh_tokens` migrations, repositories for GORM and sqlx, an auth service
with Argon2id password hashing and timing-leveled login, and JSON endpoints —
`POST /api/v1/auth/register`, `/login`, `/refresh`, `/logout`, and a
database-backed `GET /api/v1/me`. Refresh tokens are stored hashed and are
single-use: every refresh rotates the token and revokes the old one.

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

## Personal API tokens

`auth.NewAPIToken` creates a random `gk_`-prefixed secret. Return the
plaintext once and persist only `APIToken.TokenHash`; when `expiresIn` is
omitted the token does not expire. Protect routes separately from JWT auth:

```go
protected := router.Group(
    "/api/v1/imports",
    auth.RequireToken(tokens, "imports:write"),
)
```

All requested abilities must be present, while a stored `*` ability matches
every requirement. `auth.TokenFromContext` returns the verified token and
`auth.Can` performs an individual ability check. Token stores should scope
management operations to the authenticated owner, reject revoked tokens, and
update `last_used_at`; never log or persist the plaintext secret.

## Password hashing

`framework/password` provides the Argon2id primitives the auth service is
built on, usable on their own:

```go
encoded, err := password.Hash(plain)          // Argon2id with sane defaults
ok := password.Compare(plain, encoded)         // constant-time verify

hasher := password.New(password.Parameters{    // tune costs explicitly
    Memory: 64 * 1024, Iterations: 3, Parallelism: 2,
    KeyLength: 32, SaltLength: 16,
})
if hasher.NeedsRehash(encoded) { /* re-hash on next successful login */ }
```

Parameters are encoded into the hash string, so `Compare` verifies old hashes
after you raise costs, and `NeedsRehash` tells you when to upgrade them.

Client addresses are spoof-resistant by default: `X-Forwarded-For` is honored
only from proxies listed in `HTTPOptions.TrustedProxies` — set
`TRUSTED_PROXY_CIDRS` in generated projects — and no proxy is trusted until you
do so.

The primitives are intentionally not a complete policy engine. Add your own
user store, claims, authorization checks, token rotation policy, and audit
events in application code. Secrets belong in the environment or a secret
manager, never in generated source.
