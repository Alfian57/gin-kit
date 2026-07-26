# AI Agents

`AGENTS.md` is the canonical repository instruction file. The repository also includes adapters for Claude, Gemini, GitHub Copilot, Cursor, and Aider.

The gin-kit skill is available at
`.github/skills/gin-kit-development/SKILL.md`. It describes the framework
runtime, both generated editions, safe migration workflow, and scaffold
validation.

When an agent changes a template:

1. Read the relevant `.tmpl` file and `.gin-kit.yaml` behavior.
2. Scaffold framework and starter API/UI projects.
3. Run `go test ./...` and `go build ./...` in the generated projects.
4. Preserve the canonical response envelope and field-level validation errors.
5. Update English documentation and tests.

## CI and release safety

AI agents may prepare a release checklist, but must not create or push a Git tag without explicit maintainer authorization for the exact version. The release version comes from an annotated `v0.x.y` tag, not from a hand-edited source constant.

Before suggesting a release, an agent must confirm a clean worktree and run the same checks used by CI. Maintainers can follow [docs/releasing.md](releasing.md) to publish and verify the release.
