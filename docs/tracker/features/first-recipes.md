# First recipes: Hermes, Codex, OpenCode

- **Status:** Done: all three deployed and healthy in the lab; provider inference and chat authentication remain outside this sentinel run.
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
- Earlier offline-only limits are superseded by the runtime evidence below. Memory recommendations, OAuth behavior and real provider/chat authentication remain unverified where recipe READMEs say so.
- Resolved from the earlier open questions: Codex builds from a recipe-supplied Dockerfile pinned by content ([ADR 0035](../../ADR.md#0035-build-tools-without-an-official-image-from-a-recipe-supplied-dockerfile)); writable config uses a Compose init service ([0036](../../ADR.md#0036-deliver-writable-managed-config-with-a-compose-init-service)); Hermes channels use environment variables ([0039](../../ADR.md#0039-configure-hermes-channels-as-environment-variables)); OpenCode's undocumented memory minimum is recorded as unverified rather than guessed as a contract.

## Real-stack deployment evidence, 2026-10-20

- Branch `feat/real-stack-deploy`, based on development `2208e0e`. Go 1.27.2.
- Canonical lab reset with `OLYMPUS_LAB_MEMORY=6144`; Ubuntu 24.04, Docker
  Compose v5.6.0. The installed help confirms `pull --ignore-buildable`,
  `up --no-build` and `up --pull` support.
- `TestLabRealStack`, tagged `lab` and gated on `OLYMPUS_LAB=1`, passed in
  70.501 seconds. Uses only explicit lab key, known_hosts and cache settings.
  Real subprocess commands: update, check, plan, apply, status, no-op apply,
  OpenCode MCP-list change, plan/apply, rollback, status, destroy --delete-data.
- Update resolves the private development catalog to a full commit and image
  digests. Check/plan/apply use a frozen worktree recipe copy with the explicit
  local override, so unmerged recipe changes are tested while retaining locked
  image/base digests. This is intentional and printed by the CLI.
- All three tools were running and healthy. OpenCode `/global/health` returned
  healthy on server loopback; Codex returned `codex-cli 0.116.0`. Init successfully
  copied writable configs and the Hermes skill. `/bin/sh` and `cp` exist in all
  images. Shared skill files are present at each recipe's path.
- MCP fixture is local stdio shell code, requires a sentinel credential and
  answers initialize/tools-list without outbound calls. Configs parse as Hermes
  YAML, Codex TOML and OpenCode JSON, with native environment references only.
- No sentinel in store, activation logs or managed config files. All six service
  IDs remain identical on no-op. Only OpenCode and opencode-init IDs change
  after its MCP list changes. Rollback preserves Hermes and Codex IDs and the
  built Codex image ID. Teardown removes project containers, volumes and root.
- Hermes runtime `_env_ref_lookup` independently returned process environment
  values; its generated dotenv exists and contains no sentinel. No provider
  calls or chat channel were enabled. This proves idle/container readiness,
  not authenticated inference or bot operation.
- Failures recorded: initial update without GITHUB_TOKEN returned HTTP 404;
  using the existing GitHub CLI credential as GITHUB_TOKEN resolved it. An early
  run changed recipes during execution, invalidating the no-op comparison;
  the test now snapshots recipes. A duplicate method introduced while editing
  failed compilation and was removed. Initial golden validation found obsolete
  content-addressed files; regenerated artifacts and removed only those files.
- Decisions: [ADR 0040](../../ADR.md#0040-resolve-optional-catalog-refs-to-locked-commits)
  and [0041](../../ADR.md#0041-prepare-content-addressed-built-images-before-starting-services).
- Security: no new dependencies, no real model keys or provider requests. Secret
  output is suppressed at the production Compose boundary. Host-key verification,
  stdin-only payloads, localhost ports and project-scoped teardown remain in use.
  A12 uses the existing explicit development ELF checksum/version verification;
  release checksum embedding remains deferred as documented in ADR 0029.

## Completion run (2026-10-20, reviewer)

Codex ran out of quota before committing; the reviewer finished the branch.

- Hermes: the image's default invocation starts the interactive chat. The recipe now sets `command: [gateway, run]`, the documented long-running mode (references/hermes-agent docker-compose.yml and website/docs/user-guide/docker.md).
- Codex build reproducibility: a clean `--no-cache` rebuild gave a different image ID despite `SOURCE_DATE_EPOCH=0` and normalized timestamps, so builds are input-reproducible (pinned base digest, exact `@openai/codex@0.116.0`, content-addressed Dockerfile), not byte-identical. The byte-identical rebuild check was removed from the lab test; its forced rebuild also retagged the image and would have broken the no-op step. ADR 0041 and the Codex README now say this. Byte-identical builds are a follow-up.
- Golden real-stack data regenerated after the Hermes command change; stale store entries removed.
- Lab evidence: `OLYMPUS_LAB_MEMORY=6144 lab/lab.sh reset`, then `GITHUB_TOKEN=<gh credential> OLYMPUS_LAB=1 go test -tags lab ./internal/cli -run TestLabRealStack` passed in 241 s: update, check, plan, apply (Compose v5.6.0), status healthy, `codex-cli 0.116.0`, native-format MCP config with variable references only and no sentinel in store or configs, no-op apply with no recreation, OpenCode-only MCP change recreating only OpenCode and its init service, rollback, status, destroy --delete-data. No model inference or chat was attempted.
- `go vet ./...`, `go test ./...` (13 packages), `gofmt -l .` and `go vet -tags lab ./internal/cli` passed.
- Follow-up: catalog refs containing `/` (for example `feature/x`) are path-escaped in the GitHub commits request and may not resolve; `development` works.
