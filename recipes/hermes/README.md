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

- Gateway operation with a real chat channel, including platform authentication,
  remains unverified. The lab intentionally configures no channel.

## Lab observations, 2026-10-20

The real CLI lifecycle passed on the 6144 MiB Ubuntu 24.04 lab VM with Compose
v5.6.0. The image contains `/bin/sh`, `mkdir` and `cp`; init copied both config
and the shared skill successfully. The image's default invocation starts the
interactive chat, so the recipe sets `command: [gateway, run]`, the documented
long-running mode (s6 supervises the gateway). With no chat channel configured
the container stays up and `hermes --version` passes its command health check. The YAML MCP entry retained `${GITHUB_TOKEN}`;
no sentinel value appeared in managed config or the store.

Hermes's own `hermes_cli.config._env_ref_lookup` resolved the injected process
environment without printing values. No secret dotenv provisioning is required
for this default profile. An image-created `/opt/data/.env` exists but contained
no sentinel. Profile-scoped secrets and live provider/chat authentication are
not exercised. With no channel the test proves container and CLI readiness;
it does not claim that a bot or a model session ran.
