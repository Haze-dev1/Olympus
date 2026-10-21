# Shared MCP servers and skills

- **Status:** Done with rendering and real deployment verified in [first-recipes](first-recipes.md).
- **Milestone:** M3
- **Plan:** [plans/v1.md](../../plans/v1.md)

Define MCP servers and skills once and write them into each tool's config ([ADR 0012](../../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)).

## Acceptance

- One `mcps` entry appears in Hermes's YAML, Codex's TOML and OpenCode's JSON config.
- A tool without support produces a warning naming it.

## Evidence

- 2026-10-19 — MCP config facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): Hermes `mcp_servers` in `~/.hermes/config.yaml` (YAML, `${VAR}`/`${env:VAR}`); Codex `[mcp_servers.*]` in `~/.codex/config.toml` (TOML, secrets only by env-var name through `env_vars`, `bearer_token_env_var`, `env_http_headers`); OpenCode `mcp` in `~/.config/opencode/opencode.json` (JSON/JSONC, `{env:VAR}` and `{file:…}`).
- 2026-10-20 — `eval.translateMCP` renders the same stack entry into all three shapes, selected by `supports.mcps.style`. `testdata/golden-real` shows `github` with `env: GITHUB_TOKEN: ${GITHUB_TOKEN}` (Hermes), `env_vars = ['GITHUB_TOKEN']` with no interpolation (Codex) and `environment: {GITHUB_TOKEN: '{env:GITHUB_TOKEN}'}` (OpenCode). A recipe without `supports.mcps` still yields the warning `recipe does not support MCP servers; skipped`.
- 2026-10-20 — Skills are rendered once and delivered by `supports.skills.mode`: Hermes copies into `/opt/data/skills` via the init service because it writes that directory, Codex and OpenCode bind the same source read-only at `/root/.agents/skills` and `/root/.config/opencode/skills`. The golden build checks the copied skill reaches the Hermes init script and the mounted skill stays a read-only bind.
- Decisions: [ADR 0037](../../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references) and [0038](../../ADR.md#0038-mount-shared-skills-read-only-where-supported-and-copy-them-where-the-tool-writes).
- Limit: copied skills are not removed from the volume when deleted from the stack file.

- 2026-10-20: real three-tool `TestLabRealStack` passed in 70.501 s. Writable
  config init, shared skill copy/mount, native MCP references, selective MCP-list
  recreation, rollback and delete-data teardown were exercised. Full evidence,
  prior failures and authentication limits are in [first-recipes](first-recipes.md).
  ADRs 0040–0041 cover this run's changed catalog/build behavior; no additional
  format decision was needed for native MCP or skill delivery.
