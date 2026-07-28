---
title: API documentation
description: Swagger docs generated automatically — no annotations, ever.
---

Runtime applications document themselves. There are no doc
comments, no `swag init`, no YAML to maintain: the OpenAPI 3.0.3 document is
built at runtime from two sources —

1. **The live route table.** Every registered route appears in the spec, with
   path parameters inferred from `:id` segments and the standard response
   envelope.
2. **Typed descriptions emitted by generators.** Scaffolded handlers (auth,
   tasks, health) and everything from `gin-kit generate resource` carry
   `docs.Describe(...)` calls in the generated code — request/response
   schemas are reflected from the real Go structs, validation tags become
   `required`/length constraints, and list endpoints document their filter,
   sort, and pagination parameters.

Open `/docs` in development for the Swagger UI, or fetch `/openapi.json`
directly. The spec is built lazily on the first request, so it captures every
route your application registers at boot.

## Configuration

Everything is driven by `.env`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `DOCS_ENABLED` | `true` in development, `false` otherwise | Serve the docs at all |
| `DOCS_PATH` | `/docs` | Swagger UI page |
| `DOCS_SPEC_PATH` | `/openapi.json` | Raw OpenAPI document |
| `DOCS_TITLE` | project name | Spec title |
| `DOCS_VERSION` | `0.1.0` | Spec version |
| `DOCS_DESCRIPTION` | — | Spec description |
| `DOCS_SERVERS` | — | Comma-separated server URLs |
| `DOCS_BASIC_AUTH_USERNAME` / `DOCS_BASIC_AUTH_PASSWORD` | — | Protect the docs and spec with HTTP basic auth |

Docs are off in production unless you opt in; when you do, pair
`DOCS_ENABLED=true` with the basic-auth variables (or your ingress rules) if
the API is not meant to be publicly explorable.

## Security schemes

Operations registered with `Security: true` (the scaffolded `/api/v1/me`, for
example) carry a `bearerAuth` requirement, and the JWT bearer scheme is added
to the spec automatically — Swagger UI shows the Authorize button with no
extra work. Replace the scheme when your API uses something else:

```go
application.OpenAPI().SetSecurityScheme("apiKey", openapi.SecurityScheme{
    Type: "apiKey", In: "header", Name: "X-API-Key",
})
```

## Describing your own routes

Routes you write by hand are documented automatically from the route table.
To enrich them with typed schemas, add one `Describe` call next to the route
registration — the same thing the generators emit:

```go
docs.Describe(openapi.Operation{
    Method: http.MethodGet, Path: "/api/v1/reports/:id",
    Summary: "Get a report", Tags: []string{"reports"},
    Response: Report{}, Security: true,
    ErrorCodes: []string{"report_not_found"},
})
```

The standalone project type keeps its editable static `api/openapi.yaml` instead —
its philosophy is source-visible, hand-maintained infrastructure.
