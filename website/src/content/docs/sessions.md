---
title: Sessions, flash, and CSRF
description: Cookie sessions with CSRF protection for UI-mode projects.
---

[UI-mode](/gin-kit/ui-mode/) projects in both editions ship encrypted
cookie sessions (gin-contrib/sessions), one-shot flash messages, and CSRF
protection wired into the application. `SESSION_SECRET` (32+ bytes) drives both the signing
and encryption keys; in development an ephemeral secret is generated with a
warning, outside development it is required.

```go
// Session values
_ = session.Set(c, "theme", "dark")
theme := session.Get(c, "theme")

// Flash messages survive exactly one redirect
_ = session.PutFlash(c, "success", "Task created.")
// in the next handler:
flashes := session.Flashes(c) // returns and clears
```

## CSRF

Every state-changing request (POST/PUT/PATCH/DELETE) outside `/api/` must
carry the per-session CSRF token — in the `_csrf` form field or the
`X-CSRF-Token` header — or it is rejected with `403 csrf_token_mismatch`
(constant-time comparison). Safe methods mint the token lazily.

In HTML forms, render the hidden field:

```html
<form method="post" action="/tasks">
  {{.csrf_field}}
  ...
</form>
```

```go
c.HTML(http.StatusOK, "tasks.html", gin.H{
    "csrf_field": session.TemplateField(c),
    "flashes":    session.Flashes(c),
})
```

JSON APIs under `/api/` are exempt by default (they authenticate with Bearer
tokens, which browsers cannot send cross-site automatically); customize with
`CSRFOptions.Skipper`.
