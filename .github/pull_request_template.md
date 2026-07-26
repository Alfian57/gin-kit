## Summary

Describe the user-facing change and why it belongs in gin-kit.

## Validation

- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] Generated API/UI projects were checked when templates changed
      (including `--example` scaffolds when `tasks_*` templates changed).

## Documentation

- [ ] Documentation site updated (`website/src/content/docs/` — required for
      new features).
- [ ] `CHANGELOG.md` entry added under `[Unreleased]`.
- [ ] `docs/cli.md` updated when the CLI surface changed.
- [ ] Template AI guidance (`internal/cli/templates/AGENTS.md.tmpl` and
      friends) updated when the generated-project workflow changed.

## AI-agent changes

If an AI agent contributed, explain which files or workflows it changed and how
the result was reviewed.
