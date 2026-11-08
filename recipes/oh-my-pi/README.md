# Oh My Pi recipe

Sources checked **2026-11-06** against official repository documentation and
GitHub releases.

## Checked facts

- **Identity & repository.** Official repository is `can1357/oh-my-pi`
  (<https://github.com/can1357/oh-my-pi>). Verified via GitHub API on
  2026-11-06:
  `gh api repos/can1357/oh-my-pi` → `fork: false`.
  Description: "⌥ Coding agent with the IDE wired in. Built by Stencil Labs."
  Official site: <https://omp.sh>.
- **Tool classification.** Oh My Pi runs as `omp`, a standalone terminal coding agent
  with native Rust IDE integration, tree-sitter AST patching, and in-process subagents.
  It is a separately runnable tool, not an extension or config pack for Pi.
- **Version pin & binary download.** Exact GitHub release `v18.17.0` (`2026-11-06T05:12:54Z`,
  <https://github.com/can1357/oh-my-pi/releases/tag/v18.17.0>).
  The recipe builds from `node:22-bookworm-slim` and downloads the precompiled standalone
  `omp-linux-<arch>` binary, verified against the release's `SHA256SUMS.txt`:
  - `amd64` (`x64`): `a4ce6001ab1012ca835f4184d675437531bd1ac477a7ba8424ed0b5476e9e4cb`
  - `arm64`: `a9423fd7a69d869bf514931226b21d4ff55474225f574e2911fec457fc470e68`
  Downloaded directly from GitHub Releases:
  `https://github.com/can1357/oh-my-pi/releases/download/v18.17.0/omp-linux-${ARCH}`.
- **Resources.** Multi-model coding agent with native AST patching and embedded tooling.
  `resources.min_ram: 4GB` is an Olympus sizing choice for coding agents with native AST
  parsing and embedded tooling, matching `recipes/claude-code`, `recipes/codex`, and `recipes/pi`.
- **State directory.** User-level configuration and session state live under
  `~/.omp/agent` (`docs/config-usage.md` at tag `v18.17.0`,
  <https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/config-usage.md>).
  The recipe mounts a named volume `home` at `/root`, keeping `/root/.omp/agent`
  across upgrades.
- **Updates & settings configuration.**
  - Upstream Oh My Pi has no environment variables for skipping version checks or disabling
    telemetry (there are no `OMP_SKIP_VERSION_CHECK` or `OMP_TELEMETRY` variables).
  - Version checking and auto-updates are controlled by persistent settings in
    `~/.omp/agent/config.yml` (path resolved in `packages/coding-agent/src/config/settings.ts:736-737`
    via `getAgentDir()` from `packages/utils/src/dirs.ts:641` and `MAIN_CONFIG_FILENAMES[0]` = `"config.yml"`
    from `packages/utils/src/dirs.ts:32`, documented in
    <https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/settings.md>):
    - `startup.checkUpdate` (boolean, default `true`, defined in
      `packages/coding-agent/src/modes/settings.ts:1283` as `cfgStartupCheckUpdate`).
    - `update.auto` (boolean, default `false`, defined in
      `packages/coding-agent/src/modes/settings.ts:1318` as `cfgUpdateAuto`).
  - In `packages/coding-agent/src/main.ts:275-278`, `checkForNewVersion` skips network update
    checks when both `startup.checkUpdate` and `update.auto` are `false`.
  - The recipe installs a template `config.yml.tmpl` at `/root/.omp/agent/config.yml` seeding
    `startup.checkUpdate: false` and `update.auto: false` so that background auto-updates and
    startup update checks are disabled (A7). Because Oh My Pi rewrites `config.yml` at runtime
    when `/settings` or `omp config set` is run, the file is marked `writable` and delivered into
    the volume by the `oh-my-pi-init` service. Olympus owns this file: re-applying the stack
    re-renders it from the recipe template, overwriting interactive settings edits made inside the
    container. Other files under `/root` (sessions, chat history) are preserved.
- **Command & health check.** Container runs `command: [sleep, infinity]`.
  Health check is `command: [omp, --version]` with `timeout: 60s`
  (<https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/cli-reference.md>).
- **Model key & secrets.** Primary provider credential is `ANTHROPIC_API_KEY`,
  declared as secret `model-key`
  (<https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/environment-variables.md>).
  `GITHUB_TOKEN` is declared as secret `github-token` for MCP servers that reference it.
- **MCP configuration & writable delivery.**
  - Config file: `/root/.omp/agent/mcp.json` under `mcpServers` in JSON format
    (<https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/mcp-config.md>).
  - Shape: stdio servers use `command`, `args`, and `env` with `${VAR}` expansion;
    HTTP servers require `"type": "http"` (or `"sse"`), `url`, and `headers` with `${VAR}` expansion.
  - Style: matches Olympus's `claude-code` style (`supports.mcps.style: claude-code`),
    which emits `"type": "http"`.
  - Delivery (`writable`): Oh My Pi rewrites `/root/.omp/agent/mcp.json` when configuring MCP servers,
    so the file is marked `writable` and delivered into the volume by the `oh-my-pi-init` service.
    Olympus owns this file: re-applying the stack re-evaluates `mcp.json` from the stack and recipe,
    overwriting any interactive edits made inside the container. Other files under `/root`
    (sessions, chat history) are preserved.
- **Skills.** OMP discovers skills from `~/.omp/agent/skills/` and `~/.agents/skills/`
  (<https://github.com/can1357/oh-my-pi/blob/v18.17.0/docs/skills.md>).
  The recipe declares `supports.skills.dir: /root/.omp/agent/skills` with `mode: mount`.

## Gaps and unverified items

- **No live lab run.** No QEMU is available on the macOS test harness, so live
  container execution was not run.
- **UNVERIFIED:** live interactive sessions, token consumption against active model
  provider APIs, and LSP in-process diagnostics.
