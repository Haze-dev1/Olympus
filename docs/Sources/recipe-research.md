# Recipe research: Hermes Agent, Codex and OpenCode

- **As of:** 2026-10-19
- **Method:** each tool's official documentation, repository and release notes, read on 2026-10-19. Hermes Agent was read from the local clone at `references/hermes-agent` (HEAD `6e9b3e3b8852f12bf965f888213470574b72803e`), whose `website/docs/` is the source of the published docs; every other source is a link below. Nothing was installed or executed. Anything not confirmed by a primary source is marked **UNVERIFIED** rather than guessed.

This file answers the same question list for each tool so the recipe files can be written from it: image and tags, ports and health, data and config, MCP servers, skills, chat channels, authentication, runtime config writes, resources and licence. The last section maps the findings onto the open questions in [TRD section 16](../TRD.md#16-open-technical-questions).

The three tools split along the same axis as the recipe spec ([`docs/specs/recipe.md`](../specs/recipe.md)):

- **Hermes Agent** is a harness: official image, config as data, MCP in YAML, skills, chat channels.
- **Codex** is a coding agent with **no official image**, config as data, MCP in TOML, no chats.
- **OpenCode** is a coding agent with an official image, config as data, MCP in JSON, skills, no chats.

---

## Hermes Agent

Nous Research's agent harness. Official docs live in the repository under `website/docs/` and are published at `hermes-agent.nousresearch.com/docs`; file paths below are relative to `references/hermes-agent`.

### Official container image and tags

- Image: **`nousresearch/hermes-agent`** (Docker Hub). The Dockerfile carries the image-provenance marker for that name (`Dockerfile`, line ~417).
- Tags (`website/docs/user-guide/docker.md`, lines 16–26): `latest` and `stable` (both resolve to the stable release gate), `main` (development builds from the `main` branch) and version tags `X.Y.Z`. The build workflow tests both `amd64` and `arm64`. The docs recommend pinning a digest for an exact release.
- Base is `debian:13.4` pinned by digest, with s6-overlay `3.2.3.0` as PID 1. Code is immutable under `/opt/hermes`; all mutable state lives under `/opt/data` (a declared `VOLUME`, `ENV HERMES_HOME=/opt/data`). Container user is non-root `hermes`, UID 10000, remappable with `HERMES_UID`/`HERMES_GID`.
- The official `docker-compose.yml` uses `network_mode: host` and runs `gateway run` with `restart: unless-stopped`. Olympus will not copy that: it runs one service per tool on a private network and publishes ports itself (TRD 7.3).
- Licence: **MIT** (`LICENSE`, Copyright (c) 2025 Nous Research).

### Ports and health

| Port | What | Notes |
|---|---|---|
| 8642 | OpenAI-compatible API server | Default; `gateway/platforms/api_server.py` `DEFAULT_PORT = 8642`, config key `gateway.api_server.port` |
| 9119 | Dashboard | `HERMES_DASHBOARD_PORT` default 9119; the docs bind it to `127.0.0.1` |
| 8644 | Webhook receiver | `hermes_cli/gateway_migrate.py` |
| 8646 | Microsoft Graph webhook | same |

- **Health endpoint:** `GET /health` on the API server returns `200` with `{"status":"ok","platform":"hermes-agent","version":...}` and is **not authenticated** (`gateway/platforms/api_server.py`, `_handle_health` has no `_require_auth`). `/health/detailed` and `/v1/health` require a bearer token.
- The API server is **off unless `API_SERVER_ENABLED=true`**, and it binds loopback unless `API_SERVER_HOST=0.0.0.0` with `API_SERVER_KEY` (at least 8 chars). So port 8642 and the `/health` path are only usable as the recipe health check if the recipe enables the API server. Otherwise use `health.command` (TRD 8.5 allows a `docker exec` command).
- Chat-only deployments need no published port at all (`docker.md`: exposing 8642 is "optional if you only use chat platforms").

### Data directory, config file and format

- One data directory: **`/opt/data`** in the container, `~/.hermes` on a bare host (`HERMES_HOME`).
- **`$HERMES_HOME/config.yaml`** — all non-secret configuration, **YAML**. Documented in `website/docs/user-guide/configuration.md`.
- **`$HERMES_HOME/.env`** — secrets, as `UPPER_SNAKE` environment variables. `hermes config set` routes `UPPER_SNAKE` names to `.env` and dotted settings to `config.yaml` (`configuration.md`, line ~54). There is a documented precedence order below that.
- Other contents of the data dir (`docker.md` ~200–215, `configuration.md` 19–32): `auth.json` (OAuth provider credentials), `SOUL.md`, `sessions/`, `memories/` (`MEMORY.md`, `USER.md`), `skills/`, `home/`, `cron/`, `hooks/`, `logs/`, `skins/`, `state.db`, `mcp-tokens/<server>.json` (mode 0600), `profiles/<name>/`.

### MCP servers

- **File** `$HERMES_HOME/config.yaml`, **top-level key `mcp_servers`**, **YAML**. `website/docs/user-guide/mcp.md` (~line 484). Gateway watches the file and reloads within about a minute; `/reload-mcp` forces it.

Minimal stdio server:

```yaml
mcp_servers:
  filesystem:
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/work"]
```

Keys: `command`, `args`, `env`, `cwd`.

Minimal HTTP server:

```yaml
mcp_servers:
  company_api:
    url: "https://mcp.internal.example.com"
    headers:
      Authorization: "Bearer ${COMPANY_MCP_TOKEN}"
```

Alternatively `transport: sse`, or `auth: oauth` for OAuth 2.1 with PKCE (tokens stored at `$HERMES_HOME/mcp-tokens/<server>.json`, mode 0600).

- **Environment-variable references: yes.** Any string value in a server entry (`env`, `headers`, `args`, `url`, …) may use `${VAR}` or `${env:VAR}`. Values resolve from the active profile's secret scope, then the process environment; the documented place to put the secret is `$HERMES_HOME/.env`. Behaviour when unset: the literal placeholder is kept, **except** that a stdio `env` entry that is exactly one unset reference is dropped, and an unset `${VAR}` in a remote `url` or `headers` fails closed with a 401-style error naming the variable. Context variables `${userHome}`, `${workspaceFolder}`, `${workspaceFolderBasename}` and `${pathSeparator}` also exist. Source: `website/docs/user-guide/mcp.md`, "Environment variable expansion".
- Security details worth carrying into the recipe: stdio servers receive only explicitly configured env plus a safe baseline (not the whole shell env); per-server `enabled`, `trust` (`full`/`untrusted`), tool include/exclude globs; tools are named `mcp__<server>__<tool>`.

### Skills

- Primary directory **`$HERMES_HOME/skills/`** (i.e. `/opt/data/skills`), one folder per skill containing **`SKILL.md`**, following the agentskills.io specification (`website/docs/user-guide/skills.md`). Bundled skills are copied there on a fresh install.
- Extra directories via `config.yaml` → `skills.external_dirs`, a list whose entries may use `~` and `${VAR}` (for example `~/.agents/skills`, `/home/shared/team-skills`, `${SKILLS_REPO}/skills`). Precedence: project → local → `skills.create_dir` → `external_dirs`. Non-existent paths are skipped silently.
- Caveat: external directories are not a write-protection boundary. If they are writable the agent can change them, and the primary `skills/` directory is explicitly agent-writable.

### Chat channels

- The gateway supports Telegram, Discord, Slack, WhatsApp (Baileys and Cloud API), Signal, SMS, Email, Home Assistant, Mattermost, Matrix, DingTalk, Feishu/Lark, WeCom, Weixin, BlueBubbles/iMessage, QQ, Yuanbao, Microsoft Teams, LINE, ntfy, webhooks and the API server. Index: `website/docs/user-guide/messaging/index.md`.
- Credentials and allow-lists are **environment variables in `.env`**, not keys in `config.yaml`:
  - Telegram (`messaging/telegram.md` ~lines 286–287, 1467): `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_USERS` (comma-separated IDs), plus `TELEGRAM_GROUP_ALLOWED_USERS`, `TELEGRAM_REQUIRE_MENTION`.
  - Discord (`messaging/discord.md` ~279–308): `DISCORD_BOT_TOKEN`, `DISCORD_ALLOWED_USERS`, `DISCORD_ALLOWED_ROLES`, `DISCORD_REQUIRE_MENTION`, `DISCORD_FREE_RESPONSE_CHANNELS`. Server-member usernames need the Server Members Intent.
- The gateway is **deny-by-default**: users who are not allow-listed (and not DM-paired) are refused (`messaging/index.md` ~line 352). Discord denies everyone unless `DISCORD_ALLOWED_USERS`/`DISCORD_ALLOWED_ROLES` is set, or one of the explicit allow-all flags is turned on. `config.yaml` has a `platforms:` section for enable/disable and non-secret settings, but the tokens and allow-lists stay in `.env`.
- Consequence for the recipe spec: Hermes' `supports.channels` is **env-based** (a token plus an allow-list), not a `config.yaml` key, so the current `channels` shape (file + key) does not describe it. See the TRD-16 answers below.

### Authentication

- **API keys:** put the provider key in `$HERMES_HOME/.env` (for example `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`); `hermes auth add <provider> --api-key …`; `hermes setup` writes `.env`; `hermes model` is an interactive wizard. `hermes auth` also lists/removes/resets/logs out. `hermes secrets` can pull keys from Bitwarden Secrets Manager at startup instead of `.env`.
- **Subscriptions:**
  - **Nous Portal** is the recommended subscription (`hermes setup --portal`). OAuth puts a refresh token in `$HERMES_HOME/auth.json`; a short-lived JWT is minted per inference call with automatic refresh and keepalive (`nous.keepalive_interval_seconds`, maximum 900). An invalid refresh token is quarantined; `hermes auth add nous` re-logs in. The OAuth callback is a loopback listener on the Hermes host; for a remote/headless host the docs forward it over SSH (`website/docs/guides/oauth-over-ssh.md`).
  - **OpenAI Codex / ChatGPT subscription:** supported — `hermes auth add openai-codex --type oauth`. The default flow is a **device code** (headless-friendly); `--browser` uses auth-code + PKCE on `localhost:1455`.
  - **Anthropic (Claude) OAuth:** `hermes auth add anthropic --type oauth` is supported, which implies a Claude subscription login is possible.
  - **UNVERIFIED:** token lifetime and refresh interval for the Codex and Anthropic OAuth flows; the docs describe automatic refresh but give no durations.
- Chat credentials (bot tokens) are separate from model authentication.

### Does it rewrite its own config at runtime?

**Yes.** `hermes setup`, `hermes model` (writes `model.provider`, `model.default`, `agent.reasoning_effort`), `hermes config set/unset`, `hermes mcp install/configure` (writes `mcp_servers` and tokens) and the dashboard/desktop QR setup (writes `TELEGRAM_BOT_TOKEN` and `TELEGRAM_ALLOWED_USERS` into the profile `.env`, then restarts the gateway) all write `config.yaml` and/or `.env`. This is an inference from the commands' documented effects, not a single upstream statement. **A read-only bind mount of `config.yaml`/`.env` will break those commands**, so the recipe needs the TRD 7.3 `writable: true` copy-into-volume path.

### Resources and licence

- `docker.md` ~507–511: **1 GB** RAM is enough without browser tools; **2–4 GB recommended**; at least **2 GB** if browser automation (Playwright/Chromium) is used. No CPU or disk figure is given.
- The prototype recipe and TRD 5.2 use `min_ram: 4GB`, which is a conservative superset of the recommended range. The exact upstream recommendation for a specific channel set is **UNVERIFIED** (it depends on the enabled tools).
- Licence **MIT**.

### Other useful facts

- The image is stateless; upgrading means pulling a new image. `hermes update` refuses on an image-managed install (exit 2) and prints the `docker pull` command (`website/docs/user-guide/updating.md` ~line 572).
- `gateway run` is supervised by s6 inside the official image; `HERMES_GATEWAY_NO_SUPERVISE=1` opts out.
- `hermes mcp add codex --preset codex` exists (writes a stdio server running `codex mcp-server`), but Olympus configures MCPs itself.

---

## Codex

OpenAI's coding agent. Sources: [github.com/openai/codex](https://github.com/openai/codex) and the docs under `developers.openai.com/codex/…` (the older `/codex/config/` paths now redirect to `/codex/config-file/*`).

### Official container image and tags

- **No official container image is documented.** The repository root has no `Dockerfile`, and neither the README nor the install docs mention running the CLI as a container. (`codex-universal` is OpenAI's hosted cloud environment, not a documentedly self-hostable image.)
- Documented installs (`docs/install.md`): the standalone installer `curl -fsSL https://chatgpt.com/codex/install.sh | sh` (binaries from `releases.openai.com`, falling back to GitHub Releases; `CODEX_INSTALLER_USE_RELEASES_OPENAI_COM=false` forces GitHub), `npm install -g @openai/codex`, `brew install --cask codex`, or the GitHub Release binaries (`codex-x86_64-unknown-linux-musl.tar.gz`, `codex-aarch64-unknown-linux-musl.tar.gz`).
- **Consequence:** the Codex recipe cannot pin an upstream image. It has to build one (for example a small base image with Node or the musl binary installed), which the recipe spec's required `image` field does not currently express. See the TRD-16 answers.
- Licence: **Apache-2.0** (README and `LICENSE`).

### Ports and health

- The Codex CLI listens on nothing by default.
- `codex app-server` (the supported embedding interface) speaks JSON-RPC over **stdio** by default, a unix socket, or an **experimental** WebSocket: `codex app-server --listen ws://127.0.0.1:4500`. In WebSocket mode it serves:
  - `GET /readyz` → `200` once the listener accepts connections.
  - `GET /healthz` → `200` when the request has no `Origin` header; requests with an `Origin` header get `403`.
- The WebSocket transport is explicitly "experimental" and "not supported for production workloads". Non-loopback listeners currently accept unauthenticated connections during rollout; `--ws-auth capability-token --ws-token-file …` or `--ws-auth signed-bearer-token` is available. `codex --remote ws://…` attaches the TUI.
- **Consequence:** a Codex recipe has no stable HTTP health endpoint. Its health check should be a command (for example `codex --version` or `codex login status`), or the recipe can run `codex app-server` on stdio with no port at all (TRD 8.5 `health.command`).

### Data directory, config file and format

- `CODEX_HOME` (default **`~/.codex`**) is the root for "config, auth, logs, sessions, skills, and standalone package metadata"; if set, the directory must already exist.
- **Config file `$CODEX_HOME/config.toml`** (default `~/.codex/config.toml`), **TOML**. Precedence: CLI flags → project `.codex/config.toml` (trusted projects only) → profile `~/.codex/<profile>.config.toml` → user `~/.codex/config.toml` → cloud-managed → `/etc/codex/config.toml` → built-in.
- Credentials at `~/.codex/auth.json` (plaintext) or the OS keyring, selected by `cli_auth_credentials_store = file | keyring | auto | ephemeral`. Sessions are JSONL rollout files; SQLite state via `CODEX_SQLITE_HOME`.
- Sources: `developers.openai.com/codex/config-file/…`.

### MCP servers

- **File** `$CODEX_HOME/config.toml`, **TOML tables `[mcp_servers.<name>]`**. (`developers.openai.com/codex/extend/mcp`.)

Minimal stdio server:

```toml
[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]
env_vars = ["LOCAL_TOKEN"]

[mcp_servers.context7.env]
MY_ENV_VAR = "MY_ENV_VALUE"
```

Minimal HTTP server:

```toml
[mcp_servers.figma]
url = "https://mcp.figma.com/mcp"
bearer_token_env_var = "FIGMA_OAUTH_TOKEN"
http_headers = { "X-Figma-Region" = "us-east-1" }
```

- stdio keys: `command` (required), `args`, `env` (a table of **literal** values), `env_vars` (names to forward from Codex's own environment; an entry is either `"NAME"` or `{ name = "NAME", source = "remote" }`), `cwd`, `experimental_environment`.
- HTTP keys: `url` (required), `auth` (`oauth` by default, or `chatgpt`), `bearer_token_env_var` (the **name** of an env var holding the token), `http_headers` (static values), `env_http_headers` (header name → env var name), `http_headers_helper`.
- Shared keys: `startup_timeout_sec` (default 10), `tool_timeout_sec` (default 60), `enabled`, `required`, `enabled_tools`, `disabled_tools`, `default_tools_approval_mode` (`auto`/`prompt`/`writes`/`approve`), per-tool `tools.<tool>.approval_mode` and `output_token_limit`; global `mcp_optional_startup_grace_ms` (default 1000).
- **Environment-variable references: no general interpolation.** Codex does **not** expand `${VAR}` in arbitrary strings. Secrets arrive only through `env_vars` (stdio forwarding), `bearer_token_env_var` (HTTP bearer), or `env_http_headers` (HTTP header). Values written under `[mcp_servers.<name>.env]` are literal, so a secret must never be written there. OAuth login uses `codex mcp login <server>`, with the callback on `http://127.0.0.1/callback` (configurable via `mcp_oauth_callback_port` / `mcp_oauth_callback_url`).

### Skills

- Codex reads `SKILL.md` skill folders in the open Agent Skills format (agentskills.io) — `developers.openai.com/codex/build-skills`.
- Locations: the repository's `$CWD/.agents/skills` and every parent up to `$REPO_ROOT/.agents/skills` (it walks up from the working directory), user `$HOME/.agents/skills`, admin `/etc/codex/skills`, plus the system-bundled skills. `~/.codex/skills` is **not** documented as a skill directory, even though `CODEX_HOME` holds skill metadata.
- Frontmatter requires `name` and `description`; optional `scripts/`, `references/`, `assets/` and `agents/openai.yaml` (interface metadata, `policy.allow_implicit_invocation`, MCP tool dependencies). Symlinked skill folders work. Individual skills can be disabled with `[[skills.config]] path = "…/SKILL.md" enabled = false`. Invoke with `/skills` or `$skill`.

### Chat channels

- **None for the local CLI.** The Slack and Teams integrations are part of ChatGPT/Codex **Cloud** (mention `@ChatGPT`, workspace admin setup, delegating to a published environment) and are not configurable from a local `config.toml`, so an Olympus recipe cannot set them up on the user's server. `supports.channels` should be empty for Codex.

### Authentication

- Two methods: **Sign in with ChatGPT** (subscription) or an **API key** (usage-based). (`developers.openai.com/codex/auth`.)
- `codex login` opens a browser; **`codex login --device-auth`** uses the OAuth device-code flow (headless-friendly). `codex login --with-api-key` and `codex login --with-access-token` take a credential on stdin/argument (`printenv CODEX_ACCESS_TOKEN | codex login --with-access-token`). `codex login status` exits 0 when credentials exist; `codex logout` clears them.
- `CODEX_API_KEY` supplies an API key to non-interactive processes (`codex exec`, review, the SDK, exec-server); `CODEX_ACCESS_TOKEN` supplies a ChatGPT/Codex access token.
- Credentials cache at `~/.codex/auth.json` (plaintext by default) or the OS keyring, shared between the CLI and IDE extensions. Custom model providers use `env_key = "<ENV_VAR>"` or `requires_openai_auth = true`.
- **Token lifetime: UNVERIFIED.** The docs only state that "Codex refreshes tokens automatically during use before they expire"; no expiry duration or refresh cadence is documented. Enterprise access tokens exist for trusted automation but their lifetime is not stated either.

### Does it rewrite its own config at runtime?

**Yes.** `codex mcp add`/`remove` write `config.toml`, and the app-server JSON-RPC methods `config/value/write` and `config/batchWrite` write `config.toml` on disk (`config/read`, `config/mcpServer/reload`, `skills/list` and `skills/config/write` also exist). `codex login` writes `auth.json`. Sessions and logs are written under `CODEX_HOME` as well. So `config.toml` and `auth.json` need to be writable (TRD 7.3 `writable: true`).

### Resources and licence

- `docs/install.md`: macOS 12+, Ubuntu 20.04+/Debian 10+, or Windows 11 via WSL2. **4 GB RAM minimum, 8 GB recommended.** No CPU or disk figure. Git 2.23+ is optional.
- Licence **Apache-2.0**.

### Other useful facts

- `codex exec` is the headless one-shot mode (`--json` for JSONL events); it defaults to a read-only sandbox (`--sandbox workspace-write` to widen), and has `--skip-git-repo-check`, `--ephemeral`, `--ignore-user-config`.
- `codex mcp-server` (a stdio MCP server that exposes Codex itself) is separate and experimental; `codex app-server` is the supported way to drive Codex programmatically.

---

## OpenCode

An open-source coding agent. Current docs at [opencode.ai/docs](https://opencode.ai/docs) (v1) with the v2 docs at [opencode.ai/v2/docs](https://opencode.ai/v2/docs); the repository moved to [github.com/anomalyco/opencode](https://github.com/anomalyco/opencode) (previously `sst/opencode`). The site footer read "Last updated: Oct 19, 2026".

### Official container image and tags

- Official image **`ghcr.io/anomalyco/opencode`**, documented in the install section of `opencode.ai/docs` (`docker run -it --rm ghcr.io/anomalyco/opencode`).
- Tags in GHCR at the time of checking: `latest` plus version tags `1.0.177` … `1.0.196`. No `dev` or `beta` tag was visible. The image is a multi-arch index for `amd64` and `arm64`.
- Image configuration: entrypoint `["opencode"]`, `WorkingDir /`, **no `USER`** (it runs as root), no `EXPOSE`, no `VOLUME`; the only environment is `PATH` and `BUN_RUNTIME_TRANSPILER_CACHE_PATH=0`. A recipe has to set the command (`serve --hostname 0.0.0.0 --port 4096`), the volumes and the port itself.
- Alternative installs: `curl -fsSL https://opencode.ai/v2/install | bash`, `npm i -g @opencode/cli@latest`, `brew install anomalyco/tap/opencode-v2`, pacman, mise, nix, and a desktop app.
- Licence **MIT** (`LICENSE`, Copyright 2025 opencode).

### Ports and health

- `opencode serve [--port N] [--hostname H] [--cors <origin>]`; defaults **port 4096**, **hostname 127.0.0.1** (`opencode.ai/docs/server`).
- **`GET /global/health`** returns `{"healthy": true, "version": "<string>"}`. The OpenAPI spec is at `/doc`; `/global/event` is an SSE stream. No richer readiness endpoint is documented.
- Optional HTTP basic auth: `OPENCODE_SERVER_PASSWORD` (username defaults to `opencode`, override with `OPENCODE_SERVER_USERNAME`). `opencode web` honours the same variables.

### Data directory, config file and format

- **Config file** `~/.config/opencode/opencode.json` — **JSON or JSONC** (`.jsonc` is also accepted; the docs show both `opencode.json` and `opencode.jsonc`). Project-level `opencode.json` sits at the repository root; a `.opencode/` directory is also read. `OPENCODE_CONFIG` overrides the file path and `OPENCODE_CONFIG_DIR` the directory; managed files live in `/etc/opencode/` on Linux.
- Sources are deep-merged, later overrides earlier: remote `.well-known/opencode` → global → `OPENCODE_CONFIG` → project → `.opencode/` directories → `OPENCODE_CONFIG_CONTENT` (inline) → managed files → macOS MDM.
- **Runtime/data directory** `~/.local/share/opencode/`: `auth.json`, `mcp-auth.json`, `log/`, sessions and storage. TUI settings live in `~/.config/opencode/tui.json`.

### MCP servers

- **File** `opencode.json`, **top-level key `mcp`** (`opencode.ai/docs/mcp-servers`).

Minimal stdio (local) server:

```json
{
  "mcp": {
    "fs": {
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/work"]
    }
  }
}
```

Keys: `type`, `command` (an **array**), `environment`, `cwd`, `enabled`, `timeout` (milliseconds, default 5000).

Minimal HTTP (remote) server:

```json
{
  "mcp": {
    "ctx": {
      "type": "remote",
      "url": "https://mcp.example.com/mcp",
      "headers": { "Authorization": "Bearer {env:MCP_TOKEN}" }
    }
  }
}
```

Keys: `type`, `url`, `headers`, `oauth` (object or `false`), `enabled`, `timeout`.

- OAuth is automatic (dynamic client registration, RFC 7591); `opencode mcp auth <name>` runs it, tokens land in `~/.local/share/opencode/mcp-auth.json`, and `oauth.clientId`/`clientSecret`/`scope` can be pre-registered. `opencode mcp list`, `opencode mcp logout` and `opencode mcp debug` exist. Tool names are prefixed `<server>_` and can be disabled with `tools: {"name_*": false}`.
- **Environment-variable references: yes.** `{env:VAR}` is substituted anywhere in the config (an unset variable becomes the empty string), and `{file:path}` substitutes a file's contents. Both are documented under Config → Variables. This is how Olympus can keep a secret out of `opencode.json`.

### Skills

- `SKILL.md` per folder (`opencode.ai/docs/skills`). Global: `~/.config/opencode/skills/<name>/SKILL.md`. Project: `.opencode/skills/<name>/SKILL.md`. OpenCode also reads the Claude-compatible `.claude/skills/<name>/SKILL.md` and the agent-compatible `.agents/skills/<name>/SKILL.md` (project and home). Discovery walks up to the git worktree root.
- Frontmatter requires `name` (1–64 characters, lowercase alphanumeric with single hyphens, must match the folder) and `description` (1–1024 characters); optional `license`, `compatibility` and `metadata` (a string map using keys such as `opencode/slash` and `opencode/autoinvoke`). `permission.skill` globs (allow/deny/ask) gate invocation, with per-agent overrides.

### Chat channels

- **None.** There is no Telegram/Discord/Slack channel for the agent. GitHub and GitLab integrations exist (repository automation, `opencode github`, `opencode gitlab`, GitLab Duo) but they are code-host automation, not chat channels. `supports.channels` should be empty for OpenCode.

### Authentication

- API keys are added through `/connect` in the TUI or `opencode auth login` (`--provider/-p`, `--method/-m` to skip the prompts) and stored in **`~/.local/share/opencode/auth.json`**. Headless provisioning is supported: the GitLab integration documents storing the whole auth JSON as a masked CI variable and writing it out (`opencode.ai/docs/gitlab`), which is the same file Olympus would provision. The exact JSON shape for each credential type is **UNVERIFIED** (the docs show that the file is provisioned wholesale, not its schema).
- Provider keys can also come from the standard environment variables (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, …), and `provider.<id>.options.apiKey` accepts `{env:…}` or `{file:…}`.
- **Subscriptions** (`opencode.ai/docs/providers`): ChatGPT Plus/Pro is offered as an OpenAI `/connect` option; GitHub Copilot uses a device-code flow at `github.com/login/device`; GitLab Duo (Premium/Ultimate) works with no setup; OpenCode Go is OpenCode's own low-cost subscription (API key); OpenCode Zen is pay-as-you-go. **Claude Pro/Max is prohibited** by Anthropic for use with OpenCode, and plugins that enabled it were removed as of OpenCode 1.3.0.
- Headless subscription login: GitHub Copilot's device-code flow is documented; the ChatGPT flow opens a browser and no fully headless path is documented, so headless ChatGPT subscription login is **UNVERIFIED**. Token lifetime and refresh are not documented (**UNVERIFIED**).

### Does it rewrite its own config at runtime?

- OpenCode writes `~/.local/share/opencode/auth.json` and `mcp-auth.json` through `/connect`, `opencode auth login` and `opencode mcp auth`, so the runtime data directory must be writable.
- The server also exposes `PATCH /config` ("Update config"); whether that persists `opencode.json` to disk is **UNVERIFIED**. No normal operation is documented to rewrite `opencode.json` itself, but since `/connect` writes into the data directory, a read-only data volume would break login. Treat the config file as managed and the data directory as writable; use `writable: true` if the recipe relies on `/connect`.

### Resources and licence

- **No minimum RAM, CPU or disk is documented** in the install, config, server, troubleshooting or repository pages checked — **UNVERIFIED**. It ships as a single Bun-compiled binary and the image is small.
- Licence **MIT**.

### Version note

The site advertises "New OpenCode v2 is now available" and the install commands default to v2 (`@opencode/cli@latest`), while the `/docs/*` pages are v1 docs; `/v2/docs/server` returned 404 during this check. The recipe should pin a concrete image tag (for example `ghcr.io/anomalyco/opencode:1.0.196`) rather than `latest`, and the v1/v2 CLI compatibility of the recipe's command line should be re-checked when the recipe is written.

---

## Answers to TRD section 16

- **Per-tool MCP secret delivery — environment placeholders vs a wrapper reading `/run/secrets`: environment variables work for all three, no wrapper needed.**
  - Hermes: write `${VAR}` (or `${env:VAR}`) in `mcp_servers` values; the value comes from the process environment or `$HERMES_HOME/.env`.
  - OpenCode: write `{env:VAR}` anywhere in `opencode.json`. `{file:…}` is also available if Olympus would rather point at a file than an env var.
  - Codex: there is **no** string interpolation. Olympus writes the environment variable **name** into `env_vars`, `bearer_token_env_var` or `env_http_headers`, and must never write a secret value into `config.toml`.
  - So the core can deliver every MCP secret as an environment variable on the container and let each tool resolve it from there. If secrets are staged as files (`/var/lib/olympus/secrets/<tool>/<name>`, TRD 8.4), the activator still has to expose them as environment variables (env file or entrypoint export) for these three tools; none of them reads a secret from `/run/secrets` by itself. `{file:…}` in OpenCode is the one exception that accepts a file path directly.
  - Hermes caveat to encode in the recipe: an unset `${VAR}` in a remote `url`/`headers` fails closed (good), but a stdio `env` entry that is exactly one unset reference is silently dropped, so keep such entries to a single reference or handle their absence.

- **Skills — copy or shared read-only mount: a shared mount is possible, but it fits each tool differently.**
  - Codex reads `$HOME/.agents/skills` (and walks up from the working directory), OpenCode reads `~/.config/opencode/skills`, `.agents/skills` and `.claude/skills`, and Hermes reads `$HERMES_HOME/skills` plus `skills.external_dirs`.
  - A single shared directory mounted read-only can serve all three only if each tool is pointed at a path it accepts (`.agents/skills` works for Codex and OpenCode; Hermes needs it listed in `skills.external_dirs`). Hermes warns that external skill directories are mutable and that the primary skills directory is agent-writable, so a read-only mount may make some skill operations fail. Recommendation: use a shared read-only mount for Codex and OpenCode, and copy into the volume for Hermes (or accept Hermes' write failures). The decision is a core question, not settled by this research.

- **`writable: true` copy approach for tools that rewrite config: confirmed necessary for all three.**
  - Hermes writes `config.yaml` and `.env`; Codex writes `config.toml` and `auth.json`; OpenCode writes `auth.json` (and `mcp-auth.json`, possibly `opencode.json` through `PATCH /config`). Read-only bind mounts would break `hermes auth`/`hermes mcp install`, `codex login`/`codex mcp add` and `opencode auth login`. All three recipes should mark their managed config writable and let the activator copy it into the tool's volume.

- **Integration test servers: not answered here.** All three need outbound network and a provider credential. OpenCode's `/global/health` and Codex's app-server `/readyz` (experimental) give HTTP health signal; Hermes needs `API_SERVER_ENABLED=true` for its `/health`. Codex has no stable HTTP health endpoint, so its recipe should use `health.command`.

- **Catalog URL scheme and git requirement: not answered here, with one added constraint.** OpenCode publishes a v2 line while the v1 docs are still what the site serves under `/docs`, so the catalog must pin concrete tags/digests, not `latest`, and the Codex recipe has no upstream image digest to pin at all (OpenAI publishes no image). The recipe format may need a way to express a built image or an install step for Codex; recorded as an open question, not decided here.

- **CLI framework (cobra vs stdlib): not addressed by this research.**

## Sources

All links read on **2026-10-19**.

| Tool | Source | Used for |
|---|---|---|
| Hermes Agent | `references/hermes-agent` (HEAD `6e9b3e3b`) — `website/docs/user-guide/docker.md`, `configuration.md`, `mcp.md`, `skills.md`, `messaging/index.md`, `messaging/telegram.md`, `messaging/discord.md`, `guides/oauth-over-ssh.md`; `Dockerfile`; `docker-compose.yml`; `LICENSE` | everything in the Hermes section |
| Hermes Agent | [hermes-agent.nousresearch.com/docs](https://hermes-agent.nousresearch.com/docs) | published copy of the above (site paths not re-checked) |
| Codex | [github.com/openai/codex](https://github.com/openai/codex) — README, `docs/install.md`, `LICENSE` | image absence, install methods, resources, licence |
| Codex | [developers.openai.com/codex/config-file](https://developers.openai.com/codex/config-file/) | `CODEX_HOME`, `config.toml`, precedence, `auth.json` |
| Codex | [developers.openai.com/codex/extend/mcp](https://developers.openai.com/codex/extend/mcp) | MCP table, keys, examples, env-var handling |
| Codex | [developers.openai.com/codex/auth](https://developers.openai.com/codex/auth) | login flows, device auth, token env vars, lifetime |
| Codex | [developers.openai.com/codex/build-skills](https://developers.openai.com/codex/build-skills) | skill locations and format |
| Codex | [developers.openai.com/codex/app-server](https://developers.openai.com/codex/app-server) | stdio/socket/WebSocket, `/readyz`, `/healthz`, config-write methods |
| Codex | [developers.openai.com/codex/non-interactive-mode](https://developers.openai.com/codex/non-interactive-mode) | `codex exec` |
| OpenCode | [opencode.ai/docs](https://opencode.ai/docs) and [opencode.ai/docs/server](https://opencode.ai/docs/server) | image and install, `serve`, port 4096, `/global/health`, basic auth |
| OpenCode | [opencode.ai/docs/config](https://opencode.ai/docs/config/) | config file paths, JSONC, merge order, variables |
| OpenCode | [opencode.ai/docs/mcp-servers](https://opencode.ai/docs/mcp-servers/) | `mcp` key, local/remote shapes, OAuth, `{env:…}` |
| OpenCode | [opencode.ai/docs/skills](https://opencode.ai/docs/skills/) | skill directories and frontmatter |
| OpenCode | [opencode.ai/docs/providers](https://opencode.ai/docs/providers/) | API keys, subscriptions, Claude Pro/Max restriction |
| OpenCode | [opencode.ai/docs/cli](https://opencode.ai/docs/cli/) | `auth login`, `mcp auth`, `serve` flags |
| OpenCode | [opencode.ai/docs/gitlab](https://opencode.ai/docs/gitlab/) | headless provisioning of auth JSON |
| OpenCode | [opencode.ai/docs/troubleshooting](https://opencode.ai/docs/troubleshooting/) | data directory paths |
| OpenCode | [github.com/anomalyco/opencode](https://github.com/anomalyco/opencode) — `LICENSE` | licence, repository move |
