# Oh My Pi recipe

Sources checked **2026-11-06** against official repository documentation and
GitHub releases.

## Checked facts

- **Identity & repository.** Official repository is `can1357/oh-my-pi`
  (<https://github.com/can1357/oh-my-pi>). Verified via GitHub API on
  2026-11-06:
  `gh api repos/can1357/oh-my-pi` → `fork: false`, `stargazers_count: 38025`.
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
- **Resources.** Multi-model coding agent with ~80k lines of Rust core and embedded tooling.
  `resources.min_ram: 4GB` (<https://omp.sh>).
- **State directory.** User-level configuration and session state live under
  `~/.omp/agent` (`docs/config-usage.md` at tag `v18.17.0`,
  <https://github.com/can1357/oh-my-pi/blob/main/docs/config-usage.md>).
  The recipe mounts a named volume `home` at `/root`, keeping `/root/.omp/agent`
  across upgrades.
- **Environment variables.**
  - `OMP_SKIP_VERSION_CHECK=1` disables remote version checks
    (<https://github.com/can1357/oh-my-pi/blob/main/docs/environment-variables.md>).
  - `OMP_TELEMETRY=0` disables telemetry and provider attribution headers.
- **Command & health check.** Container runs `command: [sleep, infinity]`.
  Health check is `command: [omp, --version]` with `timeout: 60s`
  (<https://github.com/can1357/oh-my-pi/blob/main/docs/cli-reference.md>).
- **Model key & secrets.** Primary provider credential is `ANTHROPIC_API_KEY`,
  declared as secret `model-key`
  (<https://github.com/can1357/oh-my-pi/blob/main/docs/environment-variables.md>).
  `GITHUB_TOKEN` is declared as secret `github-token` for MCP servers that reference it.
- **MCP configuration.**
  - Config file: `/root/.omp/agent/mcp.json` under `mcpServers` in JSON format
    (<https://github.com/can1357/oh-my-pi/blob/main/docs/mcp-config.md>).
  - Shape: stdio servers use `command`, `args`, and `env` with `${VAR}` expansion;
    HTTP servers require `"type": "http"` (or `"sse"`), `url`, and `headers` with `${VAR}` expansion.
  - Style: matches Olympus's `claude-code` style (`supports.mcps.style: claude-code`),
    which emits `"type": "http"`.
- **Skills.** OMP discovers skills from `~/.omp/agent/skills/` and `~/.agents/skills/`
  (<https://github.com/can1357/oh-my-pi/blob/main/docs/skills.md>).
  The recipe declares `supports.skills.dir: /root/.omp/agent/skills` with `mode: mount`.

## Gaps and unverified items

- **No live lab run.** No QEMU is available on the macOS test harness, so live
  container execution was not run.
- **UNVERIFIED:** live interactive sessions, token consumption against active model
  provider APIs, and LSP in-process diagnostics.
