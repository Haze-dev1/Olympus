# Hermes Agent recipe

Sources checked 2026-10-19 and recorded in `docs/Sources/recipe-research.md`:

- Image, tags, ports, state layout, config keys, MCP shape, skills directory and
  Telegram variables: Hermes Agent documentation and source at
  <https://github.com/NousResearch/hermes-agent> (read-only clone in
  `references/hermes-agent`), plus Docker Hub `nousresearch/hermes-agent`.

## Checked facts

- Image: `nousresearch/hermes-agent`, mutable tags `latest`, `stable`, `main`
  and `X.Y.Z` releases. The recipe uses `latest`; the lock resolves and pins the
  digest (A7). **UNVERIFIED:** no concrete version tag was confirmed, so the
  recipe should move to a version tag when one is checked.
- State: `/opt/data` is the `HERMES_HOME` volume; code lives in `/opt/hermes`.
- Config: `/opt/data/config.yaml` (YAML), key `mcp_servers` for MCP servers.
  `hermes` rewrites this file, so it is listed under `writable`.
- MCP stdio shape: `command`, `args`, `env`; HTTP: `url`, `headers`.
  Secrets expand from the environment as `${VAR}`.
- Skills: one folder per skill with `SKILL.md` under `$HERMES_HOME/skills`.
  Hermes can write its primary skills directory, so the recipe copies skills
  into the volume instead of bind-mounting them (`mode: copy`).
- Telegram: `TELEGRAM_BOT_TOKEN` and `TELEGRAM_ALLOWED_USERS` (comma-separated).
  The gateway denies users outside the allow-list by default, so an empty
  allow-list is rejected during the build.
- Resources: 2-4 GB recommended; 4 GB is the conservative figure used here.
- Health: the API server on 8642 is off unless `API_SERVER_ENABLED=true`, so the
  health check uses `hermes --version` instead of HTTP. No port is published.

## UNVERIFIED

- That Hermes reads `ANTHROPIC_API_KEY` and `TELEGRAM_BOT_TOKEN` from the
  process environment, not only from `/opt/data/.env`. Compose delivers secrets
  as environment variables (A1); if Hermes ignores them, the activator must
  write them into `.env` at deploy time.
- That `hermes --version` exits 0 on the image; confirm during the VM test.
- Whether the image's default command runs the gateway without extra arguments.
