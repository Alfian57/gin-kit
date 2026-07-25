# GinKit Development Skill

## Workflow

Read `AGENTS.md` before changing generated code. Keep the explicit
router-to-database flow and pass `context.Context` through every boundary.

## Safety defaults

Keep request IDs, security headers, trusted proxy configuration, body limits,
readiness checks, and rate-limit middleware enabled. Never trust forwarded
client IP headers without configuring trusted proxy CIDRs.

## Validation

Run `ginkit check`, exercise the selected database migration, and test the
affected handler or service. Do not put secrets in source files or migrations.
