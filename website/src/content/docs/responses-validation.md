---
title: Responses and validation
description: Predictable contracts for every request.
---

gin-kit returns a consistent JSON envelope. Validation failures use HTTP `422`
and include field-level details:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "The given data was invalid.",
    "details": {
      "fields": {
        "email": [
          {
            "rule": "required",
            "message": "The email field is required.",
            "parameters": {}
          }
        ]
      }
    },
    "request_id": "01HXZ8Q5Y3Z6J7K8M9N0P1Q2R3"
  }
}
```

Use `httpx.BindJSON[T]` in handlers. It maps malformed JSON to `400
invalid_json`, validation to `422 validation_failed`, and resolves field names
from JSON/form tags. `httpx.OK`, `Created`, `List`, `NoContent`, and `Fail`
keep success and failure shapes consistent.

`httpx.BindQuery[T]` and `httpx.BindURI[T]` bring the same contract to query
and path parameters:

```go
type searchQuery struct {
    Term string `form:"term" validate:"required,min=3"`
    Page int    `form:"page"`
}

query, ok := httpx.BindQuery[searchQuery](c)
if !ok {
    return // 400 invalid_query or 422 validation_failed already written
}
```

`BindQuery` binds through `form` tags and answers `400 invalid_query` on
malformed input; `BindURI` binds through `uri` tags and answers
`400 invalid_path`. Both validate with the same rules and never echo submitted
values.

### Which validator runs

Binders resolve the validator in a fixed order:

1. An explicit argument — `httpx.BindJSON[T](c, myValidator)` — always wins.
2. The application validator from `Options.Validator`. The framework places it
   on the request context, so custom rules and messages registered through
   `app.Validator()` apply to every binder without extra wiring.
3. `validation.Default` as the fallback outside a gin-kit application (plain
   `gin.Engine` in tests, for example).

### Parity across editions

Starter projects vendor the same contract in `internal/platform/httpx` and
`internal/platform/validation`: identical envelope, identical `422` details,
identical binder behavior. Code written against one edition reads the same in
the other. In the starter, binders resolve an explicit argument first and fall
back to `validation.Default`.

List endpoints respond with the standard pagination metadata produced by the
[query builder](/gin-kit/querying/):

```json
{ "data": [...], "meta": { "page": 2, "per_page": 25, "total": 101, "total_pages": 5 } }
```

### Safe by default

Responses never include submitted values, tokens, database errors, or stack
traces. Use the request ID to find sanitized structured diagnostics in logs.
Register a custom validator, message, or error mapper when your product needs
different semantics.
