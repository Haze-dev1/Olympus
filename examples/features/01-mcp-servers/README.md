# Feature: Shared MCP Servers (stdio and HTTP)

This example shows how to define Model Context Protocol (MCP) servers once and share them across different agents.

Olympus supports two kinds of MCP servers:
1. **stdio MCP servers:** Local processes spawned inside the container (configured with `command`, `args`, and `env`).
2. **HTTP/remote MCP servers:** Remote endpoints reachable over the network (configured with `url`, `transport`, and `headers`).

Here, Hermes and Codex both share a local GitHub stdio MCP server and a remote documentation HTTP MCP server.

## Per-Tool Translation

Different agent harnesses and coding tools use completely different configuration formats, file paths, and syntax for MCP servers. Olympus translates your single `mcps:` definition into each tool's native configuration format during evaluation:

- **Hermes Agent (`recipes/hermes`):**
  - Config file: `/opt/data/config.yaml` (YAML format, key `mcp_servers`).
  - Stdio MCP: Writes `command`, `args`, and `env` with `${VAR}` references.
  - Remote HTTP MCP: Writes `url` and `headers`. For `transport: sse`, it adds `transport: sse`.
- **Codex (`recipes/codex`):**
  - Config file: `/root/.codex/config.toml` (TOML format, key `mcp_servers`).
  - Stdio MCP: Writes `command = "..."`, `args = [...]`, `env_vars = ["VAR"]`.
  - Remote HTTP MCP: Writes `url = "..."`. If a single `Authorization: Bearer ${VAR}` header is passed, Codex translates it to `bearer_token_env_var = "VAR"`. Codex only supports Streamable HTTP; `transport: sse` is rejected.
- **Claude Code (`recipes/claude-code`):**
  - Config file: `/root/.mcp.json` (JSON format, key `mcpServers`).
  - Stdio MCP: Writes `command`, `args`, and `env` with `${VAR}` references.
  - Remote HTTP MCP: Writes `url`, `headers`, and required field `type: "http"` (or `type: "sse"`).
- **OpenCode (`recipes/opencode`):**
  - Config file: `/root/.config/opencode/opencode.json` (JSON format, key `mcp`).
  - Stdio MCP: Writes `type: "local"`, `command: [...]`, and `environment: { KEY: "{env:VAR}" }`.
  - Remote HTTP MCP: Writes `type: "remote"`, `url`, and `headers: { KEY: "{env:VAR}" }`. OpenCode automatically negotiates Streamable HTTP and SSE.
- **Gemini CLI (`recipes/gemini-cli`):**
  - Config file: `/root/.gemini/settings.json` (JSON format, key `mcpServers`).
  - Remote HTTP MCP: Uses `httpUrl` for Streamable HTTP and `url` for SSE.

Secret values are never written to disk in plain text. They are passed as variable references and injected at runtime by Olympus (Rule A1, Rule A5).

## What You Need

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Set the required environment variables:
- `ANTHROPIC_API_KEY`: API key for Hermes.
- `CODEX_API_KEY`: API key for Codex.
- `GH_TOKEN`: GitHub personal access token used by both the stdio MCP and the remote MCP.

## Commands

Validate the stack offline:

```sh
olym check -f stack.yaml
```

Build the local generation to inspect the rendered configs:

```sh
olym build -f stack.yaml -o /tmp/build-01
```

Inspect the rendered configs in `/tmp/build-01/store/`:
- Notice Hermes config has YAML `mcp_servers`.
- Notice Codex config has TOML `[mcp_servers]`.

Preview server changes:

```sh
olym plan -f stack.yaml
```

Deploy to your server:

```sh
olym apply -f stack.yaml
```

Check runtime status:

```sh
olym status -f stack.yaml
```

## What You Should See

- `olym check` confirms:
  ```
  Stack is valid.
  ```
- `olym plan` reports:
  - Addition of `hermes` container and its rendered `/opt/data/config.yaml`.
  - Addition of `codex` container and its rendered `/root/.codex/config.toml`.
  - No secrets printed or leaked.
- `olym status` shows:
  - Both containers running and healthy.
  - MCP configurations active in their respective native formats.
