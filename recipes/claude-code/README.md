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
  (`WORKDIR`), so the managed file is `/root/.mcp.json`.
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

## Format gap: remote MCP servers

Claude Code requires `"type": "http"`, `"sse"` or `"ws"` on an entry that has a
`url`; an entry with a `url` and no `type` is a configuration error and Claude
Code skips that server
(<https://code.claude.com/docs/en/mcp>, "Option 1: Add a remote HTTP server").

The recipe format cannot express that yet: `translateMCP` supports the `hermes`,
`codex` and `opencode` styles only, and the `hermes` style emits `url` and
`headers` without a `type` field
([ADR 0037](../../docs/ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references)).
As a result:

- **stdio servers work** (`command`/`args`/`env` with `${VAR}`), which is what
  the recipe's stack in `testdata/claude-code/` uses and what the golden
  evidence covers;
- **`url`-based servers do not.** They are still rendered, but Claude Code
  skips them with `MCP server "<name>" has a "url" but no "type"`. Until a
  dedicated style (or a per-style extra field) exists, point the stack at a
  stdio server instead.

This gap is recorded in `docs/tracker/features/recipe-catalog.md` and in the
pull request that added the recipe; it is not worked around in Go.

## UNVERIFIED

- **Interactive key approval.** The docs say an interactive session asks once
  before `ANTHROPIC_API_KEY` overrides a subscription. That prompt was not
  exercised: no model request was made, and no interactive session was driven.
- **Image build and health check on a real server.** No QEMU lab VM was
  available for this change, so `docker build`, `claude --version` and the
  `.mcp.json` approval flow were not run against the built image. Only offline
  rendering was verified.
- **npm install egress.** The npm package "downloads a native binary that
  doesn't use your Node.js at runtime"
  (<https://code.claude.com/docs/en/setup>). It is not documented whether that
  download uses the npm registry or a separate host, so the build may need
  egress beyond the registry. Not observed.
- **`DISABLE_AUTOUPDATER` inside the image.** The variable is documented; its
  effect on this exact pinned release inside a container was not observed.

## Reproducibility

Builds are reproducible in their inputs (base digest, package version,
Dockerfile content), not byte-identical: the Codex recipe found that a clean
rebuild produced a different image ID even with `SOURCE_DATE_EPOCH=0` and
normalized timestamps, and this Dockerfile uses the same approach. On a server,
the content-addressed tag (`olympus/claude-code:sha256-…`) means one generation
reuses one image for activation and rollback.
