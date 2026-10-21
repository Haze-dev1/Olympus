# Deploy

- Status: Done for the requested package and server command scope.
- Milestone: M1.
- Scope: client package and server activate/confirm commands, without an apply command.

## Implementation

`internal/deploy` reuses `remote` argument-list SSH commands and Docker install steps. OS validation precedes mutation; non-root preparation requires `sudo -n`. The server root and transfer directories are root-owned 0700. Generation uploads validate the canonical manifest identity and declared tool file hashes, transfer a regular-file-only tar on stdin, verify all files remotely, and rename only after verification. Existing entries are reused only if they verify. Failed stages are removed when the connection remains usable; a disconnected upload can leave only an unpublished `.incoming` directory.

Activator uploads validate Linux ELF architecture, stage mode 0755, compare remote SHA256 to the local bytes before running `version`, require an exact version match, publish and update the stable symlink. Activation rechecks checksum and version. Payloads match the activator JSON format and travel exclusively on stdin to its PayloadPath at mode 0600. Secret-bearing remote output and errors are discarded. Named file secrets are checked against manifest declarations; the caller still supplies the recipe-derived environment mapping because the manifest does not describe that mapping.

Activation uses a unique detached systemd service, retains the exit result with `RemainAfterExit`, polls awaiting-confirm, rejects a pre-existing nonce and confirms through a fresh SSH connection. It observes exit codes 0, 1, 3 and 4, including failures before state is written. Successful and failed services are cleaned after their result is observed. Failed reconnection permits automatic rollback while the original connection continues monitoring.

`internal/cli/server.go` implements root-only activate/confirm and a version query. The dispatch change is one early branch. It uses the real Compose runner and Checker, with a temporary empty HealthSource because this branch's generation manifest does not yet contain health data. Container output is suppressed before the activator can log an error. `remote.Config.HostKeyAlgorithms` permits the lab to select its pinned Ed25519 key without weakening host-key verification.

## Decisions and remaining work

[ADR 0029](../../ADR.md#0029-verify-deployment-uploads-and-retain-the-activation-result) records verified transfers and retained transient service results. No changes to internal/plan, internal/eval or internal/generation, and no new dependencies.

Follow up when manifest health lands: replace emptyHealth with a manifest-backed source for activation and rollback. Release checksums remain local-file verification as explicitly scoped. No apply command, release downloading, fingerprints, secret-change recreation or container-user chown was added. The existing fixed secrets payload path requires serialization by the eventual apply caller; concurrent payload uploads are not protected by the activator's lock. Disconnected uploads and completed transient units can require later cleanup. Existing corrupted immutable entries are refused rather than overwritten.

## Verification

- Go 1.27.2.
- Package unit tests pass. Fake commands cover unsupported OS, missing sudo, installed/missing Docker, successful upload and verified reuse, checksum failure with no published entry or staging residue, manifest disagreement, binary checksum/version rejection, publication after verification, secret stdin/mode/error safety, undeclared secrets, fresh connection, cleanup and exit mappings.
- Final `go build -o /tmp/olym ./cmd/olym/`, `go vet ./...` and `go test ./...` passed; `gofmt -l .` printed nothing. Tests used an explicit /tmp Go cache. The initial sandbox attempt hit the read-only default cache and localhost listener restrictions; subsequent full tests ran outside the sandbox with the explicit cache.
- Lab reset uses `/home/haze/Projects/Olympus/lab/lab.sh`; key, known_hosts and XDG cache are explicit, with no developer SSH/cache fallback. The first attempt correctly refused host-key negotiation against a different key type; selecting the pinned Ed25519 algorithm resolved it. The next attempt installed Docker and activated/confirmed a generation but exposed systemd unloading the successful unit before result polling. Added result retention and cleanup. The fresh rerun passed in 104.389 seconds: Docker/Compose installed, pinned BusyBox generation uploaded, activated and confirmed through a new SSH connection, current resolved to it, and a second activation with a simulated reconnect failure rolled back autonomously after its confirmation timeout. The client observed exit 3 and current still resolved to the first generation. The canonical lab VM was shut down after verification.
- Every relative Markdown link and anchor in docs resolved; tracker JSON parsed successfully. `git diff --check` passed. The lab-tag test skips without OLYMPUS_LAB=1 as required. No real VPS, arm64 or macOS run, no manifest health readiness checks and no external security scan.

## Security

A1: secret payload only on stdin, remote output discarded; server Compose errors omit container output. A2: every remote operation uses argv through remote.Run. A5: 0600 payload and inherited per-tool secret enforcement; environment mapping and container-user chown remain caller/activator limitations. A6: both SSH connections verify host keys. A11: deployment state stays under /var/lib/olympus and Compose stays in project olympus; the explicitly requested Docker bootstrap additionally modifies system apt/Docker configuration. A12: local file checksum and version checked before activation; embedded release trust is deferred as requested. No external security scan was run.

## Merge into development (2026-10-20)

- End-to-end lab test re-run by the reviewer on a reset lab VM: `OLYMPUS_LAB=1 go test -tags lab ./internal/deploy -run TestLabDeploy` passed in 101 s. Docker and Compose were installed through sudo; a generation was uploaded, activated and confirmed, and `current` was verified; a second, unconfirmed activation rolled back by itself.
- Merged `development` (plan and manifest health, ADR 0028). The server commands now load health checks from `store/gen-<id>/manifest.json`, replacing the empty health source.
- `RollbackFailedError` now maps to exit code 5 on the server and in `deploy.Activate`, documented in TRD 10.1.

## Per-tool entries, 2026-10-20

ADR 0030 changes upload layout: validate every declared config hash, upload and verify each `store/<tool hash>/files/...` entry independently, reuse only verified entries, then publish the generation's Compose and manifest. A regression test uploads two generations sharing one tool entry and refuses reuse after tampering. No live lab evidence for this contract yet; the full M1 cycle remains pending.

## M1 integration, 2026-10-20

Apply and rollback now share the detached start and fresh-SSH confirmation protocol. Each deployment has a unique payload and request id; current-id validation runs under the server lock. Shared connection setup, repeated sudo detection, status verification and safe exit-status propagation are tested. Full CLI lab cycles include fresh Docker bootstrap, verified tool-entry reuse, no-op reapply, concurrent apply and cleanup. See core-plan-apply.md for exact results, failures and release trust limits.

Final M1 evidence: source-complete `TestLabM1` passed in 77.902 s on a reset Ubuntu VM, 2026-10-20. Build, vet, full unit suite and formatting passed; see [core-plan-apply.md](core-plan-apply.md) for failures, exact commands and release limits.
