# First recipes: Hermes, Codex, OpenCode

- **Status:** In progress: recipes build offline and produce the expected Compose and configs; not deployed on a server yet.
- **Milestone:** M2
- **Plan:** [plans/v1.md](../../plans/v1.md)

Three different kinds of tool, to test the recipe format before writing more.

## Acceptance

- All three run from recipes on one server.
- Every path, port and image in them is checked against upstream docs.

## Evidence

- 2026-10-19 — upstream facts for all three tools gathered into [Sources/recipe-research.md](../../Sources/recipe-research.md) (one section per tool, plus the TRD section 16 answers). Covers image and tags, ports and health, data and config paths and formats, MCP, skills, channels, authentication, runtime config writes, resources and licence, with a link and checked date per source. Nothing was installed or executed; unverified items are marked UNVERIFIED.
- 2026-10-20 — `recipes/hermes`, `recipes/codex` and `recipes/opencode` written with a README per folder naming sources and unverified items. Images: `nousresearch/hermes-agent:latest` (concrete version unverified), `node:22-bookworm-slim` as Codex's pinned build base, `ghcr.io/anomalyco/opencode:1.0.196`.
- 2026-10-20 — `internal/eval/real_test.go` builds `testdata/real/stack.yaml` with all three tools, one shared GitHub MCP server carrying a secret, one skill and Telegram on Hermes, with pinned digests supplied as a lock would. Golden output is in `testdata/golden-real/`. Asserted: Hermes YAML, Codex TOML and OpenCode JSON shapes; a secret appears only as `${VAR}` / `{env:VAR}` / a Codex variable name; Codex `build.context` is the content-addressed store path, `dockerfile` is `Dockerfile` and `BASE_IMAGE` is the pinned base digest; only OpenCode publishes a port and it binds `127.0.0.1:4096:4096`; every tool with a writable file gets a `<tool>-init` service and `depends_on: service_completed_successfully`.
- 2026-10-20 — `go build`, `go vet ./...`, `go test ./...` and `gofmt -l .` pass locally. No Docker was run and the lab VM was not used, per the round's coordination.
- Limits:
  - Not deployed: no one-tool or three-tool run on a clean server. The activator (not edited this round) pulls images and does not run `docker compose build`, so Codex cannot deploy until activation builds.
  - Hermes reading `ANTHROPIC_API_KEY` and `TELEGRAM_BOT_TOKEN` from the process environment rather than only `/opt/data/.env` is unverified; the activator may need to write them at deploy time.
  - Codex's `sleep infinity` command, the npm `CODEX_VERSION=latest` default and the Node base choice are unverified.
  - OpenCode's 2 GB minimum and the presence of `/bin/sh` and `cp` in its image for the init service are unverified.
- Resolved from the earlier open questions: Codex builds from a recipe-supplied Dockerfile pinned by content ([ADR 0035](../../ADR.md#0035-build-tools-without-an-official-image-from-a-recipe-supplied-dockerfile)); writable config uses a Compose init service ([0036](../../ADR.md#0036-deliver-writable-managed-config-with-a-compose-init-service)); Hermes channels use environment variables ([0039](../../ADR.md#0039-configure-hermes-channels-as-environment-variables)); OpenCode's undocumented memory minimum is recorded as unverified rather than guessed as a contract.
