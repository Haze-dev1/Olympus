# Claude Code recipe

Sources checked **2026-11-06** against Anthropic's Claude Code documentation and
the npm registry; the pinned release's prompt strings, update flags and `.mcp.json`
discovery were re-checked on **2026-11-22**.

## Checked facts

- **No maintained container image.** Anthropic ships an example dev container in
  `anthropics/claude-code` and describes it as "provided as a working example
  rather than a maintained base image"; the same page says to pin a version by
  running `npm install -g @anthropic-ai/claude-code@X.Y.Z` in your own
  Dockerfile and setting `DISABLE_AUTOUPDATER`. The recipe therefore uses
  `build` with a recipe-supplied Dockerfile on `node:22-bookworm-slim`, which
  is a normal registry image the lock can pin by digest.
  Sources: <https://code.claude.com/docs/en/devcontainer> (install and pinning
  guidance), <https://code.claude.com/docs/en/setup> (npm install, Node.js 22
  or later).
- **Install.** `npm install -g @anthropic-ai/claude-code`; the npm package
  requires Node.js 22 or later (<https://code.claude.com/docs/en/setup>). The
  Dockerfile runs `npm ci --omit=dev --ignore-scripts` from the committed
  `package.json` and `package-lock.json` in `/opt/claude-code` and puts
  `node_modules/.bin` on `PATH`, so every transitive package is pinned by
  integrity hash (ADR 0051). To move the pin, edit `package.json` and run
  `npm install --package-lock-only --ignore-scripts` in this folder.
- **Version pin.** `@anthropic-ai/claude-code@2.1.318`, the `latest` dist-tag
  on 2026-11-06; `stable` was `2.1.311`.
  Source: <https://registry.npmjs.org/@anthropic-ai/claude-code/2.1.318>.
- **Resources.** 4 GB+ RAM, x64 or ARM64 CPU
  (<https://code.claude.com/docs/en/setup>). `resources.min_ram: 4GB`.
- **npm install egress.** The native binary is an npm optional dependency
  (`@anthropic-ai/claude-code-linux-x64`, `-linux-arm64` and musl variants), and
  `install.cjs`, the package's `postinstall` script, only links it into place;
  it does not download from another host. The build therefore needs only the
  npm registry. Checked in the 2.1.318 tarball on 2026-11-06. Do not add
  `--omit=optional` to the install. npm runs no lifecycle scripts; the
  Dockerfile invokes only that one audited script by path after `npm ci`,
  because the package's `bin` is an error stub until it runs (R5).
- **Health.** `claude --version` prints the installed version
  (<https://code.claude.com/docs/en/setup>). There is no documented HTTP
  endpoint for the CLI, so the recipe uses a command check.
- **Model key.** `ANTHROPIC_API_KEY` is sent as the `X-Api-Key` header. In
  non-interactive `-p` runs the key is always used when present; in interactive
  sessions Claude Code asks once before the key overrides a subscription
  (<https://code.claude.com/docs/en/env-vars>). Declared as the `model-key`
  secret; the recipe never writes the value.
- **Updates.** `DISABLE_AUTOUPDATER=1` disables background updates and keeps
  the pinned release; `claude update` still works. `DISABLE_UPDATES=1` blocks
  manual updates too (<https://code.claude.com/docs/en/env-vars>, re-checked
  against the pinned 2.1.318 package on 2026-11-22). The recipe sets
  `DISABLE_AUTOUPDATER` because that is what the pinning guidance above
  recommends. The keys' effect on the exact container was not observed; see
  UNVERIFIED.
- **MCP config file and scope.** Claude Code reads project-scope servers from
  `.mcp.json` at the project root under the `mcpServers` key; local and user
  scope live in `~/.claude.json`. The container's working directory is `/root`
  (`WORKDIR`), so the managed file is `/root/.mcp.json`. A check against the
  pinned 2.1.318 binary loaded both a parent and a child `.mcp.json`, so a
  session in a clone under `/root` does see the managed servers; the real limit
  is workspace trust/approval for that directory, not file discovery
  (<https://code.claude.com/docs/en/mcp>, "MCP installation scopes" and
  "Project server approvals and workspace trust", checked 2026-11-22).
- **MCP entry shape (stdio).** `command`, `args`, `env`
  (<https://code.claude.com/docs/en/mcp>, "Add a server from an `mcpServers`
  JSON block"). This is the shape the `hermes` translation style emits, so
  stdio servers render exactly as documented.
- **MCP secrets.** `${VAR}` and `${VAR:-default}` are expanded in a server's
  `command`, `args`, `env`, `url` and `headers` in `.mcp.json`, which means the
  generation writes environment variable references only
  (<https://code.claude.com/docs/en/mcp>).
- **MCP approval.** Interactive sessions prompt once before loading
  project-scope servers from `.mcp.json`; `claude -p` runs and Agent SDK
  sessions load them without asking, and the answer is stored in
  `~/.claude.json` (<https://code.claude.com/docs/en/mcp>, "Project scope" and
  "Project server approvals and workspace trust").
- **Skills.** Personal skills are read from `~/.claude/skills/<name>/SKILL.md`
  and are available in all projects
  (<https://code.claude.com/docs/en/skills>, "Where skills live"). Claude Code
  reads that directory; it only writes there for skills synced from claude.ai,
  so the recipe mounts the shared skills read-only (`mode: mount`).
- **State.** Claude Code stores its settings and session state under `~/.claude`
  and `~/.claude.json`; the recipe keeps both on the `home` volume (`/root`) so
  they survive upgrades (<https://code.claude.com/docs/en/settings>).
- **Secrets declared.** `model-key` → `ANTHROPIC_API_KEY`, and `github-token`
  → `GITHUB_TOKEN` for MCP servers whose entries reference it (the generic
  stack MCP secret wiring needs a declared target). Claude Code itself does not
  read `GITHUB_TOKEN`.
- **Channels.** The local CLI has no chat-channel configuration, so the recipe
  declares none.

## Remote MCP servers

Claude Code requires `"type": "http"`, `"sse"` or `"ws"` on an entry that has a
`url`; an entry with a `url` and no `type` is a configuration error and Claude
Code skips that server
(<https://code.claude.com/docs/en/mcp>, "Option 1: Add a remote HTTP server").

The recipe uses `mcps.style: claude-code`. That style renders stdio entries as
`command`/`args`/`env` and remote entries as `type` + `url` + `headers`, with
`type` taken from the stack's `transport`: `http` (the default) or `sse`
([ADR 0047](../../docs/ADR.md#0047-carry-an-explicit-remote-transport-for-mcp-servers)).
The recipe does not emit `ws`, because the stack file has no way to ask for it;
a WebSocket MCP server cannot be configured from an Olympus stack yet.

`testdata/claude-code/stack.yaml` now carries one remote server and its golden
contains `"type": "http"`, so the rendered shape is covered by evidence. What
is not covered is a live connection: no running Claude Code session has loaded
that file (see the gaps below).

### Config file location

The managed file is still `/root/.mcp.json`. The earlier claim that a session
started in a cloned repository cannot see it was wrong: a check with the pinned
2.1.318 binary loaded `.mcp.json` from both the working directory and its
parent, so a clone under `/root` inherits the managed servers.

The other documented locations are root-owned `~/.claude.json` (local and user
scope), whose format and content the CLI rewrites for its own state, and on
Linux the managed file `/etc/claude-code/managed-mcp.json`, same `mcpServers`
format (<https://code.claude.com/docs/en/managed-mcp>).

`managed-mcp.json` is the wrong choice here: when it exists it takes
exclusive control of MCP, the user cannot add or use other servers, and
`--mcp-config` refuses to run. That would make a normal Olympus image
surprising and block every other way of adding a server, so the recipe keeps
the project-scope file.

Approval, not discovery, is the remaining limit: in an interactive session,
servers from a cloned repository can stay at "Pending approval" until the
workspace is trusted. `claude -p` runs and Agent SDK sessions load project-scope
servers without asking (<https://code.claude.com/docs/en/mcp>, "Project server
approvals and workspace trust"). No live approval prompt was driven; see
UNVERIFIED.

## Verification and remaining gaps

- **Interactive key approval.** The prompt strings exist in the pinned 2.1.318
  binary, but no model request was made and no interactive session was driven.
- **Image build and health check on a real server — verified 2026-11-22.** A
  three-tool apply on a reset QEMU VM built the image, started the container and
  passed the CLI health check. `olym status` reported `running healthy`,
  `docker exec olympus-claude-code-1 claude --version` printed
  `2.1.318 (Claude Code)`, and the container had zero restarts. The `.mcp.json`
  approval flow was not exercised there.
- **`DISABLE_AUTOUPDATER` inside the image.** The variable and its scope
  (background updates only) are documented and present in the pinned release;
  no live update attempt was observed in the container.
- **Live MCP connection and workspace trust.** Parent-directory `.mcp.json`
  discovery is verified from the pinned binary; no running Claude Code session
  connected to a server or drove an approval prompt.

## Reproducibility

Builds are reproducible in their inputs (base digest, package version,
Dockerfile content), not byte-identical: the Codex recipe found that a clean
rebuild produced a different image ID even with `SOURCE_DATE_EPOCH=0` and
normalized timestamps, and this Dockerfile uses the same approach. On a server,
the content-addressed tag (`olympus/<tool name>:sha256-…`) means one generation
reuses one image for activation and rollback.
