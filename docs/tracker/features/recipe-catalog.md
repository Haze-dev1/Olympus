# Recipe catalog

- **Status:** In progress
- **Milestone:** M5
- **Plan:** [plans/v1.md](../../plans/v1.md)

Recipes for the remaining major tools: Claude Code, Gemini CLI, OpenClaw, pi, oh-my-pi, Goose, Aider, Grok bot, OpenSpaces.

## Acceptance

- Each recipe passes the [recipe checklist](../../specs/recipe.md#checklist-for-a-new-recipe).

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

Known gaps:

- **Streamable-HTTP remote MCP servers do not work.** `hermes` can emit `url`,
  which Gemini CLI reads as an **SSE** endpoint; streamable HTTP needs the
  separate `httpUrl` field, and no MCP style emits it. SSE remotes and stdio
  servers are unaffected. Closing this needs a new MCP style (Go change), so it
  is reported here and in the PR rather than worked around. This is narrower
  than the Claude Code gap: Gemini CLI accepts `url` without a `type` field.
- **No committed automated test.** Adding one means editing `internal/eval`
  and `testdata/real`/`testdata/golden-real`, which this change is scoped out
  of. `testdata/gemini-cli/golden/` is ready for a follow-up
  `TestGeminiCLIRecipe`.
- **No live lab run.** No QEMU on the host, so `docker build`,
  `gemini --version`, first-run auth with `GEMINI_API_KEY`, `/settings`
  persistence, and a live MCP connection were not exercised.
- **UNVERIFIED:** whether the npm install needs egress beyond the registry;
  whether the CLI writes `settings.json` on first interactive run before
  `/settings` is used (the `writable` delivery makes that safe either way);
  whether a user-level `mcpServers` entry still prompts for approval on first
  run. All three are recorded in `recipes/gemini-cli/README.md`.

ADR: none added. The recipe reuses ADRs 0035 (built images), 0036 (writable
init delivery), 0037 (MCP translation styles) and 0038 (skills mounting); no
format, durable behaviour or architecture changed.

