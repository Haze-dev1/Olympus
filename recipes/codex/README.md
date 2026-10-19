# Codex recipe

Sources checked 2026-10-19 and recorded in `docs/Sources/recipe-research.md`:

- Install methods, `CODEX_HOME`, `config.toml`, MCP tables, skill directories,
  auth and resources: OpenAI Codex documentation and
  <https://github.com/openai/codex>.

## Checked facts

- **No official container image is documented.** The recipe builds one with a
  recipe-supplied Dockerfile (`build`); Compose builds it on the server. The
  base image (`node:22-bookworm-slim`) is a normal registry image, so the lock
  resolves and pins it by digest; the Dockerfile is content-addressed in the
  generation and included in the tool hash.
- `CODEX_HOME=/root/.codex` (the directory is created by the init copy).
  Config is `/root/.codex/config.toml` (TOML); Codex rewrites it, so it is
  `writable`.
- MCP: `[mcp_servers.<name>]` tables. stdio uses `command`, `args` and
  `env_vars` (a list of environment variable *names*); HTTP uses `url` plus
  `bearer_token_env_var` or `env_http_headers`. Codex has no value
  interpolation, so the generation never writes a secret value.
- Skills: `$HOME/.agents/skills/<name>/SKILL.md`, discovered by walking up.
  Mounted read-only (`mode: mount`).
- Resources: 4 GB minimum, 8 GB recommended; the recipe uses 4 GB.
- Health: `codex --version`; there is no supported HTTP endpoint.
- No chat channels for the local CLI (Slack/Teams are ChatGPT Cloud).

## UNVERIFIED

- `sleep infinity` as the container command. Codex is a CLI/app-server, not a
  daemon; confirm the intended long-running invocation during the VM test.
- The npm package version is `latest` because no version was confirmed. Pin
  `CODEX_VERSION` in the Dockerfile.
- `node:22-bookworm-slim` as the base; Codex documents Linux support but not a
  container base image.
- `CODEX_API_KEY` from the process environment for headless use (alternative:
  `CODEX_ACCESS_TOKEN` and `codex login --with-access-token`).
