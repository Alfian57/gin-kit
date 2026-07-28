---
title: Runtime API map
description: Find the explicit gin-kit runtime package that owns each concern.
---

Runtime projects import versioned packages from
`github.com/Alfian57/gin-kit/runtime/...`. This map is an entry point, not a
second copy of GoDoc: use it to choose the package, follow the linked guide for
the workflow, and use the package documentation for every option and method.

## Application foundation

| Package | Start with | Guide |
| --- | --- | --- |
| [`runtime`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime) | `runtime.New`, `Application.Run`, `Application.Go` | [Customizing the runtime](/gin-kit/customization/) |
| [`runtime/config`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/config) | `LoadDotenv`, `Load`, `Config.Options` | [Configuration](/gin-kit/configuration/) |
| [`runtime/database`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/database) | `Open`, `Connection` | [Database and ORM](/gin-kit/database-orm/) |
| [`runtime/httpx`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/httpx) | `BindJSON`, `OK`, `Handle` | [Responses and validation](/gin-kit/responses-validation/) |
| [`runtime/validation`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/validation) | `New`, `RegisterRule`, `Struct` | [Validation](/gin-kit/validation/) |
| [`runtime/query`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/query) | `Parse`, `BuildSQL`, `ApplyGORM` | [Filtering and pagination](/gin-kit/querying/) |
| [`runtime/openapi`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/openapi) | `NewRegistry`, `Describe`, `Build` | [API documentation](/gin-kit/api-docs/) |

## Identity and security

| Package | Start with | Guide |
| --- | --- | --- |
| [`runtime/auth`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/auth) | `New`, `RequireAuth`, `RequireToken` | [Authentication and security](/gin-kit/auth-security/) |
| [`runtime/authz`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/authz) | `Allow`, `Deny`, `Authorize` | [Authorization](/gin-kit/authorization/) |
| [`runtime/oauth`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/oauth) | `NewManager`, `NewGoogle`, `NewGitHub` | [OAuth social sign-in](/gin-kit/oauth/) |
| [`runtime/password`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/password) | `Hash`, `Compare`, `Hasher` | [Authentication and security](/gin-kit/auth-security/) |
| [`runtime/session`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/session) | `Middleware`, `CSRF`, `PutFlash` | [Sessions and CSRF](/gin-kit/sessions/) |

## Background work, state, and delivery

| Package | Start with | Guide |
| --- | --- | --- |
| [`runtime/cache`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/cache) | `NewMemory`, `NewRedis`, `Remember` | [Caching and events](/gin-kit/caching-events/) |
| [`runtime/events`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/events) | `NewBus`, `On`, `Emit` | [Caching and events](/gin-kit/caching-events/) |
| [`runtime/queue`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/queue) | `New`, `Register`, `Dispatch` | [Queues and scheduling](/gin-kit/background-work/) |
| [`runtime/schedule`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/schedule) | `New`, `Cron`, `Every` | [Queues and scheduling](/gin-kit/background-work/) |
| [`runtime/realtime`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/realtime) | `New`, `Public`, `Private`, `SSE` | [Realtime updates](/gin-kit/realtime/) |
| [`runtime/mail`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/mail) | `New`, `NewMessage`, `Mailer.Send` | [Mail, WhatsApp, and file storage](/gin-kit/mail-storage/) |
| [`runtime/storage`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/storage) | `New`, `Put`, `SaveUpload` | [Mail, WhatsApp, and file storage](/gin-kit/mail-storage/) |
| [`runtime/whatsapp`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/whatsapp) | `New`, `NewMessage`, `NewAuthenticationMessage` | [WhatsApp Cloud API](/gin-kit/whatsapp/) |

## Operations and testing

| Package | Start with | Guide |
| --- | --- | --- |
| [`runtime/devtools`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/devtools) | `New`, `Middleware`, `Mount` | [Devtools dashboard](/gin-kit/devtools/) |
| [`runtime/metrics`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/metrics) | `New`, `Middleware`, `Handler` | [Observability](/gin-kit/observability/) |
| [`runtime/flags`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/flags) | `FromEnv`, `Enabled`, `Set` | [Customizing the runtime](/gin-kit/customization/) |
| [`runtime/factory`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/factory) | `Define`, `Seeded`, `Create` | [Seeding and factories](/gin-kit/seeding-factories/) |
| [`runtime/apptest`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/apptest) | `New`, `OpenSQLite`, `Client` | [Testing](/gin-kit/testing/) |
| [`runtime/browsertest`](https://pkg.go.dev/github.com/Alfian57/gin-kit/runtime/browsertest) | `Launch`, `StartServer`, `Install` | [Testing](/gin-kit/testing/) |

The runtime intentionally does not create hidden dependencies between these
packages. Construct the subsystem in application wiring, pass interfaces into
your services, and register background runners through `Application.Go`.
