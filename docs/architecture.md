# GinKit Architecture

GinKit separates the project generator from the generated application. The CLI owns project creation and developer workflow; the generated project owns runtime behavior.

## Runtime flow

```text
HTTP request
  -> router
  -> handler (bind, validate, translate)
  -> service (business rules)
  -> repository (persistence)
  -> database
```

Each boundary has a small constructor. This keeps dependencies visible and makes the generated source approachable for a new Go developer.

## Runtime foundation

The application composition root loads and validates typed configuration,
opens the selected SQL database, configures Gin middleware, registers health
routes, and returns an explicit cleanup function. Liveness does not depend on
external services; readiness pings the database with a short timeout.

Generated middleware provides request IDs, security headers, request body
limits, restrictive CORS, trusted-proxy-aware client addresses, and in-memory
rate limiting. The default limiter has separate policy classes for general,
authentication, and expensive endpoints.

Authentication helpers reject short signing secrets, validate the expected JWT
algorithm, use Argon2id password hashes, and generate opaque refresh tokens
whose database representation is a SHA-256 hash.

## Why global layers?

GinKit uses global layers because the first generated project should make the request flow easy to discover. `ginkit generate resource` creates coordinated files in those layers, while the developer remains free to reorganize the project later.

## Standalone output

Generated projects contain their own server and migration binaries. GinKit is not imported by the application, so the project can be cloned, built, deployed, and maintained independently.
