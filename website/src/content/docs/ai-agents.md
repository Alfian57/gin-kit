---
title: AI agents
description: Give coding agents a safe, useful project context.
---

Generated projects include `AGENTS.md`, provider adapters, and a GinKit
development skill. These files explain the project boundaries, test commands,
response contract, and safe migration workflow.

Keep instructions close to the code they govern. Agents should run the
generated-project checks after template changes, preserve field-level
validation errors, and never create release tags without explicit maintainer
approval. Review agent-authored changes like any other pull request.
