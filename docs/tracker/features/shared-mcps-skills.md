# Shared MCP servers and skills

- **Status:** Not started
- **Milestone:** M3
- **Plan:** [plans/v1.md](../../plans/v1.md)

Define MCP servers and skills once and write them into each tool's config ([ADR 0012](../../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)).

## Acceptance

- One `mcps` entry appears in Hermes's YAML, Codex's TOML and OpenCode's JSON config.
- A tool without support produces a warning naming it.
