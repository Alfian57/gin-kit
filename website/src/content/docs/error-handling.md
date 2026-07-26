---
title: Middleware and errors
description: Make failures useful without leaking internals.
---

The default middleware order is deliberate: request ID, access logging, panic
recovery, error mapping, security headers, body limits, CORS, rate limiting,
then your routes. A panic becomes a stable `500 internal_error` response and a
correlated log entry.

## Stable error codes

Every failure renders the same envelope (`{"error": {code, message, details,
request_id}}`) with a machine-readable code that will not change between
releases:

| Code | Status | Emitted when |
| --- | --- | --- |
| `invalid_json` | 400 | Malformed body, unknown fields, or trailing JSON. The payload is never echoed. |
| `invalid_query` | 400 | Query parameters fail binding, or the query builder rejects a filter/sort. |
| `invalid_path` | 400 | Path parameters fail binding. |
| `validation_failed` | 422 | Semantic validation; `details.fields` carries per-field failures. |
| `body_too_large` | 413 | The body exceeds `MAX_BODY_BYTES`. |
| `rate_limited` | 429 | The endpoint's rate-limit class is exhausted. |
| `missing_token` / `invalid_token` | 401 | Protected route without/with a bad bearer token. |
| `invalid_credentials` / `invalid_refresh_token` / `email_taken` | 401 / 401 / 409 | Auth flows. |
| `csrf_token_mismatch` | 403 | UI-mode form posts without a valid CSRF token. |
| `forbidden` | 403 | An [authorization policy](/gin-kit/authorization/) denied the action. The internal deny reason goes to logs, never to the body. |
| `<resource>_not_found` | 404 | Generated handlers, e.g. `ticket_not_found`. |
| `not_found` / `method_not_allowed` | 404 / 405 | Router-level fallbacks. |
| `internal_error` | 500 | Anything unexpected; the cause goes to logs, never to clients. |

## Mapping domain errors

`Options.ErrorMapper` turns domain errors into public responses in one place.
The mapper receives every error passed to `httpx.Handle`; return `nil`-safe
`*httpx.Error` values and fall back to the default:

```go
app, err := framework.New(framework.Options{
    ErrorMapper: func(err error, c *gin.Context) *httpx.Error {
        switch {
        case errors.Is(err, domain.ErrQuotaExceeded):
            return httpx.NewError(http.StatusPaymentRequired, "quota_exceeded",
                "The workspace quota is exhausted.")
        case errors.Is(err, domain.ErrTicketNotFound):
            return httpx.NewError(http.StatusNotFound, "ticket_not_found", "Ticket not found.")
        default:
            return httpx.DefaultMapper(err, c)
        }
    },
})
```

The default mapper already understands `*httpx.Error` values (pass-through)
and `*validation.Errors` (rendered as `422 validation_failed`); everything
else becomes `500 internal_error` with the cause preserved for logs only.

In development, logs include the operation and cause. Production responses
remain stable and concise in every environment. Configure the logger through
`Options.Logger`, and correlate any failure with its log entry via the
`request_id` field.
