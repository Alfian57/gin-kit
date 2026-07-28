---
title: Upgrade notes
description: Breaking changes and how to move past them.
---

gin-kit is pre-1.0: minor versions may break APIs, and each break is recorded
here and in the [changelog](https://github.com/Alfian57/gin-kit/blob/main/CHANGELOG.md).
Breaking changes apply when you upgrade the runtime module or regenerate code
with a newer CLI.

## How projects upgrade

**Runtime** projects import the versioned runtime, so upgrading is
a module bump:

```bash
go get github.com/Alfian57/gin-kit@vX.Y.Z && go mod tidy
```

**Standalone** projects vendor the runtime under `internal/platform/`
and upgrade with the CLI:

```bash
gin-kit upgrade           # report: what is outdated, modified, or missing
gin-kit upgrade --diff    # inspect the changes as unified diffs
gin-kit upgrade --apply   # write the safe updates (stale copies + missing files)
gin-kit upgrade --apply --force   # also overwrite locally modified files
```

The command re-renders the current CLI's platform templates for your
manifest and compares them with disk. A checksum baseline in `.gin-kit.sum`
(written at scaffold time, refreshed on every `--apply`) records what the
CLI last wrote, so the command can tell a stale vendored copy — updated
automatically — from your local edits, which are never overwritten without
`--force`. Files under `internal/platform/` that the current templates do
not render (for example a retired package) are reported as `unmanaged` and
never touched. Commit `.gin-kit.sum` together with the updated files.

Projects scaffolded **before** `.gin-kit.sum` existed have no baseline:
files that differ from the render are reported as `differs` because the
command cannot verify whether they were edited. Inspect them with `--diff`;
running `upgrade --apply` (with `--force` if you want the rendered versions)
bootstraps baseline entries for every file that matches the render, and the
next upgrade classifies precisely.

## Unreleased

### Breaking: Runtime and Standalone project types

The CLI now creates only manifest v3 projects with
`project_type: runtime|standalone`. Runtime applications import
`github.com/Alfian57/gin-kit/runtime/...`. Earlier manifest formats, CLI
selectors, and runtime import paths are not supported by this release.

### Services take DTOs instead of input structs

`generate resource` services now accept `dto.Create<Name>Request` /
`dto.Update<Name>Request` and the service-local `<Name>Input` struct,
`validate()` method, and `ErrInvalid<Name>` sentinel are gone. Validation
happens once, at bind time, answering `422` with field details.

Existing generated code keeps compiling — this affects newly generated
resources. To adopt the layout in an existing project, run
`gin-kit generate dto <Name> --fields ...` and migrate the service signature
to accept the request type.

### Standalone: `internal/platform/response` → `internal/platform/httpx`

Regenerated standalone projects vendor the runtime envelope and binders as
`internal/platform/httpx` (plus `internal/platform/validation`); the old
`internal/platform/response` package is no longer scaffolded. Error responses
gain `details` and `request_id` fields.

For an existing standalone project, either keep the old package (it still works)
or copy the new `platform/httpx` from a fresh scaffold and update imports:
`response.Error(c, status, code, msg)` becomes
`httpx.Fail(c, httpx.NewError(status, code, msg))`.

### `apptest.Do` takes request options

The test helper signature changed from an `http.Header` parameter to
variadic options:

```go
// before
apptest.Do(t, app, http.MethodGet, "/me", nil, http.Header{"Authorization": {"Bearer " + token}})

// after
apptest.Do(t, app, http.MethodGet, "/me", nil, apptest.WithBearer(token))
```

`WithHeader`, `WithBearer`, and `WithCookie` cover the common cases; see
[Testing](/gin-kit/testing/).
