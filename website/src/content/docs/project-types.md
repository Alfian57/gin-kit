---
title: Project types
description: Choose the right gin-kit project type for your team.
---

gin-kit has two project types. They differ only in where the generic runtime
code lives; your routes, handlers, services, domains, repositories, migrations,
and UI stay editable in either project.

| | Runtime | Standalone |
| --- | --- | --- |
| Core runtime | Versioned gin-kit dependency | Copied into the project |
| Best for | Production teams and long-lived services | Learning, auditing, or full ownership |
| Generic tests | In the gin-kit repository | In your generated project |
| Customization | Options, hooks, middleware, and `replace` | Edit the source directly |
| CLI value | `runtime` (default) | `standalone` |

API and UI are application modes, not project types. Standalone is not a UI or
authentication scaffold; it is a source-included runtime choice.

## What the standalone vendors

Standalone projects carry copies of the runtime under
`internal/platform/`: `config`, `database`, `factory`, `query`, `httpx`
(response envelope and request binders), `validation`, and — for UI projects —
`session`. The HTTP contract is identical in both project types: the same JSON
envelope, the same `422` validation details, the same binder helpers. Handler
code moves between project types with only an import-path change.
