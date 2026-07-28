Read `AGENTS.md` before making changes — it describes this project's exact
project type, layout, commands, and rules.

The rules most often broken by agents:

- Bind request DTOs from `internal/dto` with `httpx.BindJSON[T]`; do not
  hand-roll input structs in handlers or re-validate in services.
- Wrap responses with the `New<Name>Response` mappers; never add
  credential-like fields to a response type.
- Pass `context.Context` through service and repository boundaries.
- Schema changes go in `migrations/` as versioned SQL — never AutoMigrate.
- Keep the OpenAPI description in sync when routes change.
- If OAuth is enabled, preserve verified-email linking, PKCE/state/nonce
  validation, and the rule that provider tokens never enter logs, models, or responses.
- For Runtime WhatsApp delivery, use approved templates only and never log
  recipients, codes, template parameters, or Cloud API tokens.

In Runtime projects, do not copy or edit gin-kit core; customize it through
public runtime options and hooks. Run `gin-kit check` before
finishing.
