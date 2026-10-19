# Shared MCP servers and skills

- **Status:** Not started
- **Milestone:** M3
- **Plan:** [plans/v1.md](../../plans/v1.md)

Define MCP servers and skills once and write them into each tool's config ([ADR 0012](../../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)).

## Acceptance

- One `mcps` entry appears in Hermes's YAML, Codex's TOML and OpenCode's JSON config.
- A tool without support produces a warning naming it.

## Evidence

- 2026-10-19 — MCP config facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): Hermes `mcp_servers` in `~/.hermes/config.yaml` (YAML, `${VAR}`/`${env:VAR}`); Codex `[mcp_servers.*]` in `~/.codex/config.toml` (TOML, secrets only by env-var name through `env_vars`, `bearer_token_env_var`, `env_http_headers`); OpenCode `mcp` in `~/.config/opencode/opencode.json` (JSON/JSONC, `{env:VAR}` and `{file:…}`).
- Secret delivery is an environment variable for all three, so the core can use one mechanism per tool with no wrapper. Codex needs the variable name rather than an inline value, which the renderer has to encode.
- Not done: no renderer or recipe written; the shared-resources decision (copy vs shared read-only mount) is still open (TRD 16).
