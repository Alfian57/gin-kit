# Contributing to gin-kit

Thank you for helping improve gin-kit. Contributions may target the runtime,
CLI, generated project types, documentation, or delivery pipeline.

Participation is covered by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

Requirements:

- Go 1.26 or newer;
- Git;
- Node.js 20+ when working on the UI template or documentation site;
- a C compiler when running SQLite integration tests.

Useful commands:

```bash
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd runtime internal)"
go run ./cmd/doccheck
go run ./cmd/gin-kit new /tmp/gin-kit-check --non-interactive --project-type standalone --module example.com/check --mode api --database sqlite --orm gorm
cd website && npm ci && npm run check && npm run build
```

The pull request workflow validates the runtime, documentation, and generated
Runtime and Standalone projects. Template changes must pass the complete
database/ORM matrix on `main`, including migration smoke tests for PostgreSQL,
MySQL, MariaDB, and SQLite.

## Source documentation

Write English doc comments for every package-level declaration, struct field,
and interface method in `cmd/`, `runtime/`, and `internal/`. Explain behavior,
defaults, ownership, concurrency, errors, or security boundaries when they are
relevant; do not merely repeat the identifier. Test entry points use their
names as their specification, while reusable helpers and fixtures need comments.
Run `go run ./cmd/doccheck` locally — CI and release checks enforce it.

## Pull requests

1. Explain the user-facing behavior and why it belongs in gin-kit.
2. Add or update tests.
3. Update English documentation and generated-project instructions when behavior changes.
4. Run the relevant generated-project matrix before requesting review.
5. Keep commits focused. A separate CLA is not required.

### Merging

Branch commits and pull request titles should use concise Conventional
Commit-style subjects. The merge commit itself must keep GitHub's default
message so first-parent history remains consistent:

```text
Merge pull request #<number> from <owner>/<branch>

<pull request title>
```

Maintainers and AI agents must wait for all required checks, then merge with
`gh pr merge <number> --merge`. Do not pass `--subject` or `--body`, use
squash/rebase merging, push directly to `main`, or bypass branch protection
unless a maintainer explicitly authorizes the exact bypass. After merging,
verify that the commit has two parents and do not rewrite published history
solely to change a merge message.

## Documentation discipline

Every behavior change updates the documents it affects, in the same pull
request — a feature is not done until its documentation exists. New features
require a page or section on the documentation site
(`website/src/content/docs/`, which deploys to GitHub Pages), a
`CHANGELOG.md` entry under `[Unreleased]`, `docs/cli.md` updates when the
CLI surface changes, and template AI-guidance updates
(`internal/cli/templates/AGENTS.md.tmpl` and friends) when the
generated-project workflow changes. The full change-type matrix lives in
[AGENTS.md](AGENTS.md).

Pull requests must pass the Go quality, generated-project smoke, release snapshot, vulnerability scan, dependency review, and CodeQL checks before merge.

## Template changes

The template tree is embedded into the CLI. Any template change must be checked
with Runtime and Standalone projects in API and UI modes. Changes to database or
ORM templates should be tested against every supported database and ORM
combination.

## New generators and databases

New generators must:

- use the `gin-kit generate ...` command tree;
- refuse accidental overwrites;
- produce formatted Go;
- include tests and an English explanation;
- preserve the router → handler → service → repository → database flow.

New database support must include connection setup, migration dialect, Docker documentation, health checks, and CI coverage.

## Releases

gin-kit uses SemVer tags in the `v0.x.y` series. Only maintainers may create annotated release tags. Read [docs/releasing.md](docs/releasing.md) for the complete release and verification procedure.
