Read `AGENTS.md` before making changes — it describes this project's exact
edition, layout, commands, and rules.

Keep the router, handler, service, repository, and database boundaries
explicit: handlers bind request DTOs from `internal/dto` and wrap responses
with the `New<Name>Response` mappers; services take DTOs and return domain
values; `context.Context` flows through every boundary. Schema changes are
versioned SQL in `migrations/`. Keep the OpenAPI description in sync when
routes change.

In framework-edition projects, do not copy or edit gin-kit core; customize
it through public framework options and hooks. Run `gin-kit check` before
finishing.
