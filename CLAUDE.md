Read `AGENTS.md` first — it is the canonical guidance for this repository,
including the repository map, engineering rules, and the documentation
discipline (every behavior change updates the affected docs in the same PR;
new features require a documentation-site page, a CHANGELOG entry, and
`docs/cli.md` when the CLI changes).

Before finishing any change, run:

```bash
go test ./... && go vet ./... && test -z "$(gofmt -l cmd framework internal)"
```

Template changes additionally require `.github/scripts/scaffold-smoke.sh`
(plus local `--example` scaffolds when `tasks_*` templates change — CI does
not cover them). Website changes require `cd website && npm run check && npm
run build`. Never create or push release tags without explicit maintainer
authorization.
