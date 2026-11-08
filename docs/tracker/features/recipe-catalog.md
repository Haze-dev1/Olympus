# Recipe catalog

- **Status:** In progress: OpenBot, Claude Code, Gemini CLI, Goose, Aider, OpenClaw, Pi and Oh My Pi recipes added and rendered offline; none lab-verified.
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

## Gemini CLI (OLY-20)

**Status:** implemented offline; live-server verification not run.

Files: `recipes/gemini-cli/{recipe.yaml,Dockerfile,settings.json.tmpl,README.md}`,
`testdata/gemini-cli/stack.yaml`, `testdata/gemini-cli/skills/deploy-checklist/SKILL.md`,
`testdata/gemini-cli/golden/` (rendered fixture).

Decisions:

- Built image, no upstream container: Gemini CLI's official container is its
  `--sandbox` image (`ghcr.io/google/gemini-cli:latest` by default), documented
  for isolating tool execution, while the documented host-side install is an
  exact npm release. Uses the existing `build` mechanism (ADR 0035) on
  `node:22-bookworm-slim`.
- Pin: `@google/gemini-cli@0.67.0` (`latest` on 2026-11-06), verified with
  `npm view @google/gemini-cli@0.67.0 version engines` → `0.67.0`,
  `{ node: '>=20' }`.
- MCP file: `/root/.gemini/settings.json` under the top-level `mcpServers` key,
  `format: json` (`docs/cli/settings.md`, `docs/tools/mcp-server.md` at
  `v0.67.0`).
- MCP style: `hermes` (ADR 0037 reused, not amended). It emits `command`,
  `args`, `env` with `${VAR}`, and `url`/`headers` — the documented stdio and
  SSE shapes. Gemini CLI expands `${VAR}` inside `env` values and inside every
  string in `settings.json`, and it expands `headers` values too, so secret
  references never become values.
- `writable`: `/root/.gemini/settings.json`. The CLI rewrites that file through
  `/settings`, so it is copied into the `home` volume by a `gemini-init`
  service (ADR 0036) instead of a read-only bind. A re-apply re-renders it, so
  in-container `/settings` edits do not survive an apply; sessions and history
  under `/root` do.
- Template seeds `general.enableAutoUpdate: false`. The documented default is
  `true`, which would update the installed npm package inside the container and
  break the pin (A7).
- Skills: `/root/.gemini/skills`, `mode: mount`, read-only (ADR 0038).
  `~/.agents/skills/` is a documented alias; the primary `~/.gemini/skills/`
  path was chosen.
- Secret: `model-key` → `GEMINI_API_KEY` (A1, A5); `github-token` →
  `GITHUB_TOKEN` for MCP entries that reference it. The CLI redacts
  `GEMINI_API_KEY` and `*TOKEN*` variables from MCP children's inherited
  environment, and the `hermes` `env` listing is the documented way to pass a
  variable explicitly to one server.
- Health: `gemini --version` (60 s), the documented install check. Container
  command stays `sleep infinity`.
- `resources.min_ram: 4GB`, the docs' "casual" usage guidance.

Verification (offline, 2026-11-06):

- `go build`, `go vet`, `gofmt -l .` clean; `go test ./...` green except the
  three macOS-only `internal/deploy` failures listed in AGENTS.md (unrelated,
  untouched).
- `olym check -f testdata/gemini-cli/stack.yaml -recipes recipes` with a
  temporary sentinel `.env` → "Stack is valid." (exit 0).
- Rendered generation for `testdata/gemini-cli/stack.yaml` (temporary local
  harness using the existing `eval` package; harness deleted, so no Go code is
  part of this change):
  - `/root/.gemini/settings.json` = `{"general":{"enableAutoUpdate":false},
    "mcpServers":{"docs":{"headers":{"Authorization":"Bearer
    ${GITHUB_TOKEN}"},"url":"https://mcp.example.test/sse"},"github":{"args":
    ["-y","@modelcontextprotocol/server-github"],"command":"npx","env":
    {"GITHUB_TOKEN":"${GITHUB_TOKEN}"}}}}` — documented stdio and SSE shapes,
    references not values.
  - `compose.yaml` mounts the `gemini_home` volume at `/root`, binds the skill
    and Dockerfile read-only, and runs `gemini-init` to copy `settings.json`
    into the volume before the tool starts.
  - `manifest.json` and `compose.yaml` contain no secret value and no sentinel;
    the manifest lists the two secret names only.

- **Remote MCP servers (OLY-57, 2026-11-06):** Resolved the remote MCP format
  gaps for Claude Code and Gemini CLI. Added `transport: http | sse` (default
  `http`) to the stack specification and added `claude-code` (`type: http|sse`)
  and `gemini-cli` (`httpUrl` for http, `url` for sse) translation styles in
  `internal/eval`. Switched `recipes/claude-code` and `recipes/gemini-cli` to their
  respective styles. Stacks in `testdata/claude-code` and `testdata/gemini-cli`
  exercise remote servers and their goldens are regenerated. Golden-real output
  remains byte-identical. ADR 0047 amends ADR 0037.
- **Golden test (added at merge review, 2026-11-06).** `TestRecipeGoldens`
  also renders `testdata/gemini-cli/stack.yaml` and compares it with
  `testdata/gemini-cli/golden/`.
- **No live lab run.** No QEMU on the host, so `docker build`,
  `gemini --version`, first-run auth with `GEMINI_API_KEY`, `/settings`
  persistence, and a live MCP connection were not exercised.
- **UNVERIFIED:** whether the npm install needs egress beyond the registry;
  whether the CLI writes `settings.json` on first interactive run before
  `/settings` is used (the `writable` delivery makes that safe either way);
  whether a user-level `mcpServers` entry still prompts for approval on first
  run. All three are recorded in `recipes/gemini-cli/README.md`.

ADRs: ADR 0046 (vendored seccomp and pinned release for OpenBot); ADR 0047 (explicit remote MCP transport and tool styles for Claude Code and Gemini CLI, amending 0037).


## Goose (OLY-23)

**Status:** implemented offline; live-server verification not run.

Files: `recipes/goose/{recipe.yaml,Dockerfile,README.md}`,
`testdata/goose/stack.yaml`, `testdata/goose/skills/deploy-checklist/SKILL.md`,
`testdata/goose/golden/` (rendered fixture).

Decisions:

- Built image, no maintained upstream container: Goose documentation describes
  Docker usage as building from source via `documentation/docs/docker/Dockerfile`
  and Compose. Legacy registry tags on `ghcr.io/block/goose` stopped at `1.9.0`
  and `ghcr.io/aaif-goose/goose` stopped at `1.30.0`. The recipe uses the
  existing `build` mechanism (ADR 0035) on `node:22-bookworm-slim`.
- Pin: `v1.57.0`, published 2026-10-28 on GitHub Releases, verified with
  `curl -sL https://api.github.com/repos/aaif-goose/goose/releases/tags/v1.57.0 | jq -r .tag_name` → `v1.57.0`.
  Assets `goose-<arch>-unknown-linux-musl.tar.gz` verified for both `amd64`
  (`sha256:6c3a05e30e1a425dc06a410dbe24b3f3697c9721b1ea2340434ddda6b64267ae`) and `arm64`
  (`sha256:00a32522f0421ed72fc06f36bfc44c8e51cffefb4daf22a81bb84ccd29d1a1ce`), checked with
  `sha256sum -c` during image build.
- Skills: mounted read-only at `/root/.agents/skills` (`mode: mount`), the
  canonical global user skills directory (`global_skills_dir()` in
  `crates/goose/src/skills/mod.rs` at `v1.57.0`).
- Secret and provider: `model-key` → `OPENAI_API_KEY` (A1, A5). Upstream
  `GOOSE_PROVIDER` and `GOOSE_MODEL` have no default and must be configured
  (`documentation/docs/guides/environment-variables.md` at tag `v1.57.0`), so the
  recipe sets `env: GOOSE_PROVIDER: openai`. Model selection is via `GOOSE_MODEL`
  or interactive `goose configure` inside the container.
- Health: `goose --version` (60 s), which prints `goose 1.57.0` and exits 0.
  Container command stays `sleep infinity` so the CLI can be exec'd.
- `resources.min_ram: 4GB`.

Verification (offline, 2026-11-06):

- `go build`, `go vet`, `gofmt -l .` clean; `go test ./...` green except the
  three macOS-only `internal/deploy` failures listed in AGENTS.md (unrelated,
  untouched).
- `olym check -f testdata/goose/stack.yaml -recipes recipes` with a temporary
  sentinel `.env` → "Stack is valid." (exit 0).
- Rendered generation for `testdata/goose/stack.yaml` via `TestRecipeGoldens`:
  - `compose.yaml` binds `/root/.agents/skills/deploy-checklist/SKILL.md` and the
    Dockerfile read-only, sets `GOOSE_PROVIDER: openai` in the environment,
    mounts the `goose_home` volume at `/root`, sets `BASE_IMAGE` from the locked
    digest and uses content-addressed `olympus/goose:<hash>` tag.
  - `manifest.json` and `compose.yaml` contain no secret value and no sentinel;
    the manifest lists `model-key` only.

Known gaps:

- **MCP servers (extensions) do not work.** In Goose, extensions require
  `type: stdio` (tagged enum `#[serde(tag = "type")]`) and `cmd: string` (and
  `uri` for remote streamable_http); Olympus `translateMCP` has no style that
  emits `type: stdio`, `cmd`, or `uri`. Because Goose skips entries missing
  `type`/`cmd` as malformed, `supports.mcps` is omitted per the project STOP
  rule rather than worked around.
- **Golden test.** `TestRecipeGoldens` in `internal/eval/recipes_golden_test.go`
  renders `testdata/goose/stack.yaml` and compares it with `testdata/goose/golden/`.
- **No live lab run.** No QEMU on the host, so `docker build`,
  `goose --version`, and live model sessions were not exercised.
- **UNVERIFIED:** live model session execution against an active API key.

ADR: none added. Reuses ADRs 0035 (built images) and 0038 (skills mounting); no
format, durable behaviour or architecture changed.

- **Merge review, 2026-11-06:** the release tarball has a `./` entry owned by uid 1001, so the Dockerfile now extracts only `./goose` with `--no-same-owner` and `/usr/local/bin` keeps root ownership. Asset digests re-checked against the GitHub API; the amd64 tarball was downloaded and verified. The golden was regenerated.

## Aider (OLY-24)

**Status:** implemented offline; live-server verification not run.

Files: `recipes/aider/{recipe.yaml,Dockerfile,README.md}`,
`testdata/aider/stack.yaml`, `testdata/aider/golden/` (rendered fixture).

Decisions:

- Base container image: `paulgauthier/aider:v0.86.2` on Docker Hub, verified
  via Docker Hub registry API (index digest
  `sha256:0d54037f6c97c85a926ebc3c903e27446f5bca96823d9f68902f1e623289b556`,
  covering `linux/amd64` and `linux/arm64`). PyPI package `aider-chat==0.86.2`
  verified via PyPI API.
- Built image (ADR 0035): upstream sets `ENTRYPOINT ["/venv/bin/aider"]`, causing
  `command: [sleep, infinity]` to be passed as file arguments to aider, which
  exits immediately without a TTY. A derived `Dockerfile` builds on `${BASE_IMAGE}`
  and resets `ENTRYPOINT []`, allowing detached sleep execution.
- No `supports` block: Aider natively supports neither MCP client connections nor
  skills directories. As instructed by the recipe checklist and project guidance,
  no `supports` block is declared.
- State: `/app` volume (`WORKDIR /app`, `USER appuser` in upstream image, and
  `ENV HOME=/app`). All config (`.aider.conf.yml`), chat/input history
  (`.aider.*.history`), repo map cache (`.aider.tags.cache.v*`), and `~/.aider/`
  resolve inside `/app`. Docker volume initialization preserves image ownership
  (`appuser:appuser`, UID 1000) and permissions, making `/app` writable by `appuser`.
- Secret: `model-key` → `OPENAI_API_KEY` (A1, A5).
- Health: `aider --version` (60 s).
- `resources.min_ram: 4GB`.

Verification (offline, 2026-11-06):

- `go build`, `go vet`, `gofmt -l .` clean; `go test ./...` green except the
  three macOS-only `internal/deploy` failures listed in AGENTS.md (unrelated,
  untouched).
- `olym check -f testdata/aider/stack.yaml -recipes recipes` with a temporary
  sentinel `.env` → "Stack is valid." (exit 0).
- Rendered generation for `testdata/aider/stack.yaml` via `TestRecipeGoldens`:
  - `compose.yaml` declares service `aider` with `build:` block on `${BASE_IMAGE}`,
    `image: olympus/aider:sha256-...`, mounts `aider_app` volume at `/app`,
    bind-mounts `/app/.olympus-build/Dockerfile`, command `[sleep, infinity]`, and
    delivers secret `env_file`.
  - `manifest.json` tracks file hash `/app/.olympus-build/Dockerfile` and tool hash,
    and lists `model-key` secret only.
- `TestRecipeGoldens` in `internal/eval/recipes_golden_test.go` renders and
  passes against `testdata/aider/golden/`.

Known gaps:

- **No live lab run.** No QEMU on the host, so `docker build`, `docker pull`,
  `aider --version`, and container execution were not exercised against a live daemon.
- **UNVERIFIED:** live model session execution against an active API key; runtime
  Playwright browser installation under `/home/appuser/pw-browsers` if `--browser`
  is used on the core image.

ADR: none added. Reuses ADR 0035 (built images) and existing recipe format; no
format, durable behaviour or architecture changed.

- **Merge review, 2026-11-06:** the entrypoint fix and the `HOME=/app`, appuser and `/app` ownership facts were checked against the image config in the registry and upstream `docker/Dockerfile` at v0.86.2. `TestRecipeGoldens` now pins each recipe's own image with a fake digest, so the Aider golden shows `paulgauthier/aider:v0.86.2` as the build base instead of a placeholder Node image; the other goldens are unchanged.

## OpenClaw (OLY-21)

**Status:** implemented offline; live-server verification not run.

Files: `recipes/openclaw/{recipe.yaml,README.md}`,
`testdata/openclaw/stack.yaml`, `testdata/openclaw/skills/deploy-checklist/SKILL.md`,
`testdata/openclaw/golden/` (rendered fixture).

Decisions:

- Official container image: `ghcr.io/openclaw/openclaw:2026.11.1` on GHCR
  (multi-arch index digest `sha256:db0e0809fbd0df375445e9eea8daf2a278ac7ac3a263fa6bfce8fb344ad3ad77`,
  covering `linux/amd64` and `linux/arm64`), verified via GHCR API. Matches GitHub Release
  tag `v2026.11.1` (`2026-11-05T09:48:37Z`) on the non-fork official repo
  `openclaw/openclaw` (`gh api repos/openclaw/openclaw` → `fork: false`, 394k+ stars).
- Image configuration (read from registry amd64 config `sha256:45b7944d…`):
  `User: node` (UID 1000), `WorkingDir: /app`, `Entrypoint: ["tini", "-s", "--", "node", "/app/docker-entrypoint.mjs"]`,
  `Cmd: ["node", "openclaw.mjs", "gateway"]`, no exposed ports, no volume declarations.
  Upstream entrypoint runs unattended doctor maintenance on container startup and delegates
  cleanly via `process.execve` without swallowing `command`.
- Ports & binding: default Gateway multiplexed port `18789` (`ports: [18789]`), bound by
  Compose strictly to `127.0.0.1:18789:18789` (Rule A4, never public). Inside the container,
  `command: [node, openclaw.mjs, gateway, --bind, lan, --port, "18789"]` sets `--bind lan`
  (0.0.0.0) so Docker bridge port forwarding from host localhost reaches the listener.
- State: `/home/node/.openclaw` single named volume (`data`). Upstream Dockerfile pre-creates
  this directory with `node:node 0700` ownership and permissions, which Docker volume initialization
  preserves. Config (`openclaw.json`), SQLite state and agent databases, credentials, and workspace
  all reside inside this volume.
- Runtime options: `runtime.no_new_privileges: true` reproduces upstream's
  `security_opt: [no-new-privileges:true]` from `docker-compose.yml` (ADR 0045).
- Resources: `resources.min_ram: 2GB` based on documented pre-built container requirements
  (`docs/install/gcp.md`, `docs/install/digitalocean.md`).
- Health: `health.http.port: 18789`, `health.http.path: /healthz`, `timeout: 60s`.
  Upstream container probe checks `http://127.0.0.1:18789/healthz` (`src/docker-healthcheck.ts`),
  which returns HTTP 200 without authentication.
- Secrets: `model-key` → `ANTHROPIC_API_KEY`, `gateway-token` → `OPENCLAW_GATEWAY_TOKEN`.
- Skills: mounted read-only at `/home/node/.openclaw/skills` (`mode: mount`), the canonical
  managed/local skills directory (`Priority 4: Managed / local skills: <state-dir>/skills`,
  `src/skills/loading/skill-paths.ts`, `docs/tools/skills.md`).

Verification (offline, 2026-11-06):

- `go build`, `go vet`, `gofmt -l .` clean; `go test ./...` green except the three macOS-only
  `internal/deploy` failures listed in AGENTS.md (unrelated, untouched).
- `olym check -f testdata/openclaw/stack.yaml -recipes recipes` with a temporary sentinel `.env`
  → "Stack is valid." (exit 0).
- Rendered generation for `testdata/openclaw/stack.yaml` via `TestRecipeGoldens`:
  - `compose.yaml` declares service `openclaw` with image `ghcr.io/openclaw/openclaw:2026.11.1@sha256:dd...dd`,
    command `[node, openclaw.mjs, gateway, --bind, lan, --port, "18789"]`, mounts volume
    `openclaw_data` at `/home/node/.openclaw`, bind-mounts `/home/node/.openclaw/skills/deploy-checklist/SKILL.md`
    read-only, binds port `127.0.0.1:18789:18789`, sets `security_opt: [no-new-privileges:true]`,
    and delivers secrets via `env_file`.
  - `manifest.json` tracks skill file hash and tool hash, registers HTTP health check on port 18789
    path `/healthz`, and lists `gateway-token` and `model-key` secrets only (no secret values).
- `TestRecipeGoldens` in `internal/eval/recipes_golden_test.go` renders and passes against
  `testdata/openclaw/golden/`.

Known gaps:

- **MCP servers omitted (STOP rule).** OpenClaw configures MCP client servers under `mcp.servers`
  in `openclaw.json` (`src/config/zod-schema.mcp-server.ts`). Remote servers require
  `transport: "streamable-http"` or `"sse"`. No existing Olympus translation style emits
  `transport: "streamable-http"` or targets `mcp.servers` within `openclaw.json`. Per the project
  STOP rule, `supports.mcps` is omitted rather than worked around.
- **Chat channels omitted.** OpenClaw configures channels (Telegram, Discord, Slack) via structured
  keys in `openclaw.json` (`channels.telegram.allowFrom`, `channels.telegram.dmPolicy: "pairing"`).
  It does not configure channels or allow-lists via environment variables (`TELEGRAM_ALLOWED_USERS`
  does not exist in OpenClaw). Olympus's channel contract (ADR 0039, Rule A10) requires token and
  allow-list environment variables. Because OpenClaw cannot be configured with allow-lists through
  environment variables alone, `supports.channels` is omitted.
- **No live lab run.** No QEMU on the host, so `docker pull`, container execution, and live health
  probes were not exercised against a live daemon.
- **UNVERIFIED:** live model session execution against an active API key; browser Control UI
  pairing and WebSocket RPC operations.

ADR: none added. Reuses existing recipe format, ADR 0038 (skills mounting) and ADR 0045 (runtime options);
no format, durable behaviour or architecture changed.

- **Merge review, 2026-11-06:** verified the repo (not a fork), the `v2026.11.1` release, the GHCR image config (tini entrypoint that takes `command` as arguments, `User node`, `WorkingDir /app`) and the Dockerfile's `node:node 0700` state directories. Added `--allow-unconfigured` because the gateway refuses to start on a fresh volume without `gateway.mode=local`, which would have failed the first health check and rolled back. Added an `auth` volume for `/home/node/.config/openclaw`, which upstream persists separately. Regenerated the golden. Container start is still unverified (no lab).

## Pi and Oh My Pi (OLY-22)

**Status:** implemented offline; live-server verification not run.

Files:
- `recipes/pi/{recipe.yaml,Dockerfile,README.md}`
- `testdata/pi/stack.yaml`, `testdata/pi/skills/deploy-checklist/SKILL.md`, `testdata/pi/golden/`
- `recipes/oh-my-pi/{recipe.yaml,Dockerfile,README.md}`
- `testdata/oh-my-pi/stack.yaml`, `testdata/oh-my-pi/skills/deploy-checklist/SKILL.md`, `testdata/oh-my-pi/golden/`

Decisions:

- Tool identification & classification:
  - **Pi:** official non-fork repository `earendil-works/pi` (`gh api repos/earendil-works/pi` → `fork: false`), created by Mario Zechner (`badlogic`), published to npm as `@earendil-works/pi-coding-agent`.
  - **Oh My Pi:** official non-fork repository `can1357/oh-my-pi` (`gh api repos/can1357/oh-my-pi` → `fork: false`), created by Stencil Labs (`omp.sh`). Originating as a fork of Pi with native Rust IDE integration, Bun runtime and tree-sitter AST editing, it compiles to a standalone `omp` CLI and publishes prebuilt binaries. It is a separately runnable tool, not a configuration pack for Pi.
- Image & build mechanisms:
  - **Pi:** npm-built image via `build` (ADR 0035) on `node:22-bookworm-slim`, installing exact npm release `@earendil-works/pi-coding-agent@1.5.0` (`latest` on 2026-11-06) into `/opt/pi/bin` with normalized timestamps (`touch -h -d @0`). Verified with `npm view` that `1.5.0` has no `postinstall` script downloading external code.
  - **Oh My Pi:** binary-download build via `build` (ADR 0035) on `node:22-bookworm-slim`, downloading exact GitHub release `v18.17.0` standalone binary `omp-linux-<arch>` verified against upstream `SHA256SUMS.txt` (`a4ce6001…` for amd64, `a9423fd7…` for arm64) into `/usr/local/bin/omp`.
- State & volumes:
  - **Pi:** state directory is `/root/.pi/agent` (`getAgentDir()` in `packages/coding-agent/src/config.ts`), persisted via `home` volume at `/root`.
  - **Oh My Pi:** state directory is `/root/.omp/agent` (`getAgentDir()` / `docs/config-usage.md`), persisted via `home` volume at `/root`.
- Health checks:
  - **Pi:** `command: [pi, --version]` (60 s).
  - **Oh My Pi:** `command: [omp, --version]` (60 s).
- Resources & command:
  - Both declare `resources.min_ram: 4GB` (Olympus sizing choice for coding agents with AST parsing and quickjs workers; upstream docs do not specify a 4GB minimum) and run `command: [sleep, infinity]`.
- Secrets:
  - Both declare `model-key` → `ANTHROPIC_API_KEY` (primary provider key) and `github-token` → `GITHUB_TOKEN`.
- MCP configuration & writable delivery:
  - **Pi:** `/root/.pi/agent/mcp.json` under `mcpServers` in JSON format. Uses `style: hermes`, which maps stdio (`command`, `args`, `env` with `${VAR}`) and streamable HTTP (`url`, `headers` with `${VAR}`). Upstream explicitly rejects legacy `sse`. Marked `writable` and delivered via `pi-init` service into the volume because Pi rewrites `mcp.json` at runtime; edits are overwritten on apply.
  - **Oh My Pi:** `/root/.omp/agent/mcp.json` under `mcpServers` in JSON format. Uses `style: claude-code`, which emits the required `"type": "http"` for remote endpoints (`docs/mcp-config.md`). Marked `writable` and delivered via `oh-my-pi-init` service into the volume because Oh My Pi rewrites `mcp.json` at runtime; edits are overwritten on apply.
- Oh My Pi update settings:
  - Upstream Oh My Pi has no `OMP_SKIP_VERSION_CHECK` or `OMP_TELEMETRY` environment variables. Version check and auto-update are controlled by settings `startup.checkUpdate: false` and `update.auto: false` in `~/.omp/agent/config.yml` (`packages/coding-agent/src/modes/settings.ts:1283,1318`; `packages/coding-agent/src/main.ts:275-278` skips update checks when both false; path resolved in `packages/coding-agent/src/config/settings.ts:736-737` via `getAgentDir()` at `dirs.ts:641` and `MAIN_CONFIG_FILENAMES[0]` at `dirs.ts:32`).
  - Added recipe template `config.yml.tmpl` at `/root/.omp/agent/config.yml` seeding `startup.checkUpdate: false` and `update.auto: false` (A7), marked `writable` and delivered via `oh-my-pi-init` service into the volume.
- Skills:
  - **Pi:** mounted read-only at `/root/.pi/agent/skills` (`mode: mount`, ADR 0038).
  - **Oh My Pi:** mounted read-only at `/root/.omp/agent/skills` (`mode: mount`, ADR 0038).

Verification (offline, 2026-11-06, review update 2026-11-08):

- `go build -o /tmp/olym ./cmd/olym/`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green except the three pre-existing macOS-only `internal/deploy` ownership/ELF failures documented in AGENTS.md.
- `olym check -f testdata/pi/stack.yaml -recipes recipes` and `olym check -f testdata/oh-my-pi/stack.yaml -recipes recipes` with sentinel `.env` → "Stack is valid." (exit 0).
- `TestRecipeGoldens` in `internal/eval/recipes_golden_test.go` passes cleanly against `testdata/pi/golden/` and `testdata/oh-my-pi/golden/`. Upstream links pinned to `v1.5.0` and `v18.17.0`.

Known gaps:

- **Legacy SSE on Pi:** Pi rejects legacy SSE endpoints. Remote MCP servers must provide streamable HTTP endpoints.
- **No live lab run:** No QEMU on this host, so container startup, live health checks and model API calls were not exercised against a running Docker daemon.
- **UNVERIFIED:** live interactive session behavior and token consumption.

ADR: none added. Reuses existing recipe format, ADR 0035 (built images), ADR 0037/0047 (MCP styles) and ADR 0038 (mounted skills); no format, durable behaviour or architecture changed.

