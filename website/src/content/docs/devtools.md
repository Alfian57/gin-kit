---
title: Devtools dashboard
description: A development-only dashboard with a request log, mail outbox, route list, config report, and queue stats.
---

Framework-edition applications ship a development dashboard at `/_ginkit`: a
single page that polls the running application every two seconds. It exists
so you can see what your application just did — the request you sent, the
mail it tried to deliver, the routes it registered — without leaving the
browser.

Open it at [http://localhost:8080/_ginkit](http://localhost:8080/_ginkit)
while `gin-kit dev` (or `gin-kit run`) is serving.

## Hard development-only guarantee

The dashboard exposes request metadata, mail content, and configuration, so
it is refused outside development twice, independently:

1. `config.Load()` fails when `DEVTOOLS_ENABLED=true` and `APP_ENV` is not
   `development`. The default is enabled only in development, exactly like
   `DOCS_ENABLED`.
2. `framework.New` returns an error when
   `Options.DevTools.Enabled` is set and `Options.Environment` is not
   `"development"` — even if you bypass `config.Load` entirely.

A production build with `DEVTOOLS_ENABLED=true` does not start.

## Tabs

| Tab | Shows |
| --- | --- |
| **Requests** | The last 200 completed requests, newest first: method, path, status, duration, mapped public error code, request ID, client IP, and user agent. No bodies, no query strings, and no other headers are ever recorded. Dashboard requests are not self-recorded. |
| **Mail** | The outbox captured by the wrapped mailer (last 50 messages), sent and failed alike, with the failure error. Clicking a message opens its metadata and renders the HTML body in a sandboxed iframe (scripts disabled, no network beyond images); text-only mail renders as escaped plain text. Attachment content is never captured — only filename, content type, and size. |
| **Routes** | Every registered route: method, path pattern, and handler name. |
| **Config** | An allowlist of the documented gin-kit environment variables. Values whose keys match `SECRET`, `PASSWORD`, `KEY`, or `TOKEN` show as `[redacted]`; `*_URL` values have userinfo passwords replaced with `***`. Variables outside the allowlist are never read. |
| **Queue** | The queue driver and, for the Redis driver, per-queue pending/active/scheduled/retry/archived/completed counts from the broker. |

## Wiring the mail outbox

The request log, routes, config, and queue tabs work with no wiring. To
capture outgoing mail, wrap your mailer with the devtools recorder —
`Application.DevTools()` returns `nil` when the dashboard is disabled, so
guard the call:

```go
mailer, err := mail.New(cfg.MailOptions())
if err != nil {
    return nil, err
}
if devTools := application.DevTools(); devTools != nil {
    mailer = devTools.WrapMailer(mailer)
}
```

The wrapper records every send — success and failure — and returns the
underlying mailer's result unchanged, so application behavior is identical
with and without it.

## Environment variables

| Variable | Default | Notes |
| --- | --- | --- |
| `DEVTOOLS_ENABLED` | on in development only | Setting it outside development is a startup error. |
| `DEVTOOLS_PATH` | `/_ginkit` | Dashboard mount point; must start with `/`. |

Or directly through options:

```go
app, err := framework.New(framework.Options{
    Environment: "development",
    DevTools:    framework.DevToolsOptions{Enabled: true}, // serves GET /_ginkit
})
```

## Security notes

- The request log stores the URL path only — never the query string, request
  or response bodies, or headers other than the user agent.
- Failed requests record the *mapped public* error code (the same one the
  API envelope returned), never the internal error message.
- The config tab is a static allowlist read: nothing outside the documented
  gin-kit variables can appear, secret-like values are redacted, and URL
  passwords are scrubbed.
- Mail previews render inside a fully sandboxed iframe with a
  `sandbox; default-src 'none'` Content-Security-Policy, so a hostile
  message body cannot run scripts or reach the application origin.
- The devtools routes are excluded from the OpenAPI spec, and everything
  lives in memory — nothing is written to disk.
