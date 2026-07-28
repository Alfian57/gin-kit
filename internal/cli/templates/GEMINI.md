Read `AGENTS.md` before making changes — it describes this project's exact
project type, layout, commands, and rules.

Keep the router, handler, service, repository, and database boundaries
explicit: handlers bind request DTOs from `internal/dto` and wrap responses
with the `New<Name>Response` mappers; services take DTOs and return domain
values; `context.Context` flows through every boundary. Schema changes are
versioned SQL in `migrations/`. Keep the OpenAPI description in sync when
routes change.

When OAuth is enabled, preserve verified-email identity linking and keep
provider tokens out of logs, models, and responses.

For Runtime WhatsApp delivery, use approved templates only and never log
recipients, codes, template parameters, or Cloud API tokens.

In Runtime projects, do not copy or edit gin-kit core; customize it through
public runtime options and hooks. Run `gin-kit check` before
finishing.
