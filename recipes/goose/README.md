# Goose recipe

Sources checked **2026-11-06** against Block / Agentic AI Foundation's Goose
documentation and repository releases; the image and health output were
re-checked on **2026-11-22**.

## Checked facts

- **Container images.** The docs describe building from source
  (`documentation/docs/docker/Dockerfile`), but an official image does exist at
  the pin: `https://ghcr.io/v2/aaif-goose/goose/manifests/1.57.0` returns an OCI
  index with `linux/amd64`
  `sha256:e617564e88479da5f279a75be38b2a130fb555b3f08652d0bea1084444c2da8d`
  and `linux/arm64`
  `sha256:5634003869fcd0dcfc1093e6ff89501440227ad6eefc153cb828d01025f5e212`
  (checked 2026-11-22). The older claim that no maintained image existed after
  `1.30.0` was wrong. The recipe deliberately keeps its checksum-verified binary
  build: switching to the release image would change the runtime user,
  entrypoint and config/skills paths, and the build keeps inputs pinned by
  digest (A7). Source: <https://goose-docs.ai/docs/tutorials/docker/>.
- **Install.** The documented Linux CLI install downloads precompiled binary
  releases from GitHub Releases (<https://goose-docs.ai/docs/getting-started/installation/>).
  The Dockerfile downloads the exact-version static binary tarball
  (`goose-<arch>-unknown-linux-musl.tar.gz`) from GitHub Releases for `amd64`
  (`x86_64`) and `arm64` (`aarch64`), verifies the SHA-256 digest
  (`6c3a05e30e1a425dc06a410dbe24b3f3697c9721b1ea2340434ddda6b64267ae` for `amd64`,
  `00a32522f0421ed72fc06f36bfc44c8e51cffefb4daf22a81bb84ccd29d1a1ce` for `arm64`)
  against GitHub Releases assets, and installs it to `/usr/local/bin/goose`.
- **Version pin.** `v1.57.0`, published 2026-10-28 on GitHub Releases.
  Verified with:
  `curl -sL https://api.github.com/repos/aaif-goose/goose/releases/tags/v1.57.0 | jq -r .tag_name` → `v1.57.0`.
  Source: <https://github.com/aaif-goose/goose/releases/tag/v1.57.0>.
- **Resources.** Upstream documents no minimum; `resources.min_ram: 4GB` is an
  Olympus sizing choice for local agent execution, not an upstream requirement.
- **Health.** `goose --version` exits 0 without credentials or network access.
  The pinned release binary prints ` 1.57.0` (a leading space, no `goose`
  prefix), not `goose 1.57.0`; the recipe's command check only needs the zero
  exit status. There is no HTTP endpoint for the CLI.
- **Model key and provider.** Goose supports multiple providers (OpenAI, Anthropic,
  Databricks, Ollama, etc.). In Goose, `GOOSE_PROVIDER` and `GOOSE_MODEL` have no
  default and "must be configured" (see `documentation/docs/guides/environment-variables.md`
  at tag `v1.57.0`, <https://github.com/aaif-goose/goose/blob/v1.57.0/documentation/docs/guides/environment-variables.md>).
  Because the recipe's `model-key` secret is delivered as `OPENAI_API_KEY`, the
  recipe sets `env: GOOSE_PROVIDER: openai` in `recipe.yaml`. Users configure
  the model choice by setting `GOOSE_MODEL` (e.g., `gpt-4o`) in the environment
  or by running `goose configure` interactively inside the container
  (`documentation/docs/guides/environment-variables.md` at tag `v1.57.0` and
  <https://goose-docs.ai/docs/getting-started/providers#configure-provider-and-model>),
  which persists configuration to `~/.config/goose/config.yaml` in the `home` volume.
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
- **Full stack apply on a server.** `docs/tracker/features/security-review.md`
  records an arm64 Docker image build and `goose --version` health output on a
  daemon on 2026-11-19, so the earlier claim that nothing ran against a live
  daemon was too strong. No full VM apply of this recipe has been recorded.

## Reproducibility

Builds are reproducible in their inputs (base image digest, release version
pin, Dockerfile content). The Dockerfile normalizes installed binary timestamps
with `touch -d @0` so builds do not depend on the build clock. Compose builds
the image on the server using content-addressed local tags
(`olympus/goose:sha256-…`).
