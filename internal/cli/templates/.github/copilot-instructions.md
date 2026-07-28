Use `AGENTS.md` as the canonical project guidance. Prefer idiomatic Go,
explicit constructors, and tests for every behavior change.

Key conventions: handlers bind request DTOs from `internal/dto` with
`httpx.BindJSON[T]` and wrap responses with `New<Name>Response` mappers;
services accept DTOs and return domain values; `context.Context` flows
through every boundary; schema changes are versioned SQL in `migrations/`
(never AutoMigrate); credential-like fields never appear in response types;
keep the OpenAPI description in sync when routes change. Run `gin-kit check`
before finishing.

For an OAuth-enabled project, preserve verified-email linking and never log,
persist, or serialize provider tokens.
