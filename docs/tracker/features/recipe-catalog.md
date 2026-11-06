# Recipe catalog

- **Status:** In progress: OpenBot and Claude Code recipes added and rendered offline; none lab-verified.
- **Milestone:** M5
- **Plan:** [plans/v1.md](../../plans/v1.md)

Recipes for the remaining major tools: Claude Code, Gemini CLI, OpenBot, OpenClaw, pi, oh-my-pi, Goose, Aider, Grok bot and OpenSpaces.

## Acceptance

- Each recipe passes the [recipe checklist](../../specs/recipe.md#checklist-for-a-new-recipe).
- Every path, port and image in it is checked against upstream docs for the pinned release.

## Evidence

- 2026-11-06, OLY-56 — `recipes/openbot` written for
  <https://github.com/nightly-labs/openbot> at release `v0.62.0`, read from
  `docs/docker.md`, `docs/self-hosted-server.md`, `docker/compose.yaml`,
  `docker/seccomp.json` and `LICENSE` at that tag. Image
  `ghcr.io/nightly-labs/openbot:0.62.0` confirmed in the GHCR registry on
  2026-11-06 (index
  `sha256:e0673c02cf6a9e576a613f2fe2220d76fda64cacc2c563385e460450419dadac`;
  `linux/amd64` and `linux/arm64` manifests resolved); `latest`, `v<version>`
  and `sha-<commit>` tags exist but none is used.
- Recipe checklist: image pinned by exact tag and resolved by the lock (A7);
  no port published, so A4 is not exercised; three optional provider secrets
  declared with no values (A1); one named `/data` volume; health
  `openbot status`, which needs no sign-in; no managed config, writable file or
  skills path exists in the pinned docs, so none is declared and no `supports`
  block is claimed.
- Runtime: `shm_size: 1g`, `stop_grace_period: 60s`, `no_new_privileges: true`
  and `seccomp: seccomp.json` reproduce upstream's `docker/compose.yaml` and
  `docker run` example. The profile is copied verbatim from the same tag
  (SHA-256 `665289e6a1c41045f55166d0b01993c929f71df78e3a3b264f3252bb8e0b3d9e`):
  Docker's default profile plus one rule that lets `clone` and `unshare` make
  user namespaces for the Electron sandbox. Vendoring and the exact pin are
  [ADR 0046](../../ADR.md#0046-vendor-openbots-seccomp-profile-and-pin-an-exact-release).
- Tests: `testdata/openbot/stack.yaml` builds the shipped recipe with all three
  secrets, and `TestOpenBotGolden` compares the generation against
  `testdata/golden-openbot/`. Asserted: the pinned image, only the expected
  four files, `shm_size`, `stop_grace_period`, `no-new-privileges:true`, the
  `seccomp=/var/lib/olympus/store/<tool hash>/files/data/.olympus/seccomp.json`
  option, the `olympus_openbot_data` volume at `/data`, the manifest health
  command `[openbot, status]`, and that no `ports:` key is rendered.
  `testdata/real/` and `testdata/golden-real/` are untouched.
- `olym check` and `olym build` accept the stack with sentinel values in a
  throwaway `-f` directory (three files, no secret in the output).
- `go build`, `go vet ./...`, `go test ./...` and `gofmt -l .` pass locally.
  Three pre-existing macOS-only `internal/deploy` tests fail with
  `operation not permitted` on the ownership/mode assertions
  (`TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`,
  `TestToolEntryReuseAndVerification`); they are unrelated to this change.
- Not run: the QEMU lab (`lab/lab.sh` needs `qemu-system-x86_64`, absent on this
  machine), the OpenBot container itself (Docker is not installed), and the
  one-time `openbot login`. Upstream's own "Not confirmed" list is repeated in
  the recipe README. Sign-in, provider authentication and the account session
  remain unverified.

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
- **Golden test (added at merge review, 2026-11-06).** `TestRecipeGoldens` in
  `internal/eval/recipes_golden_test.go` renders `testdata/claude-code/stack.yaml`
  with a fake `node:22-bookworm-slim` digest and compares it with
  `testdata/claude-code/golden/`; a changed fixture fails it.
- **No live lab run.** No QEMU on the host, so `docker build`,
  `claude --version`, interactive `.mcp.json` approval and the
  `ANTHROPIC_API_KEY` interactive prompt were not exercised.
- **Egress resolved at review:** the native binary is an npm optional
  dependency and `postinstall` only links it, so the build needs only the npm
  registry.
- **UNVERIFIED:** `DISABLE_AUTOUPDATER` behaviour inside this image, and whether
  a session started outside `/root` still reads `/root/.mcp.json`. Both are
  recorded in `recipes/claude-code/README.md`.

ADR: none added. The recipe reuses ADRs 0035 (built images), 0037 (MCP
translation styles) and 0038 (skills mounting); no format, durable behaviour or
architecture changed.
