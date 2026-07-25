---
title: Manifest v1 migration
description: Move an older generated project to the current edition model.
---

The current CLI uses manifest v2 and defaults to the framework edition. Existing
manifest v1 applications remain buildable on their current source and are not
silently rewritten.

For a new project, create a fresh v2 scaffold, copy your domain, service,
repository, migrations, and handlers, then compare configuration and response
contracts. Run the old and new applications against a disposable database
before switching traffic. The CLI reports a migration-document link rather than
performing a risky automatic rewrite.
