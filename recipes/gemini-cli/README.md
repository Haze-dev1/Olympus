# Gemini CLI recipe

Installs Google's [Gemini CLI](https://github.com/google-gemini/gemini-cli) as an
Olympus coding-agent tool, with MCP servers in `~/.gemini/settings.json` and
shared skills in `~/.gemini/skills/`.

Pinned version: **`@google/gemini-cli@0.67.0`** (the `latest` dist-tag on
2026-11-06). Verified before pinning:

```
$ npm view @google/gemini-cli@0.67.0 version engines dist.tarball
version = '0.67.0'
engines = { node: '>=20' }
dist.tarball = 'https://registry.npmjs.org/@google/gemini-cli/-/gemini-cli-0.67.0.tgz'

$ npm view @google/gemini-cli dist-tags
{
  false: '0.42.0-nightly.20260512.g11a9edc80',
  latest: '0.67.0',
  nightly: '0.67.0-nightly.20261104.g89e9d25d6',
  preview: '0.68.0-preview.0',
  'staging-tmp': '0.67.0'
}
```

## Facts and sources

All facts below were checked on 2026-11-06 against the Gemini CLI repository at
tag `v0.67.0`, the same release the recipe installs. Paths are relative to
`https://github.com/google-gemini/gemini-cli/blob/v0.67.0/`.

| Fact | Value | Source |
|---|---|---|
| Install command | `npm install -g @google/gemini-cli` | `docs/get-started/installation.mdx` |
| Node requirement | Node.js 20.0.0+ | `docs/get-started/installation.mdx` |
| Version flag | `gemini --version` prints the CLI version | `docs/cli/sandbox.md` (install check); `docs/reference/configuration.md` (`--version` flag) |
| Memory guidance | 4 GB RAM for casual use | `docs/get-started/installation.mdx` |
| Model credential | `GEMINI_API_KEY` (env var, no settings-file edit) | `docs/get-started/authentication.mdx` |
| User settings file | `~/.gemini/settings.json` | `docs/cli/settings.md` |
| MCP servers key | top-level `mcpServers` object in `settings.json` | `docs/tools/mcp-server.md` |
| MCP stdio entry | `command`, `args`, `env` | `docs/tools/mcp-server.md` |
| MCP env expansion | `$VAR_NAME` or `${VAR_NAME}` inside `env` values | `docs/tools/mcp-server.md` |
| MCP remote entry | `url` is an SSE endpoint; `httpUrl` is a streamable-HTTP endpoint; `headers` applies to both | `docs/tools/mcp-server.md` |
| Settings string expansion | every string value in `settings.json` resolves `$VAR`, `${VAR}`, `${VAR:-default}` | `docs/reference/configuration.md` |
| MCP child env sanitization | base env is redacted for `GEMINI_API_KEY`, `*TOKEN*`, `*SECRET*`, `*PASSWORD*`, `*KEY*`, `*AUTH*`, `*CREDENTIAL*` unless the variable is listed explicitly in that server's `env` | `docs/tools/mcp-server.md` |
| User skills directory | `~/.gemini/skills/` (an `~/.agents/skills/` alias also exists) | `docs/cli/skills.md` |
| Auto-update default | `general.enableAutoUpdate` defaults to `true` and updates the installed package | `docs/reference/configuration.md` |
| `/settings` persistence | `/settings` edits are written back to `settings.json` | `docs/cli/settings.md` |

## Image

Built, not pulled. Gemini CLI's only official images are the `--sandbox`
container (`ghcr.io/google/gemini-cli:latest` by default) and a versioned
Artifact Registry example, both documented for isolating tool execution rather
than for running the CLI host-side; the documented install is an exact npm
release. So the recipe uses the existing `build` mechanism (ADR 0035) on
`node:22-bookworm-slim` and installs the pinned package:

- `ARG GEMINI_CLI_VERSION=0.67.0`, installed to `/opt/gemini-cli` and put on
  `PATH`; the npm cache is removed and file timestamps normalized so a rebuild
  does not depend on the build clock (ADR 0041).
- Read-only `BASE_IMAGE` is supplied by the generation from the digest-pinned
  recipe image, so the lock controls the base (A7).

## Layout

| Path | How | Why |
|---|---|---|
| `/root` | volume `gemini_home` | Sessions, chat history and auth state survive a rebuild. |
| `/root/.gemini/settings.json` | template + managed `mcpServers`, copied into the volume by the `gemini-init` service (`writable`) | The CLI rewrites this file through `/settings`, so a read-only bind would break that command. |
| `/root/.gemini/skills/<name>/` | `supports.skills`, `mode: mount` | Documented user-skill location; read-only (ADR 0038). |

The template seeds `{"general":{"enableAutoUpdate":false}}` because the CLI
otherwise updates its own npm package inside the container, which defeats the
version pin (A7). `mcpServers` is then merged into that document by the
evaluator. Olympus owns this file: a re-apply re-renders it from the recipe and
the stack, so `/settings` edits made inside the container do not survive an
apply. Other files under `/root` (sessions, history) do.

## MCP servers

`supports.mcps` points at `/root/.gemini/settings.json` with `format: json`,
`style: gemini-cli` and `key: mcpServers`. The `gemini-cli` style emits the
stdio and remote entry shapes documented by Gemini CLI
(`docs/reference/configuration.md` and `docs/tools/mcp-server.md` at tag `v0.67.0`):

- **stdio entries:** `command`, `args`, `env` with `${VAR}` references.
- **Remote streamable-HTTP entries (`transport: http`, the default):** `httpUrl`
  and `headers` ([ADR 0047](../../docs/ADR.md#0047-carry-an-explicit-remote-transport-for-mcp-servers)).
- **Remote SSE entries (`transport: sse`):** `url` and `headers`.

```json
{"mcpServers":{
  "github":{"args":["-y","@modelcontextprotocol/server-github"],"command":"npx","env":{"GITHUB_TOKEN":"${GITHUB_TOKEN}"}},
  "docs":{"headers":{"Authorization":"Bearer ${GITHUB_TOKEN}"},"url":"https://mcp.example.test/sse"},
  "search":{"headers":{"Authorization":"Bearer ${GITHUB_TOKEN}"},"httpUrl":"https://mcp.example.test/mcp"}
}}
```

`${GITHUB_TOKEN}` is a reference in the file; Gemini CLI expands it when it
spawns the server, and the value is only in that server's environment because
the recipe lists it in `env`. Without that explicit listing the CLI would
redact `GITHUB_TOKEN` (it matches `*TOKEN*`) from the inherited environment.

`testdata/gemini-cli/stack.yaml` covers stdio, SSE (`transport: sse`) and
streamable HTTP (`transport: http`) remotes, and its golden files verify the
rendered JSON.

## Secrets

- `model-key` → `GEMINI_API_KEY`: the documented credential variable. The CLI
  reads it directly; it is not passed to MCP child processes.
- `github-token` → `GITHUB_TOKEN`: available for MCP entries that reference it.
- Both arrive through the activators' `0600` environment file, never through
  the recipe, stack file, `plan` output or logs (A1, A5).

## Health and runtime

- Health: `gemini --version` (60 s), the documented install check.
- Command: `sleep infinity`, so the CLI is exec'd into the container like the
  other coding-agent recipes.
- No ports are published and nothing listens, so A4 is not triggered.
- `resources.min_ram: 4GB`, matching the docs' casual-use memory guidance.

## Not verified in this change

- **UNVERIFIED — image build and CLI run.** No QEMU/lab VM was available, so
  `docker build`, `gemini --version` inside the image, first-run auth with
  `GEMINI_API_KEY`, and `/settings` persistence into the volume were not
  executed.
- **UNVERIFIED — npm install egress.** Only the npm registry was checked; any
  additional host the install may need was not observed.
- **UNVERIFIED — first-run settings write.** Whether the CLI rewrites
  `settings.json` before `/settings` is used could not be observed. The
  `writable` delivery makes it safe either way.
- **UNVERIFIED — MCP connection inside the container.** The rendered entry
  matches the documented shape, but no live `npx` server was connected to a
  running Gemini CLI.
- **UNVERIFIED — interactive approval.** Whether the CLI prompts before
  trusting a user-level MCP server on first run was not observed.
