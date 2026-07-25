# GinKit Architecture

GinKit separates its versioned runtime, project CLI, and application-owned
code. The framework owns generic HTTP and lifecycle policy; the generated
project owns its business behavior and integration choices.

## Runtime flow

```text
HTTP request
  -> router
  -> handler (bind, validate, translate)
  -> service (business rules)
  -> repository (persistence)
  -> database
```

Each application boundary has a small constructor. GinKit does not use a
reflection-based container or service locator.

## Editions

- `framework` is the default. Generic lifecycle, HTTP middleware, responses,
  validation, and security primitives come from the pinned GinKit module.
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
addresses, panic recovery, and endpoint-class rate limiting.

API handlers use the framework response and validation packages. Public error
responses never contain Go internals, stack traces, database errors, tokens, or
submitted secret values; diagnostics are correlated through the request ID.

## Customization

Applications can add Gin middleware, access the raw `*gin.Engine`, register
validation rules and translations, map domain errors, add readiness checks, and
register shutdown hooks. Database adapters expose their selected `*sql.DB`,
`*gorm.DB`, or `*sqlx.DB`. Teams that need to change the core can fork the
module and use a standard Go `replace` directive.
