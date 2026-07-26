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

### Safe by default

Responses never include submitted values, tokens, database errors, or stack
traces. Use the request ID to find sanitized structured diagnostics in logs.
Register a custom validator, message, or error mapper when your product needs
different semantics.
