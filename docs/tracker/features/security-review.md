# Security review

- **Status:** In progress — F1 live verification and F4 remain open; F2 fixed offline (OLY-62); 2026-11-22 review R5 resolved (OLY-66)
- **Updated:** 2026-11-22

OLY-33 records the pre-release source review in [the report](../../security/review-2026-11-06.md). No product fixes were made. ADR reviewed; no decision is introduced by a findings-only review. `govulncheck ./...` found no reachable vulnerabilities and one non-reachable module advisory. Build and vet passed; gofmt was clean; the full suite failed on the three known macOS cases `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, and `TestToolEntryReuseAndVerification`. No QEMU lab, live release or server testing was performed. Relative Markdown links and anchors in `docs/` resolved, and tracker JSON parsed successfully.

## F3 fixed (2026-11-08, OLY-61)

Finding F3 (development clients bypass release digest matching, rule A12) is fixed in `internal/deploy`: `checkReleaseChecksum` now refuses when a development build has no release checksum for the server architecture unless `deploy.Client.UnsafeDevActivator` is set, and `UploadActivator` and `VerifyActivator` both call it before uploading or executing anything. The CLI exposes this as `-unsafe-dev-activator`, wired through `manageGeneration` and `serverCommand`. Release builds keep their previous behaviour: a missing embedded checksum fails closed, and a checksum mismatch rejects the activator. ADR 0049 amends ADR 0048.

Verification evidence:

- Red-before: in a detached tree at `fd8743d` with the new tests and only the `UnsafeDevActivator` field added to `Client` (no guard), `go test ./internal/deploy/ -run '^(TestDevelopmentActivatorRequiresExplicitOptIn|TestUploadActivatorReleaseChecksumVerification)$' -v` failed both refusal subtests — `development build uploaded an activator with no release checksum without -unsafe-dev-activator` and `development build ran an activator with no release checksum without -unsafe-dev-activator`. Against `fd8743d` without that field shim the committed test file does not compile (`unknown field UnsafeDevActivator`), because the opt-in API does not exist there.
- Independent red proof (t12): the exact two refusal subtests were copied onto `fd8743d` in a scratch tree, omitting the opt-in-success subtest so the file compiles against the old `Client`; both failed because the old `UploadActivator` and `VerifyActivator` accepted an unsigned development binary, with no unrelated failure. Green was already established by the full suite on `7c497ca`.
- After the fix the same tests pass, including upload and verify with the opt-in. The earlier `empty release checksum allows dev build` case was replaced with a case that expects the opt-in error and then succeeds with the flag. `TestMissingReleaseChecksumFailsClosed` also asserts a release build still refuses a missing checksum with the opt-in set.
- After merging `origin/development` at `e6c495d`, build, `go vet ./...` and `gofmt -l .` (empty) passed. `go test ./...` passed every package except the three known macOS deploy cases `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication` (`activator must be a Linux ELF binary`) and `TestToolEntryReuseAndVerification` (`mv failed`). The tagged lab tests were not run (no QEMU on this host); they still compile, now pass `-unsafe-dev-activator`, and `internal/deploy/lab_test.go` sets the client field directly.
- Independent review of `7c497ca` (t12): build, vet and gofmt passed; the full suite showed only the same three known macOS failures with baseline-identical reasons. Its first run failed for an environment reason, not code: `TMPDIR` pointed at the scratch worktree instead of a separate temp directory; rerunning with a separate temp directory reproduced only the known failures. The review also found the README opt-in gap, the `docs/install.md` overstatement and the ADR help claim, all corrected here.

Remaining limits: a compromised root server can still replace the binary after the check; a development build that embeds release checksums uses normal verification without the flag. F1 live sender checks, F2 build-input pinning and F4 independent artifact authentication remain open.

## F2 fixed offline (2026-11-19, OLY-62)

Finding F2 (built-image recipes did not pin transitive npm or apt inputs, rule A7) is fixed in the recipe format and the four affected recipes ([ADR 0051](../../ADR.md#0051-pin-built-image-inputs-with-committed-lockfiles-and-a-fetch-stage)). `build.files` delivers a committed `package.json` and `package-lock.json` into the build context for `codex`, `claude-code` and `gemini-cli`, which install with `npm ci --omit=dev`; Goose moves apt and curl into a `fetch` stage and ships only its checksum-verified binary. Build files feed the image tag, so ADR 0041 reuse cannot serve an image built from an older lockfile.

Verification evidence:

- `TestBuildFilesFeedImageTag` fails with the hash change reverted (`image tag olympus/codex:sha256-4ddcab8723f4 did not change with the lockfile`) and passes with it. New `TestRecipeValidation` cases reject a traversing build file and duplicate or Dockerfile-shadowing names.
- Each lockfile is v3, and every `resolved` entry has an `integrity` hash (codex 7, claude-code 9, gemini-cli 12 packages, including linux x64/arm64 optional packages). `npm ci --omit=dev --ignore-scripts` succeeded against each committed pair on macOS, left the lockfile unchanged and produced the `codex`, `claude` and `gemini` binaries the health checks call.
- Goldens regenerated; `TestRealStackGolden` and `TestRecipeGoldens` pass. Build, vet and gofmt pass. With `TMPDIR` on a path without a symlinked ancestor, `go test ./...` fails only the three known macOS deploy cases (`TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, `TestToolEntryReuseAndVerification`). The default macOS `TMPDIR` under `/var` (a symlink) adds six `unsafe build directory ancestor` failures, identically on untouched `origin/development`.

- 2026-11-19 image build on Docker 29.9.2 (`linux/arm64`, base `node:22-bookworm-slim`, `BASE_IMAGE` passed as the generation does): all four recipe Dockerfiles built, and the health commands printed `codex-cli 0.116.0`, `2.1.318 (Claude Code)`, `0.67.0` (Gemini CLI) and `1.57.0` (Goose). The Goose final image has no `curl`, confirming apt stays in the fetch stage.

Remaining limits: `linux/amd64` builds and a full lab apply were not run (no QEMU); native install scripts still run at build time. OLY-66 later removed those scripts (R5 below), so this limit applies only to the 2026-11-19 evidence.

## R5 resolved (2026-11-22, OLY-66)

Finding R5 of [the 2026-11-22 review](../../security/review-2026-11-22.md) — enabled npm lifecycle scripts could add unpinned build inputs — is resolved for `claude-code`, `codex` and `gemini-cli`. All three Dockerfiles install with `npm ci --omit=dev --ignore-scripts`. The install scripts at the pinned tarball versions were read as data: Codex has none; Gemini CLI's keytar prebuild copy and node-pty cleanup are optional helpers the CLI does not need; Claude Code's postinstall only links the platform binary shipped inside an integrity-pinned optional-dependency tarball. Claude Code invokes that single audited script explicitly by path because its `bin` remains an error stub otherwise. [ADR 0056](../../ADR.md#0056-keep-npm-lifecycle-scripts-out-of-built-images) amends 0051.

`TestRecipeNpmInstallScriptsDisabled` failed on all three Dockerfiles before the edit and passes after. Fresh single-tool lab applies with sentinel secrets passed for all three recipes: status `running healthy`, 0 restarts, pinned `claude --version` 2.1.318 / `codex --version` 0.116.0 / `gemini --version` 0.67.0, 0600 per-tool secrets, no published ports and no sentinel in CLI output, store, logs or container logs. Full validation including race and branch-scoped lint passed. Fix-verification result: **fixed** (root cause, no new surface, all instances, legitimate path intact). Dependency advisory scans named in the review remain a separate pre-release task.

## Security data-flow note (2026-11-08)

Added `docs/security/data-flow.md` based on `.agents/skills/review-app-security/reference/data-flow-note.md` at baseline `61a1e57`. Traced sensitive model keys, bot/GitHub tokens, SSH credentials, chat content, trust boundaries, and rules A1–A12 directly to source code citations.

Review evidence and limits:
- Draft revisions: Lead review rejected two initial drafts for inaccurate recipe table citations, unverified upstream runtime claims asserted as facts, conflation of file-based vs env-mapped secret delivery, anchor syntax errors (`#Lx-Ly`), and over-broad development opt-in assertions. All were corrected: recipe citations were scoped to an exact Hermes example (`recipes/hermes/recipe.yaml:18-26`), upstream chat/runtime behaviors were explicitly marked `UNVERIFIED`, file-based and env-file secret delivery were distinguished (`internal/activate/activate.go:786, 793, 830`), and development opt-in was qualified strictly to development builds lacking embedded release checksums (`internal/deploy/deploy.go:283-292`).
- Scope and threat model limits: Docker daemon and server root have full inspection access to container environments and volume mounts; Olympus does not isolate secrets from privileged host accounts. Upstream runtime logging and chat storage remain UNVERIFIED; the core does not establish a container-log collection path. Upstream model provider and chat platform data retention remain unverified.
- ADR review: Reviewed `docs/ADR.md`. No architecture decision or file format change was introduced by this documentation; no ADR was added.
- Validation: `python3 /private/tmp/olym-doc-links.py` reported 0 JSON errors and 0 doc link/anchor errors. `go build -o /tmp/olym-agy-flow-bin ./cmd/olym/`, `go vet ./...`, and `gofmt -l .` (empty) passed cleanly. `go test ./...` reproduced only the three known macOS deploy failures (`TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, and `TestToolEntryReuseAndVerification`).

- 2026-11-08 lead review: independent build/vet/formatting passed; full tests reproduced only the three baseline-identical macOS deploy failures. Corrected remaining source-line citations and removed unsupported network, logging and blanket secrecy claims in the detached review tree. Docs links/anchors and tracker JSON pass. No new ADR warranted. The documented hook-execution gap is tracked as OLY-63; no bundled recipe currently uses a hook.


## Pre-release delta review (2026-11-22)

Reviewed `fd8743d..187f33f`. The [findings report](../../security/review-2026-11-22.md) records four confirmed findings and two plausible ones. Outstanding work includes the remediation, dependency and secret scans, and the earlier upstream chat and release artifact checks.

Build, vet, full and race-short Go tests, formatting and branch-scoped lint passed. The first attempts hit read-only caches and blocked mock listener sockets. Retries used writable caches and loopback access. Govulncheck and gitleaks were unavailable. No lab run. Reviewed ADRs 0048–0052. The review changes documentation only, so no ADR was added.

## Release review of development against main (2026-11-22)

A code-only review of everything `development` carries ahead of `main` (#19 to #60) is in [release-review-2026-11-22.md](../../security/release-review-2026-11-22.md). It has four serious findings (OpenClaw can't read its root-owned 0600 skill files; Pi accepts SSE; a secret change restarts a tool before its `-init` step, including on rollback; the port preflight reports `[::1]` and other 127.x listeners as blocking 127.0.0.1) and nine minor ones. Fixes are tracked per finding in that file. No ADR: a findings-only review.
