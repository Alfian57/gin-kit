---
title: Framework or starter?
description: Choose the right GinKit edition for your team.
---

GinKit has two editions so learning and production concerns do not have to
compete.

| | Framework | Starter |
| --- | --- | --- |
| Core runtime | Versioned GinKit dependency | Copied into the project |
| Best for | Production teams and long-lived services | Learning, auditing, or full ownership |
| Generic tests | In the GinKit repository | In your generated project |
| Customization | Options, hooks, middleware, and `replace` | Edit the source directly |
| Default | Yes | `--edition starter` |

Neither edition hides your application code. Routes, handlers, domain types,
services, repositories, migrations, and UI templates stay in your repository.
