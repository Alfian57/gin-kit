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

## Why global layers?

GinKit uses global layers because the first generated project should make the request flow easy to discover. `ginkit generate resource` creates coordinated files in those layers, while the developer remains free to reorganize the project later.

## Standalone output

Generated projects contain their own server and migration binaries. GinKit is not imported by the application, so the project can be cloned, built, deployed, and maintained independently.
