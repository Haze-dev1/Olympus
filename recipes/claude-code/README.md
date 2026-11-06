# Claude Code recipe

Sources checked **2026-11-06** against Anthropic's Claude Code documentation and
the npm registry.

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
  Dockerfile installs to `/opt/claude-code` and puts its `bin` on `PATH`
  instead of touching the base image's global prefix, matching the Codex
  recipe.
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
  `--omit=optional` to the install.
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
  manual updates too (<https://code.claude.com/docs/en/env-vars>). The recipe
  sets `DISABLE_AUTOUPDATER` because that is what the pinning guidance above
  recommends.
- **MCP config file and scope.** Claude Code reads project-scope servers from
  `.mcp.json` at the project root under the `mcpServers` key; local and user
  scope live in `~/.claude.json`. The container's working directory is `/root`
  (`WORKDIR`), so the managed file is `/root/.mcp.json`. The docs place
  `.mcp.json` at the project root and do not say Claude Code reads a parent
  folder's file, so start `claude` in `/root` to get the shared servers. A
  session started inside a cloned repository may not see them (UNVERIFIED).
  Source: <https://code.claude.com/docs/en/mcp> ("MCP installation scopes",
  "Project scope").
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
that file (see UNVERIFIED).

### Config file location

The managed file is still `/root/.mcp.json`. An open question from the issue was
whether a project-scope `.mcp.json` loads from a cloned repository. The docs say:

- Project scope is `.mcp.json` at the project root and nothing in the docs says
  Claude Code walks up to a parent folder. A session started elsewhere therefore
  does not see `/root/.mcp.json` unless you start it in `/root`.
- The other documented locations are root-owned `~/.claude.json` (local and user
  scope), whose format and content the CLI rewrites for its own state, and on
  Linux the managed file `/etc/claude-code/managed-mcp.json`, same `mcpServers`
  format (<https://code.claude.com/docs/en/managed-mcp>).
- `managed-mcp.json` is the wrong choice here: when it exists it takes
  exclusive control of MCP, the user cannot add or use other servers, and
  `--mcp-config` refuses to run. That would make a normal Olympus image
  surprising and block every other way of adding a server, so the recipe keeps
  the project-scope file and documents the limit instead.

Nothing offline can prove approval behaviour, so this remains a documented
limitation rather than a verified fix: in an interactive session, servers from a
cloned repository can stay at "Pending approval" until the workspace is trusted.
`claude -p` runs and Agent SDK sessions load project-scope servers without
asking (<https://code.claude.com/docs/en/mcp>, "Project server approvals and
workspace trust").

## UNVERIFIED

- **Interactive key approval.** The docs say an interactive session asks once
  before `ANTHROPIC_API_KEY` overrides a subscription. That prompt was not
  exercised: no model request was made, and no interactive session was driven.
- **Image build and health check on a real server.** No QEMU lab VM was
  available for this change, so `docker build`, `claude --version` and the
  `.mcp.json` approval flow were not run against the built image. Only offline
  rendering was verified.
- **`DISABLE_AUTOUPDATER` inside the image.** The variable is documented; its
  effect on this exact pinned release inside a container was not observed.

## Reproducibility

Builds are reproducible in their inputs (base digest, package version,
Dockerfile content), not byte-identical: the Codex recipe found that a clean
rebuild produced a different image ID even with `SOURCE_DATE_EPOCH=0` and
normalized timestamps, and this Dockerfile uses the same approach. On a server,
the content-addressed tag (`olympus/<tool name>:sha256-…`) means one generation
reuses one image for activation and rollback.
