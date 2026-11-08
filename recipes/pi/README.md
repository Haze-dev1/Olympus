# Pi recipe

Sources checked **2026-11-06** against official repository documentation and
the npm registry.

## Checked facts

- **Identity & repository.** Official repository is `earendil-works/pi`
  (<https://github.com/earendil-works/pi>). Verified via GitHub API on
  2026-11-06:
  `gh api repos/earendil-works/pi` → `fork: false`.
  Description: "AI agent toolkit: unified LLM API, agent loop, TUI, coding agent CLI".
  Created by Mario Zechner (`badlogic`), maintained under Earendil Works.
- **Package.** Published on npm as `@earendil-works/pi-coding-agent`
  (<https://registry.npmjs.org/@earendil-works/pi-coding-agent>).
- **Version pin.** Pinned to exact version `1.5.0` (`latest` on 2026-11-06).
  Verified with `npm view @earendil-works/pi-coding-agent@1.5.0 version` → `1.5.0`.
- **Package scripts & install egress.** Verified with
  `npm view @earendil-works/pi-coding-agent@1.5.0 scripts`. There is no
  `postinstall` script downloading external binaries or assets. The package
  requires Node.js `>=22.19.0` (`npm view @earendil-works/pi-coding-agent@1.5.0 engines`).
- **No official maintained container image.** Upstream documentation
  (`packages/coding-agent/docs/containerization.md` at tag `v1.5.0`,
  <https://github.com/earendil-works/pi/blob/v1.5.0/packages/coding-agent/docs/containerization.md>)
  documents containerization with a Dockerfile running
  `npm install -g --ignore-scripts @earendil-works/pi-coding-agent`. The recipe
  uses `build` with a recipe-supplied Dockerfile based on `node:22-bookworm-slim`,
  installing into `--prefix /opt/pi` and putting `/opt/pi/bin` on `PATH` with
  normalized timestamps, matching `recipes/claude-code` and `recipes/codex`.
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
  - Style: matches Olympus's `hermes` style exactly (`supports.mcps.style: hermes`).
  - Note: Pi rejects legacy SSE endpoints (`"sse"`). Streamable HTTP servers are supported.
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

## Gaps and unverified items

- **No live lab run.** No QEMU is available on the macOS test harness, so live
  container execution was not run.
- **UNVERIFIED:** live interactive sessions, token consumption against active model
  provider APIs, and WebSocket MCP servers (unsupported by stack format).
