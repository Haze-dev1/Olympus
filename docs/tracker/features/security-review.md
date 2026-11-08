# Security review

- **Status:** In progress — F1 live verification and F2/F4 remain open
- **Updated:** 2026-11-08

OLY-33 records the pre-release source review in [the report](../../security/review-2026-11-06.md). No product fixes were made. ADR reviewed; no decision is introduced by a findings-only review. `govulncheck ./...` found no reachable vulnerabilities and one non-reachable module advisory. Build and vet passed; gofmt was clean; the full suite failed on the three known macOS cases `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, and `TestToolEntryReuseAndVerification`. No QEMU lab, live release or server testing was performed. Relative Markdown links and anchors in `docs/` resolved, and tracker JSON parsed successfully.

## F3 fixed (2026-11-08, OLY-61)

Finding F3 (development clients bypass release digest matching, rule A12) is fixed in `internal/deploy`: `checkReleaseChecksum` now refuses when a development build has no release checksum for the server architecture unless `deploy.Client.UnsafeDevActivator` is set, and `UploadActivator` and `VerifyActivator` both call it before uploading or executing anything. The CLI exposes this as `-unsafe-dev-activator`, wired through `manageGeneration` and `serverCommand`. Release builds keep their previous behaviour: a missing embedded checksum fails closed, and a checksum mismatch rejects the activator. ADR 0049 amends ADR 0048.

Verification evidence:

- Red-before: `go test ./internal/deploy/ -run '^TestDevelopmentActivatorRequiresExplicitOptIn$' -v` failed both new subtests on the unmodified code — `development build uploaded an activator with no release checksum without -unsafe-dev-activator` and `development build ran an activator with no release checksum without -unsafe-dev-activator`.
- After the fix the same test passes, including upload and verify with the opt-in. The earlier `empty release checksum allows dev build` case was replaced with a case that expects the opt-in error and then succeeds with the flag.
- Build, vet and gofmt are recorded in the PR. The tagged lab tests were not run (no QEMU on this host); they now pass `-unsafe-dev-activator`, and `internal/deploy/lab_test.go` sets the client field directly.

Remaining limits: a compromised root server can still replace the binary after the check; a development build that embeds release checksums uses normal verification without the flag. F1 live sender checks, F2 build-input pinning and F4 independent artifact authentication remain open.
