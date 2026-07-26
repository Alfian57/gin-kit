Read `AGENTS.md` before making changes — it describes this project's exact
edition, layout, commands, and rules.

The rules most often broken by agents:

- Bind request DTOs from `internal/dto` with `httpx.BindJSON[T]`; do not
  hand-roll input structs in handlers or re-validate in services.
- Wrap responses with the `New<Name>Response` mappers; never add
  credential-like fields to a response type.
- Pass `context.Context` through service and repository boundaries.
- Schema changes go in `migrations/` as versioned SQL — never AutoMigrate.
- Keep the OpenAPI description in sync when routes change.

In framework-edition projects, do not copy or edit gin-kit core; customize
it through public framework options and hooks. Run `gin-kit check` before
finishing.
