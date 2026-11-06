# Goose recipe

Sources checked **2026-11-06** against Block / Agentic AI Foundation's Goose
documentation and repository releases.

## Checked facts

- **No maintained container image.** Goose documentation describes Docker usage as
  building from source (`docker-compose build` with a repository Dockerfile in
  `documentation/docs/docker/Dockerfile`) rather than providing a maintained
  container image (<https://goose-docs.ai/docs/tutorials/docker/>). Legacy image
  tags on `ghcr.io/block/goose` stopped at `1.9.0` (January 2025) and
  `ghcr.io/aaif-goose/goose` stopped at `1.30.0` (April 2026). The recipe
  therefore uses `build` with a recipe-supplied Dockerfile on
  `node:22-bookworm-slim`, a registry base image that the lock pins by digest.
- **Install.** The documented Linux CLI install downloads precompiled binary
  releases from GitHub Releases (<https://goose-docs.ai/docs/getting-started/installation/>).
  The Dockerfile downloads the exact-version static binary tarball
  (`goose-<arch>-unknown-linux-musl.tar.gz`) from GitHub Releases for `amd64`
  (`x86_64`) and `arm64` (`aarch64`) and installs it to `/usr/local/bin/goose`.
- **Version pin.** `v1.57.0`, published 2026-10-28 on GitHub Releases.
  Verified with:
  `curl -sL https://api.github.com/repos/aaif-goose/goose/releases/tags/v1.57.0 | jq -r .tag_name` → `v1.57.0`.
  Source: <https://github.com/aaif-goose/goose/releases/tag/v1.57.0>.
- **Resources.** 4 GB+ RAM is standard for local agent execution. The recipe
  sets `resources.min_ram: 4GB`.
- **Health.** `goose --version` prints the version (`goose 1.57.0`) and exits 0
  without requiring credentials or network access. There is no HTTP endpoint for
  the CLI, so the recipe declares a command health check.
- **Model key.** Goose supports multiple providers (OpenAI, Anthropic, Databricks,
  Ollama, etc.). Provider credentials and model selection are read from
  environment variables: `GOOSE_PROVIDER` (default `openai`), `GOOSE_MODEL`
  (e.g., `gpt-4o`), and provider API keys such as `OPENAI_API_KEY`
  (<https://goose-docs.ai/docs/getting-started/installation/>). Declared as the
  `model-key` secret delivered as `OPENAI_API_KEY`.
- **Skills.** Global skills are discovered from `~/.agents/skills/<name>/SKILL.md`
  (`crates/goose/src/skills/mod.rs` at `v1.57.0` sets `global_skills_dir()` to
  `~/.agents/skills`). The recipe mounts shared skills read-only at
  `/root/.agents/skills` (`mode: mount`).
- **Configuration path.** Goose stores configuration at
  `~/.config/goose/config.yaml` on Linux/macOS via the XDG strategy
  (`crates/goose/src/config/paths.rs` and `base.rs` at `v1.57.0`).
- **Channels.** The local CLI has no chat-channel configuration, so the recipe
  declares none.

## Format gap: MCP servers (extensions)

In Goose, MCP servers are called "extensions" and configured under the
top-level `extensions` key in `~/.config/goose/config.yaml`
(<https://goose-docs.ai/docs/quickstart/> and
`crates/goose/src/config/extensions.rs`).

However, the Olympus recipe format cannot express Goose's extension schema:

- Goose requires `type: stdio` (a tagged enum with `#[serde(tag = "type")]`)
  for standard I/O extensions, `cmd: string` for the executable, and `args: []string`.
- For remote HTTP/SSE extensions, Goose requires `type: streamable_http` and
  `uri: string`.
- Olympus `translateMCP` only supports `hermes` (emits `command`, `args`, `env`,
  `url`, `headers` without `type`), `codex` (emits `command`, `args`, `env_vars`,
  `url`), and `opencode` (emits `type: local`, `command: [...]`, `type: remote`).
  None of these styles emit `cmd`, `uri`, or `type: stdio`.
- Because Goose rejects entries without the `type` tag and `cmd` field as
  malformed ("Skipping malformed extension config entry"), Olympus cannot emit
  compatible extension configuration without a new MCP translation style in Go.

Following the project STOP rule, `supports.mcps` is omitted rather than worked
around. This gap is recorded in `docs/tracker/features/recipe-catalog.md` and in
the PR.

## UNVERIFIED

- **Interactive model request.** No live model inference or session run was
  executed against an active provider API key.
- **Image build and health check on a real server.** No QEMU lab VM was
  available for this change, so `docker build` and container execution were
  not run against a live daemon. Offline generation rendering was verified.

## Reproducibility

Builds are reproducible in their inputs (base image digest, release version
pin, Dockerfile content). The Dockerfile normalizes installed binary timestamps
with `touch -d @0` so builds do not depend on the build clock. Compose builds
the image on the server using content-addressed local tags
(`olympus/goose:sha256-…`).
