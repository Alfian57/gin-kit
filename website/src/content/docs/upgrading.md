---
title: Upgrade notes
description: Breaking changes and how to move past them.
---

gin-kit is pre-1.0: minor versions may break APIs, and each break is recorded
here and in the [changelog](https://github.com/Alfian57/gin-kit/blob/main/CHANGELOG.md).
Already-generated projects always keep working — breaking changes apply when
you upgrade the framework module or regenerate code with a newer CLI.

## Unreleased

### Services take DTOs instead of input structs

`generate resource` services now accept `dto.Create<Name>Request` /
`dto.Update<Name>Request` and the service-local `<Name>Input` struct,
`validate()` method, and `ErrInvalid<Name>` sentinel are gone. Validation
happens once, at bind time, answering `422` with field details.

Existing generated code keeps compiling — this affects newly generated
resources. To adopt the layout in an existing project, run
`gin-kit generate dto <Name> --fields ...` and migrate the service signature
to accept the request type.

### Starter: `internal/platform/response` → `internal/platform/httpx`

Regenerated starter projects vendor the framework envelope and binders as
`internal/platform/httpx` (plus `internal/platform/validation`); the old
`internal/platform/response` package is no longer scaffolded. Error responses
gain `details` and `request_id` fields.

For an existing starter project, either keep the old package (it still works)
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

## Older migrations

- **Manifest v1 → v2** (`.gin-kit.yaml`): see
  [Manifest v1 migration](/gin-kit/migration-v1/).
- **Module rename** `ginkit` → `gin-kit`: update the module path in `go.mod`
  and imports; the binary, config file, and env prefix follow the new name.
