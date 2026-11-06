# Recipe catalog

- **Status:** In progress: one new recipe (OpenBot) added and rendered by a golden test; not lab-verified.
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
