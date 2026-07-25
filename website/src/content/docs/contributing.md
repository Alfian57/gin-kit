---
title: Contributing
description: Help improve GinKit.
---

Read the repository’s `CONTRIBUTING.md` and `AGENTS.md`, then create a branch
for a focused change. Keep commits small and in English. Template changes must
validate both API and UI scaffolds.

Before opening a pull request:

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal framework)"
.github/scripts/scaffold-smoke.sh
```

Explain the user-facing behavior, tests, compatibility impact, and migration
notes in the pull request. Security issues belong in `SECURITY.md`, not public
issues.
