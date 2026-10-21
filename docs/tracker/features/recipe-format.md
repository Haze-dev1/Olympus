# Recipe format

- **Status:** Done with three-tool lab deployment evidence in [first-recipes](first-recipes.md).
- **Milestone:** M2
- **Plan:** [plans/v1.md](../../plans/v1.md)

Load declarative recipes and render their templates ([spec](../../specs/recipe.md), [ADR 0005](../../ADR.md#0005-describe-each-tool-with-a-declarative-yaml-recipe)).

## Acceptance

- Adding a recipe needs no core change.
- Invalid recipes fail with file, line and field.

## Evidence

- 2026-10-20 — `internal/recipe` loads and validates the new optional fields `command`, `env`, `writable` and `build`, the `supports.mcps.style` and `supports.skills.mode` enums, and `supports.channels.<name>.env`. Paths go through the existing volume confinement checks; `ShellSafe` guards paths that reach the init script; `ReadFile` rejects symlinks and secret-looking files.
- 2026-10-20 — `schemas/recipe.schema.json` describes the same fields and is the schema `check` uses. A recipe with an unknown key or a wrong enum fails with file, line and column; `internal/recipe/recipe_test.go` covers the rejected cases.
- 2026-10-20 — The shipped `recipes/hermes`, `recipes/codex` and `recipes/opencode` add no core code; each folder's README records its sources and unverified items.
- Decisions: [ADR 0035](../../ADR.md#0035-build-tools-without-an-official-image-from-a-recipe-supplied-dockerfile) (build block), [0036](../../ADR.md#0036-deliver-writable-managed-config-with-a-compose-init-service) (writable), [0037](../../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references) (`style`), [0038](../../ADR.md#0038-mount-shared-skills-read-only-where-supported-and-copy-them-where-the-tool-writes) (`mode`), [0039](../../ADR.md#0039-configure-hermes-channels-as-environment-variables) (`channels.env`).

- 2026-10-20: real three-tool `TestLabRealStack` passed in 70.501 s. Writable
  config init, shared skill copy/mount, native MCP references, selective MCP-list
  recreation, rollback and delete-data teardown were exercised. Full evidence,
  prior failures and authentication limits are in [first-recipes](first-recipes.md).
  ADRs 0040–0041 cover this run's changed catalog/build behavior; no additional
  format decision was needed for native MCP or skill delivery.
