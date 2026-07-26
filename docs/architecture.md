# gin-kit Architecture

gin-kit separates its versioned runtime, project CLI, and application-owned
code. The framework owns generic HTTP and lifecycle policy; the generated
project owns its business behavior and integration choices.

## Runtime flow

```text
HTTP request
  -> router
  -> handler (bind + validate request DTO, wrap response DTO)
  -> service (business rules; DTO in, domain value out)
  -> repository (persistence)
  -> database
```

Each application boundary has a small constructor. gin-kit does not use a
reflection-based container or service locator.

Transport shapes live in `internal/dto`, one file per model:
`Create<Name>Request` and `Update<Name>Request` carry validation tags and a
`Normalize()` trimmer; `<Name>Response` decides exactly what leaves the API
through explicit `New<Name>Response` mappers — no persistence tags, and
credential-like fields are excluded by the generator. `internal/domain` holds
models and repository interfaces and never depends on HTTP types.

## Editions

- `framework` is the default. Generic lifecycle, HTTP middleware, responses,
  validation, and security primitives come from the pinned gin-kit module.
- `starter` contains the infrastructure implementation in the generated
  project for developers who want a standalone, source-visible reference.

Both editions keep routes, handlers, services, domains, repositories,
migrations, configuration, and selected database integration explicit.

## Runtime foundation

The framework application validates configuration, opens the selected SQL
integration, installs recovery and security middleware, registers health
checks, and coordinates graceful shutdown. Liveness does not depend on external
services; readiness checks may include the database with a short timeout.

Default middleware provides request IDs, structured request logging, security
headers, request body limits, restrictive CORS, trusted-proxy-aware client
addresses, panic recovery, and endpoint-class rate limiting. Forwarded client
addresses are honored only from proxies listed in `HTTPOptions.TrustedProxies`
(`TRUSTED_PROXY_CIDRS` in generated projects); by default no proxy is trusted.

API handlers use the framework response and validation packages. Public error
responses never contain Go internals, stack traces, database errors, tokens, or
submitted secret values; diagnostics are correlated through the request ID.

## Customization

Applications can add Gin middleware, access the raw `*gin.Engine`, register
validation rules and translations, map domain errors, add readiness checks, and
register shutdown hooks. Database adapters expose their selected `*sql.DB`,
`*gorm.DB`, or `*sqlx.DB`. Teams that need to change the core can fork the
module and use a standard Go `replace` directive.
