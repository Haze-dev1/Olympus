# Recipe catalog

- **Status:** In progress
- **Milestone:** M5
- **Plan:** [plans/v1.md](../../plans/v1.md)

Recipes for the remaining major tools: Claude Code, Gemini CLI, OpenClaw, pi, oh-my-pi, Goose, Aider, Grok bot, OpenSpaces.

## Acceptance

- Each recipe passes the [recipe checklist](../../specs/recipe.md#checklist-for-a-new-recipe).

## Claude Code (OLY-19)

**Status:** implemented offline; live-server verification not run.

Files: `recipes/claude-code/{recipe.yaml,Dockerfile,README.md}`,
`testdata/claude-code/stack.yaml`, `testdata/claude-code/skills/deploy-checklist/SKILL.md`,
`testdata/claude-code/golden/` (rendered fixture).

Decisions:

- Built image, no upstream container: Anthropic documents its dev container as
  "a working example rather than a maintained base image" and recommends
  `npm install -g @anthropic-ai/claude-code@X.Y.Z` plus `DISABLE_AUTOUPDATER`
  in your own Dockerfile. Uses the existing `build` mechanism (ADR 0035) on
  `node:22-bookworm-slim` (Node 22+ is the package's engine floor).
- Pin: `@anthropic-ai/claude-code@2.1.318` (`latest` on 2026-11-06; `stable`
  was `2.1.311`), verified with `npm view @anthropic-ai/claude-code@2.1.318
  version engines` → `2.1.318`, `{ node: '>=22.0.0' }`.
- MCP file: `/root/.mcp.json` (project scope, `mcpServers` key), read-only bind
  from the generation. Read-only rather than `writable` because Claude Code
  does not rewrite `.mcp.json` at runtime; approvals live in `~/.claude.json`,
  which stays on the `home` volume. No init service is needed (ADR 0036 not
  triggered).
- MCP style: `hermes`, which emits `command`/`args`/`env` with `${VAR}` — the
  exact stdio shape and expansion Claude Code documents for `.mcp.json`
  (ADR 0037 reused, not amended).
- Skills: `/root/.claude/skills`, `mode: mount`; Claude Code reads personal
  skills there and only writes `synced/` for claude.ai skills (ADR 0038).
- Secret: `model-key` → `ANTHROPIC_API_KEY` (A1, A5); `github-token` →
  `GITHUB_TOKEN` exists only for MCP entries that reference it.
- Health: `claude --version` (no documented HTTP endpoint). Container command
  stays `sleep infinity` so the CLI can be exec'd.

Verification (offline, 2026-11-06):

- `go build`, `go vet`, `gofmt -l .` and `go test ./...` pass (see the PR for
  the command output); the macOS-only `internal/deploy` failures listed in
  AGENTS.md are unrelated and untouched.
- Rendered generation for `testdata/claude-code/stack.yaml` (temporary local
  harness using the existing `eval` package; the harness was deleted, so no Go
  code is part of this change):
  - `/root/.mcp.json` = `{"mcpServers":{"github":{"args":["-y",
    "@modelcontextprotocol/server-github"],"command":"npx","env":{"GITHUB_TOKEN":
    "${GITHUB_TOKEN}"}}}}` — documented stdio shape, reference not value.
  - `compose.yaml` binds `/root/.mcp.json`, `/root/.claude/skills/deploy-checklist/SKILL.md`
    and the Dockerfile read-only, mounts the `claude_home` volume at `/root`,
    sets `BASE_IMAGE` from the locked digest and uses the content-addressed
    `olympus/claude:<hash>` tag.
  - `manifest.json` and `compose.yaml` contain no secret value and no
    sentinel; the manifest lists the two secret names only.

Known gaps:

- **Remote (`url`) MCP servers do not work.** Claude Code requires
  `"type": "http"|"sse"|"ws"` on a `url` entry and skips an entry without it;
  `translateMCP` has no style that emits `type`. A local harness build of a
  `url` server rendered `{"url": ..., "headers": {...}}` with no `type`.
  stdio servers are unaffected. Closing this needs a new MCP style (Go change),
  so it is reported here and in the PR instead of being worked around.
- **No committed automated test.** Adding a Go test would mean editing
  `internal/eval` (and `testdata/real` / `testdata/golden-real`), which this
  change is scoped out of. The fixture under `testdata/claude-code/golden/` is
  ready for a follow-up `TestClaudeCodeRecipe` that pins `Images["claude"]`.
- **No live lab run.** No QEMU on the host, so `docker build`,
  `claude --version`, interactive `.mcp.json` approval and the
  `ANTHROPIC_API_KEY` interactive prompt were not exercised.
- **UNVERIFIED:** whether the npm install's native-binary download uses extra
  egress beyond the registry; `DISABLE_AUTOUPDATER` behaviour inside this
  image. Both are recorded in `recipes/claude-code/README.md`.

ADR: none added. The recipe reuses ADRs 0035 (built images), 0037 (MCP
translation styles) and 0038 (skills mounting); no format, durable behaviour or
architecture changed.
