# OpenClaw recipe

Sources checked **2026-11-06** against OpenClaw's official repository at
<https://github.com/openclaw/openclaw>, release `v2026.11.1`, and upstream documentation.

## Checked facts

- **Official repository.** <https://github.com/openclaw/openclaw> is the official
  repository (verified via `gh api repos/openclaw/openclaw`: `"fork": false`,
  `"full_name": "openclaw/openclaw"`, 394,000+ stars).
- **Version pin.** `v2026.11.1` published on GitHub Releases on 2026-11-05
  (verified via `gh api repos/openclaw/openclaw/releases/tags/v2026.11.1` → tag `v2026.11.1`,
  published `2026-11-05T09:48:37Z`).
- **Official container image.** OpenClaw publishes official pre-built container images
  to the GitHub Container Registry (`ghcr.io/openclaw/openclaw`) and Docker Hub mirror
  (`openclaw/openclaw`) (<https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/install/docker.md>, 2026-11-06).
  The recipe uses `ghcr.io/openclaw/openclaw:2026.11.1`.
  Verified via GHCR registry API on 2026-11-06:
  Multi-architecture index digest:
  `sha256:db0e0809fbd0df375445e9eea8daf2a278ac7ac3a263fa6bfce8fb344ad3ad77`
  covering `linux/amd64` (manifest `sha256:c504dec53e7f7ad588624e782db1643e1971410a23c99ccbdbb2e0c7a79324cf`)
  and `linux/arm64` (manifest `sha256:3bed3bfd841a7e543d044a0b09e72960ec2ce0977c83ea784ac41cd0ad4f5507`).
- **Image configuration from registry.**
  Read from the `linux/amd64` image config blob (`sha256:45b7944d9e433c9d39deb00d660b23b9a0979565128e2e0ef1651bc6c6163c7f`):
  - `User`: `node` (UID 1000)
  - `WorkingDir`: `/app`
  - `Entrypoint`: `["tini", "-s", "--", "node", "/app/docker-entrypoint.mjs"]`
  - `Cmd`: `["node", "openclaw.mjs", "gateway"]`
  - `ExposedPorts`: none declared in the image configuration
  - `Volumes`: none declared in the image configuration
- **Entrypoint behavior.** Upstream's `/app/docker-entrypoint.mjs` inspects the supplied
  command (`process.argv.slice(2)`), executes unattended maintenance (`doctor --fix --non-interactive`)
  on container startup/image upgrade, and then invokes `process.execve(program, command, process.env)`.
  The entrypoint does **not** swallow `command`; it delegates cleanly to the configured command.
- **Ports and bind mode.**
  - The Gateway multiplexes WebSocket and HTTP on a single port, defaulting to `18789`
    (`DEFAULT_GATEWAY_PORT = 18789` in `src/config/paths.ts`; `<https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/help/faq/gateway-ports-and-remote-mode.md>`, 2026-11-06).
  - Rule A4: Compose binds this port strictly to `127.0.0.1:18789:18789` on the host, preventing public exposure.
  - Inside the container, `command` passes `--bind lan` (`0.0.0.0`) so the container network
    interface receives traffic forwarded from host localhost under Docker bridge networking.
    (Upstream documents: "With Docker bridge networking (-p 18789:18789), loopback bind makes
    the gateway unreachable from the host... Override --bind to 'lan' (0.0.0.0)" in `Dockerfile`
    and `docker-compose.yml`, 2026-11-06).
- **Resources.** 2 GB RAM is the documented requirement for running the pre-built image
  (<https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/install/gcp.md>,
  <https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/install/digitalocean.md>, 2026-11-06).
  The recipe sets `resources.min_ram: 2GB`.
- **State and workspace volume.**
  - `ENV HOME=/home/node`, `USER node`.
  - State directory defaults to `~/.openclaw` (`/home/node/.openclaw`), resolved by
    `resolveStateDir()` in `src/config/state-dir.ts` (<https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/install/docker/networking-and-storage.md>, 2026-11-06).
  - The upstream Dockerfile pre-creates `/home/node/.openclaw` with `node:node 0700` ownership
    and permissions. Docker named-volume initialization preserves image ownership and permissions.
  - Configuration (`openclaw.json`), SQLite state database (`state/openclaw.sqlite`), agent databases
    (`agents/<agentId>/agent/openclaw-agent.sqlite`), credentials (`credentials/`), and workspace
    (`workspace/`) all reside inside `/home/node/.openclaw`.
  - The recipe mounts the named volume `data` at `/home/node/.openclaw` and a second volume `auth`
    at `/home/node/.config/openclaw`, where upstream keeps auth-profile secrets
    (`docker-compose.yml` mounts it as `OPENCLAW_AUTH_PROFILE_SECRET_DIR`). The Dockerfile
    pre-creates both as `node:node 0700` (`Dockerfile` at `v2026.11.1`, the `install -d` step), so
    the empty named volumes inherit that ownership.
- **First start without onboarding.** A fresh state volume has no `openclaw.json`, and the gateway
  refuses to start unless `gateway.mode=local` is configured. `--allow-unconfigured` "Start[s]
  without enforcing `gateway.mode=local`" (`docs/cli/gateway/running.md` at `v2026.11.1`), and
  upstream's own Render blueprint runs `node openclaw.mjs gateway --allow-unconfigured`
  (`docs/install/render.mdx`). It does not waive the shared-secret requirement for a non-loopback
  bind, which `OPENCLAW_GATEWAY_TOKEN` satisfies. After the first apply, configure OpenClaw inside
  the container, e.g. `docker exec -it <container> node dist/index.js onboard --mode local
  --no-install-daemon`; the config persists in the `data` volume. Checked 2026-11-06.
- **Runtime options.** `runtime.no_new_privileges: true` reproduces upstream's
  `security_opt: [no-new-privileges:true]` from `docker-compose.yml` (ADR 0045).
- **Health check.** Upstream's container health check probes `GET /healthz` on port 18789
  (`src/docker-healthcheck.ts`, <https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/gateway/health.md>, 2026-11-06).
  `/healthz` returns HTTP 200 without authentication. The recipe declares an HTTP check:
  `health.http.port: 18789`, `health.http.path: /healthz`, `timeout: 60s`.
- **Secrets.**
  - `model-key`: provider API key delivered as `ANTHROPIC_API_KEY` (`.env.example`, 2026-11-06).
  - `gateway-token`: gateway authentication token delivered as `OPENCLAW_GATEWAY_TOKEN`
    (`.env.example`, `docker-compose.yml`, 2026-11-06).
- **Skills support.** OpenClaw discovers managed file-backed skills in `<state-dir>/skills`
  (`Priority 4: Managed / local skills: <state-dir>/skills`, `src/skills/loading/skill-paths.ts`,
  <https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/tools/skills.md>, 2026-11-06).
  Resolved inside the volume as `/home/node/.openclaw/skills`. The recipe mounts skills read-only
  (`supports.skills.mode: mount`) per the recipe checklist.

## Format gaps & STOP rule

- **MCP servers.** OpenClaw configures MCP client connections under `mcp.servers` in its unified
  `openclaw.json` configuration file (`src/config/zod-schema.mcp-server.ts`,
  <https://github.com/openclaw/openclaw/blob/v2026.11.1/docs/gateway/config-extensions.md>, 2026-11-06).
  For remote servers, OpenClaw expects `transport: "streamable-http"` or `"sse"`.
  Olympus's existing MCP translation styles (`hermes`, `codex`, `opencode`, `claude-code`, `gemini-cli`)
  emit different transport shapes (e.g. `type: "http"`, `httpUrl`, or no transport key) and target
  standalone MCP files. Following the project STOP rule (ADR 0037, recipe checklist), `supports.mcps`
  is omitted rather than worked around.
- **Chat channels.** OpenClaw supports chat channels (Telegram, Discord, Slack), but configures
  them via structured keys in `openclaw.json` (`channels.telegram.botToken`, `channels.telegram.allowFrom`,
  `channels.telegram.dmPolicy: "pairing"`). It does not configure channels or allow-lists via
  environment variables (`TELEGRAM_ALLOWED_USERS` does not exist in OpenClaw). Olympus's channel
  translation contract (ADR 0039, Rule A10) requires both token and allow-list environment variables.
  Because OpenClaw cannot be configured with channel allow-lists purely through environment variables,
  `supports.channels` is omitted.

## UNVERIFIED

- **Live model session execution.** No live inference was performed against active model provider credentials.
- **Live container run on a server.** No QEMU lab VM was available on this host, so container startup,
  unattended doctor execution, and live HTTP health check were not run against a live Docker daemon.
  Offline generation rendering was verified.
- **Control UI browser pairing.** Browser-based Control UI device pairing and WebSocket RPC operations
  were verified from upstream code and documentation, but not executed in a live browser session.
