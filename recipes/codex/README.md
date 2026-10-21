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

- `CODEX_API_KEY` from the process environment for headless use (alternative:
  `CODEX_ACCESS_TOKEN` and `codex login --with-access-token`).

## Version pin

`@openai/codex@0.116.0` exists in the
[npm registry](https://registry.npmjs.org/@openai/codex/0.116.0), checked
2026-10-20. The recipe installs this exact release, including its exact-version
platform package. Builds are reproducible in their inputs (base digest, package version,
Dockerfile content), not byte-identical: a clean rebuild in the lab produced a
different image ID even with `SOURCE_DATE_EPOCH=0` and normalized install
timestamps. On a server, the content-addressed tag means the same generation
reuses the same image for activation and rollback.

## Lab observations, 2026-10-20

The real CLI lifecycle passed on the 6144 MiB Ubuntu 24.04 lab VM with Compose
v5.6.0. The Debian Node base contains `/bin/sh`, `mkdir` and `cp`; the init
service successfully copied `config.toml`. `sleep infinity` kept the container
running and `codex --version` returned `codex-cli 0.116.0` with sentinel provider
credentials. The configured TOML contains environment variable names only.
A no-op kept its container ID; an OpenCode-only change and rollback kept both
Codex container and image IDs. This is an idle CLI environment ready for exec,
not a running model session. No provider authentication or inference was attempted;
those require a valid API key or supported login. The process-environment API key
behavior remains unverified by a model request.
