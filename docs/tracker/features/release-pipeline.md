# Release pipeline

- **Status:** Done offline (CI and release script ready; tag/release not pushed)
- **Updated:** 2026-11-06
- **Milestone:** M1

Cross-compiles the Olympus CLI (`darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`) and the Linux activator (`linux/amd64`, `linux/arm64`), publishes `SHA256SUMS`, and validates the Linux activator against embedded release checksums before executing it on the target server (rule A12). Release workflow runs via GitHub Actions (`.github/workflows/release.yml`) and `scripts/build-release.sh`. See [ADR 0048](../../ADR.md#0048-cross-compile-release-binaries-and-verify-activator-checksum-against-release-build) and [TRD section 13](../../TRD.md#13-build-and-release).

## Evidence, 2026-11-06

- `scripts/build-release.sh`: Builds Linux activator binaries (`amd64`, `arm64`), calculates their SHA-256 digests, and cross-compiles the CLI binaries (`darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`) embedding the activator digests via `-ldflags` (`ReleaseChecksumLinuxAmd64`, `ReleaseChecksumLinuxArm64`). Packages `.tar.gz` release archives and generates `SHA256SUMS`.
- `.github/workflows/release.yml`: Tag-triggered release workflow on `v*` using official GitHub actions (`actions/checkout@v4`, `actions/setup-go@v5`) and `gh release create`.
- `internal/deploy/deploy.go`: `UploadActivator` calculates the local binary's SHA-256 digest and verifies it matches `c.ExpectedReleaseChecksum(arch)`. Mismatches fail closed with `deploy: activator checksum does not match release build` before any server upload or execution.
- `internal/deploy/commands.go`: `VerifyActivator` checks the server's binary matches the expected release checksum for read-only commands (`status`, `rollback`, `gc`, `destroy`).
- `internal/cli/commands.go`: When `--activator <path>` is supplied in development mode without embedded checksums, prints a visible warning to stderr (`Warning: using custom activator; skipping release checksum verification`) while permitting local testing.
- `internal/deploy/release_test.go`: Tests matching and mismatched release checksum validation on `UploadActivator` and `VerifyActivator`, and package-level `ExpectedChecksum` lookup.
- `docs/install.md` and `docs/release.md`: Complete installation, verification, and tag-release instructions.
- Verification commands:
  - `gofmt -l .`: clean
  - `go vet ./...`: clean
  - `go build -o /tmp/olym ./cmd/olym/`: clean
  - `TMPDIR=/private/tmp/olym-oc go test ./internal/deploy/...`: passed new unit tests
  - `scripts/build-release.sh 0.1.0-test`: successfully produced all 4 CLI binaries, 2 activator binaries, 6 `.tar.gz` archives, and `SHA256SUMS`.
- **Not verified:**
  - Pushing git tags or creating live GitHub releases (per explicit instructions: do NOT create tags, releases, or signing keys, and do not push tags).
  - QEMU lab VM tests (no QEMU on host).
