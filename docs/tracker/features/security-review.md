# Security review

- **Status:** In progress — findings and live verification remain open
- **Updated:** 2026-11-06

OLY-33 records the pre-release source review in [the report](../../security/review-2026-11-06.md). No product fixes were made. ADR reviewed; no decision is introduced by a findings-only review. `govulncheck ./...` found no reachable vulnerabilities and one non-reachable module advisory. Build and vet passed; gofmt was clean; the full suite failed on the three known macOS cases `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, and `TestToolEntryReuseAndVerification`. No QEMU lab, live release or server testing was performed. Relative Markdown links and anchors in `docs/` resolved, and tracker JSON parsed successfully.
