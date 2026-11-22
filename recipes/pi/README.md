# Pi recipe

Sources checked **2026-11-06** against official repository documentation and
the npm registry; the pinned version and install inputs were re-checked against
the registry and the tagged upstream docs on **2026-11-21**.

## Checked facts

- **Identity & repository.** Official repository is `earendil-works/pi`
  (<https://github.com/earendil-works/pi>). Verified via GitHub API on
  2026-11-06:
  `gh api repos/earendil-works/pi` → `fork: false`.
  Description: "AI agent toolkit: unified LLM API, agent loop, TUI, coding agent CLI".
  Created by Mario Zechner (`badlogic`), maintained under Earendil Works.
- **Package.** Published on npm as `@earendil-works/pi-coding-agent`
  (<https://registry.npmjs.org/@earendil-works/pi-coding-agent>).
- **Version pin.** `package.json` pins the exact version `1.5.0` (`latest` on
  2026-11-06), re-checked against the npm registry on 2026-11-21. npm `latest`
  has since moved to `1.6.0` (2026-11-20), but `1.5.0` remains the correct pin
  for v1 and its tagged docs and lockfile have been checked together. The committed
  `package-lock.json` (lockfile v3) pins every transitive package by integrity
  hash and `npm ci` installs only those packages (OLY-62, ADR 0051). To move the
  pin, edit `package.json` and run
  `npm install --package-lock-only --ignore-scripts` here.
- **Package scripts & install egress.** The pinned package has no install-time
  script that downloads external binaries or assets (its `scripts` are build and
  publish helpers only), and its native dependency
  `@silvia-odwyer/photon-node@0.3.4` declares none. Following upstream's
  containerization guide, the Dockerfile passes `--ignore-scripts` to `npm ci`
  so no transitive install script can reach the network. The package requires
  Node.js `>=22.19.0` (npm registry metadata, re-checked 2026-11-21); the
  `node:22-bookworm-slim` tag currently carries Node `22.23.5` (image config,
  re-checked 2026-11-21), and the lock pins its digest.
- **No official maintained container image.** Upstream documentation
  (`packages/coding-agent/docs/containerization.md` at tag `v1.5.0`,
  <https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/containerization.md>,
  re-read 2026-11-21) documents containerization with a Dockerfile running
  `npm install -g --ignore-scripts @earendil-works/pi-coding-agent`. The recipe
  uses `build` with a recipe-supplied Dockerfile based on `node:22-bookworm-slim`
  that `COPY`s the two lockfiles and installs them into `/opt/pi` with
  `npm ci --omit=dev --ignore-scripts`, putting `/opt/pi/node_modules/.bin` on
  `PATH` with normalized timestamps, matching `recipes/codex` and
  `recipes/gemini-cli` after OLY-62.
- **Resources.** Node.js coding agent with QuickJS codemode, ripgrep, and AST parsing.
  Pinned upstream documentation (`packages/coding-agent/docs/containerization.md` at tag `v1.5.0`,
  <https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/containerization.md>)
  does not specify an upstream minimum memory requirement; `resources.min_ram: 4GB` is an Olympus
  sizing choice for coding agents with AST parsing, ripgrep, and QuickJS worker pools, matching
  `recipes/claude-code` and `recipes/codex`.
- **State directory.** User-level configuration and session state live under
  `~/.pi/agent` (`getAgentDir()` in `packages/coding-agent/src/config.ts`,
  <https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/src/config.ts>).
  The recipe mounts a named volume `home` at `/root`, keeping `/root/.pi/agent`
  across upgrades.
- **Environment variables.**
  - `PI_SKIP_VERSION_CHECK=1` disables remote version checks against `pi.dev`
    (<https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/environment-variables.md>).
  - `PI_TELEMETRY=0` disables telemetry and provider attribution headers.
- **Command & health check.** Container runs `command: [sleep, infinity]`.
  Health check is `command: [pi, --version]` with `timeout: 60s`
  (<https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/cli.md>).
- **Model key & secrets.** Primary provider credential is `ANTHROPIC_API_KEY`,
  declared as secret `model-key`
  (<https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/providers.md>).
  `GITHUB_TOKEN` is declared as secret `github-token` for MCP servers that reference it.
- **MCP configuration & writable delivery.**
  - Config file: `/root/.pi/agent/mcp.json` under `mcpServers` in JSON format
    (<https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/mcp.md>).
  - Shape: stdio servers use `command`, `args`, and `env` with `${VAR}` expansion;
    HTTP servers use `url` and `headers` with `${VAR}` expansion.
  - Style: matches Olympus's `hermes` style (`supports.mcps.style: hermes`), with
    `supports.mcps.sse: false` because Pi rejects legacy SSE endpoints. An entry
    with `transport: sse` now fails `check` and `build` instead of failing silently
    in the container. Streamable HTTP servers are supported.
  - Delivery (`writable`): Pi rewrites `/root/.pi/agent/mcp.json` when adding or modifying tools,
    so the file is marked `writable` and delivered into the volume by the `pi-init` service.
    Olympus owns this file: re-applying the stack re-evaluates `mcp.json` from the stack and recipe,
    overwriting any interactive edits made inside the container. Other files under `/root`
    (sessions, chat history) are preserved.
- **Skills.** Pi discovers skills from `~/.pi/agent/skills/` and `~/.agents/skills/`
  (<https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/skills.md>).
  The recipe declares `supports.skills.dir: /root/.pi/agent/skills` with `mode: mount`.
- **Relationship to oh-my-pi.** Oh My Pi (`omp`, <https://github.com/can1357/oh-my-pi>)
  originated as a fork of Pi with native Rust IDE integration, but is published as a
  standalone tool (`@oh-my-pi/pi-coding-agent`) with its own CLI (`omp`) and release binaries.
  It is not a configuration pack for Pi and cannot be layered into Pi as an extension;
  it is packaged separately in `recipes/oh-my-pi`.

## Gaps and lab verification

- **Lab run completed 2026-11-21.** A single-tool apply on a reset QEMU VM with
  `source <(lab/lab.sh env)` and a development CLI (`-unsafe-dev-activator`)
  printed `pi --version` → `1.5.0`, kept `${GITHUB_TOKEN}` in
  `/root/.pi/agent/mcp.json`, mounted the shared skill read-only, delivered
  secrets as `0600` files, published no host port and ended with zero container
  restarts in 92.6 s; `olym destroy` kept `olympus_pi_home`. The evidence is
  recorded in `docs/tracker/features/recipe-catalog.md`. The image build and
  health check are therefore verified on a live daemon, not just by golden
  rendering.
- **UNVERIFIED:** live interactive sessions, token consumption against active
  model provider APIs, and WebSocket MCP servers (unsupported by stack format).
