---
title: UI mode
description: Generate a small gin-kit server-rendered UI.
---

Choose `--mode ui` to generate server-rendered HTML with a distinct landing
page, static assets, and the same application lifecycle as API mode:

```bash
gin-kit new ./portal --module example.com/acme/portal \
  --mode ui --database sqlite --orm gorm
```

Templates and assets are application-owned and can be replaced with your
preferred design system. Use the raw Gin router for streaming, WebSockets, or
additional content types.

UI-mode projects come wired with encrypted cookie sessions, one-shot flash
messages, and CSRF protection — see [Sessions and CSRF](/gin-kit/sessions/).
Form handlers build [request DTOs](/gin-kit/dto/) from `PostForm` values and
validate them with `validation.Default` before calling services.
