---
title: Validation
description: Built-in rules, human messages, and custom validators.
---

gin-kit validates request DTOs with
[go-playground/validator](https://github.com/go-playground/validator) behind
an explicit wrapper (`framework/validation`, vendored as
`internal/platform/validation` in starter projects). Failures are reported by
JSON/form field name with human-readable messages and named parameters —
never the submitted values.

```json
{
  "fields": {
    "age": [
      {
        "rule": "gte",
        "message": "The age field must be at least 18.",
        "parameters": { "min": "18" }
      }
    ]
  }
}
```

## Built-in messages

Any `validate` tag the underlying engine supports will work; these rules ship
with polished English messages and named parameters out of the box:

| Rule | Message | Parameter |
| --- | --- | --- |
| `required` | The X field is required. | — |
| `email` | The X field must be a valid email address. | — |
| `min` | The X field must be at least N. | `min` |
| `max` | The X field must not be greater than N. | `max` |
| `len` | The X field must contain exactly N items. | `length` |
| `oneof` | The selected X is invalid. | `allowed` |
| `gte` / `lte` | must be at least / must not be greater than | `min` / `max` |
| `gt` / `lt` | must be greater than / less than | `greater_than` / `less_than` |
| `url` | The X field must be a valid URL. | — |
| `uuid` | The X field must be a valid UUID. | — |
| `numeric` | The X field must be a number. | — |
| `alphanum` | The X field must only contain letters and numbers. | — |
| `datetime` | The X field must match the LAYOUT format. | `layout` |
| `eqfield` | The X field must match the OTHER field. | `other` |
| `ne` | The X field must not be VALUE. | `disallowed` |
| `startswith` / `endswith` | must start with / must end with | `prefix` / `suffix` |

Rules without a listed message fall back to "The X field is invalid." —
register a message (below) when you use them.

## Which validator runs

The `httpx` binders resolve the validator in a fixed order:

1. An explicit argument: `httpx.BindJSON[T](c, myValidator)`.
2. The application validator from `framework.Options.Validator` — the
   framework places it on the request context, so rules registered through
   `app.Validator()` apply to every binder with no extra wiring.
3. `validation.Default` as the fallback (plain `gin.Engine` in tests, and
   always in starter projects, where `validation.Default` is the application
   validator).

## Custom rules, messages, and translations

```go
v := validation.New()

// A domain-specific rule.
err := v.RegisterRule("slug", func(fl validator.FieldLevel) bool {
    return !strings.Contains(fl.Field().String(), " ")
})

// Its message. {field} and {parameter} expand at render time.
v.RegisterMessage("slug", "The {field} field must not contain spaces.")

app, err := framework.New(framework.Options{Validator: v})
```

Every handler using `httpx.BindJSON`/`BindQuery`/`BindURI` now enforces
`validate:"slug"` and renders the custom message. To localize all messages,
replace the translator:

```go
v.SetTranslator(func(c validation.Context) string {
    // c.Field, c.Rule, c.Parameter
    return messagesByRule[c.Rule](c)
})
```

Validating outside a binder works too — the UI-mode form handlers do exactly
this:

```go
request.Normalize()
if err := validation.Default.Struct(request); err != nil {
    var failures *validation.Errors
    if errors.As(err, &failures) { /* failures.Fields */ }
}
```

See [Responses and validation](/gin-kit/responses-validation/) for the HTTP
contract these failures render as, and
[Request and response DTOs](/gin-kit/dto/) for where `validate` tags live.
