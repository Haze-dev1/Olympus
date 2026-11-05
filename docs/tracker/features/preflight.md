# Apply preflight

- **Status:** Done offline; lab verification not run (no QEMU on this host)
- **Updated:** 2026-11-05
- **Milestone:** M1

`plan` and `apply` stop before any server mutation when the server cannot run the
stack: unsupported OS, unsupported architecture or an image without the server's
architecture, less than 1 GiB free disk, or a declared port already in use. Less
than 5 GiB free disk and less memory than a recipe's `resources.min_ram` warn and
are covered by the normal apply confirmation. The server's own current
generation never conflicts with its published ports. See
[ADR 0044](../../ADR.md#0044-preflight-the-server-before-plan-and-apply),
[TRD 7.2](../../TRD.md#72-server-preparation) and
[edgecases/apply.md](../../edgecases/apply.md).

## Evidence, 2026-11-05

- `internal/deploy/preflight.go`: read-only checks built only from argument
  lists, run after SSH/privilege checks and before `ReadCurrent` or
  `PrepareServer`. OS via `/etc/os-release`, architecture via `uname -m`,
  memory via `/proc/meminfo`, disk via `df -kP /var/lib`, listeners via
  `ss -ltnp`, and the current generation's own published ports via
  `docker ps --filter label=com.docker.compose.project=olympus`. Remote process
  names and addresses are stripped of control characters before they reach a
  message. All problems are reported together as `preflight failed:` with exit 1.
- `internal/lock`: `update` and a lock-creating `check` record the resolved
  index's sorted, de-duplicated `os/architecture` pairs as `platforms` on each
  lock tool; the field is optional and validated. A digest-pinned reference
  fetches no index and records none.
- `internal/recipe`: `resources.min_ram` must parse as a size (`4GB`,
  `512MiB`); malformed values fail recipe validation instead of being ignored.
- `internal/cli`: `preflightRequest` maps tool ports (and their public flag),
  recipe minimum memory, images and locked platforms into the request; warnings
  print after the plan diff and before the apply prompt.
- Tests added: `TestPreflightPassesHealthyServer`,
  `TestPreflightRejectsUnsupportedOS`, `TestPreflightRejectsUnsupportedArchitecture`,
  `TestPreflightChecksLockedPlatforms`, `TestPreflightWarnsWithoutPlatformRecord`,
  `TestPreflightWarnsForLowMemory`, `TestPreflightDiskThresholds`,
  `TestPreflightPortConflicts` (including own-project and foreign-container
  ports), `TestPreflightSanitizesRemoteText`, `TestPreflightPublicPortConflict`,
  `TestPreflightSkipsUnreadableMeasurements`, `TestPreflightRequestUsesStackAndRecipes`,
  `TestParseSize`, `TestParseSizeRejectsBadValues`, recipe `min_ram` validation
  cases, lock platform canonicalisation/rejection cases, and resolver platform
  extraction with an ignored `unknown/unknown` attestation.
- `go build -o /tmp/olym ./cmd/olym/`, `go vet ./...` and `gofmt -l .` were
  clean. `go test ./internal/deploy/ ./internal/lock/ ./internal/recipe/` passed
  apart from the pre-existing failures listed below.
- Full `go test ./...` still fails only in the same tests that fail at
  `origin/development` (`5b2e052`), confirmed by stashing this change and rerunning:
  `internal/cli` `TestCatalogCommands`, `TestOfflineCommands`,
  `TestBuildReplacesDefaultOutputAfterStackChange`; `internal/deploy`
  `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`,
  `TestToolEntryReuseAndVerification`; `internal/eval` `TestCopiedSkillsPruned`;
  `internal/generation` `TestReplacementIsCompleteAndFailedStagingPreservesOutput`,
  `TestReplacementDoesNotFollowOldOutputSymlinks`. They are host-specific
  sandbox/build failures unrelated to this change and were not "fixed" here.
- **Lab verification: not run.** `lab/lab.sh` needs QEMU, `qemu-img` and
  `sha256sum`, which are absent on this arm64 macOS host, and the lab image is
  amd64 under emulation. The user chose to skip installing them, as for OLY-30.
  Therefore the real `ss`, `df`, `/proc/meminfo`, Docker label and prompt output
  on a live server are unverified; only the fake-transport tests above cover
  them.
- New tests exercise a new entry point, so there is no red-before revision to
  compare against. The lock field and resolver changes do run against existing
  tests, which were updated alongside them.

## Limits

- Public ports are checked, but public-port builds are still rejected by the
  evaluator, so that path cannot reach `apply` yet.
- Memory is compared per tool against `MemTotal`, not summed or against
  currently available memory.
- Missing `ss` or Docker degrades the port check to a warning; a hand-edited lock
  can claim platforms the image does not have, and the pull remains the real
  check.
- No registry, network or pull verification happens in the preflight.
- Security: the preflight adds no secret handling, prints no remote stderr and
  writes nothing on the server (A1, A2, A11). No dependency was added. No
  external scanner or independent review ran.
