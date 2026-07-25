---
title: Releasing
description: Publish GinKit from a protected main branch.
---

Releases are annotated `v0.x.y` tags created from `main`. GitHub Actions builds
the CLI, publishes checksums, and creates SLSA provenance through OIDC. No
long-lived signing key or release token belongs in the repository.

Before tagging, confirm a clean worktree, passing CI and security gates, a
reviewed changelog, and a successful generated-project smoke test. Never reuse
an already-published tag. Follow the repository’s
[release checklist](https://github.com/Alfian57/ginkit/blob/main/docs/releasing.md)
for verification commands.
