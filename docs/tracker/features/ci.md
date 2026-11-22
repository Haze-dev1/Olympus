# CI

- **Status:** Done locally; first remote run happens on the pull request.
- **Updated:** 2026-11-22
- **Milestone:** none (quality gate)
- **Workflow:** [`.github/workflows/ci.yml`](../../../.github/workflows/ci.yml)
- **Link checker:** [`scripts/check-links.py`](../../../scripts/check-links.py)

## Coverage

`ci.yml` runs on `pull_request` to `development`/`main`, `push` to those branches, and `workflow_dispatch`. It has top-level `permissions: contents: read`, a per-workflow/per-ref concurrency group that cancels in-progress PR runs, and every action pinned to a full commit SHA (the `actions/checkout` v7.0.1 and `actions/setup-go` v7.0.0 pins Caddy uses). Checkout uses `persist-credentials: false`; Go uses `go-version-file: go.mod` with `check-latest: true`. No secrets are used and no environment is echoed.

Jobs (each with a status-check name and a timeout):

| Job | Checks |
| --- | --- |
| `test (ubuntu-latest)`, `test (macos-latest)` | `go build ./...`, `go vet ./...`, `go vet -tags lab ./...`, `go test ./...`, `go test -race -short ./...` |
| `format` | fails when `gofmt -l .` prints anything or `go mod tidy` changes `go.mod`/`go.sum` (`git diff --exit-code -- go.mod go.sum`) |
| `cross-build` (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) | `CGO_ENABLED=0 go build -o olym_<os>_<arch> ./cmd/olym` for each target |
| `release-bundle` | `scripts/build-release.sh 0.0.0-ci` into `DIST_DIR`, `sha256sum -c SHA256SUMS` there, then `TestReleaseBundle` with `OLYM_TEST_RELEASE_BUNDLE=1` |
| `docs` | `docs/tracker/progress.json` parses as JSON and `scripts/check-links.py` resolves relative Markdown paths and heading anchors in `docs/`, `examples/`, `recipes/` and top-level `*.md` |

`TestReleaseBundle` finds the activator next to the test executable (`bundledActivator` uses `os.Executable`), so the job compiles `./internal/deploy` with `go test -c -o "$DIST_DIR/deploy.test"` and injects both activator digests from `SHA256SUMS` with `-ldflags -X`, then runs that binary from `DIST_DIR`. This exercises the rule A12 checksum verification against real release artifacts instead of a fixture.

`.github/workflows/release.yml` gains one `go test ./...` step before the cross-compile step; the rest of that workflow is unchanged.

`scripts/check-links.py` uses only the Python 3 standard library, skips fenced code and inline code, ignores external URLs, decodes percent escapes and applies GitHub heading slug rules (lowercase, punctuation removed except `-`/`_`, spaces to `-`, `-1`/`-2` for duplicate headings). It exits non-zero with `file:line` diagnostics.

## Evidence, 2026-11-22

All commands were run on this branch in the worktree with Go 1.27.4 (`golang.org/x/crypto` requires Go 1.26+):

- `go build ./...`, `go vet ./...`, `go vet -tags lab ./...`: passed, no output.
- `go test ./...`: all packages passed.
- `go test -race -short ./...`: all packages passed.
- `gofmt -l .`: empty; `go mod tidy` followed by `git diff --exit-code -- go.mod go.sum`: passed.
- Cross-build loop for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` with `CGO_ENABLED=0`: all four binaries built.
- `./scripts/build-release.sh 0.0.0-ci` with `DIST_DIR=/tmp/opencode/olym-dist`, then `sha256sum -c SHA256SUMS` in that directory: all 12 artifacts OK.
- `go test -c` with the two `-ldflags -X` digests from `SHA256SUMS`, then `OLYM_TEST_RELEASE_BUNDLE=1 ./deploy.test -test.run '^TestReleaseBundle$' -test.v`: passed ("bundled activator accepted; modified activator rejected").
- `python3 -c 'import json; json.load(open("docs/tracker/progress.json"))'`: passed.
- `python3 scripts/check-links.py`: checked 70 Markdown files, all relative links and anchors resolve; no existing links needed fixing. A temporary-tree self-check confirmed it reports both a missing file and a missing anchor.
- `python3 -c "import yaml,sys;[yaml.safe_load(open(f)) for f in sys.argv[1:]]" .github/workflows/*.yml`: passed.

## Limits and decisions

- First remote run (PR #67) caught three real problems: `runner.temp` is not allowed in job-level `env`, so GitHub rejected `ci.yml` (now set in a step); macOS lint flagged goimports grouping in `internal/cli/terminal_darwin.go`, which Linux lint never compiles (fixed); and the repository had no dependency graph, so `dependency-review` could not run (Dependabot alerts enabled, which turns it on).
- macOS tests: `TMPDIR` on macOS runners is under the `/var` symlink, which build output refuses as an unsafe ancestor, so the job points `TMPDIR` at `$RUNNER_TEMP`. `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication` and `TestToolEntryReuseAndVerification` emulate a Linux server locally and assert root ownership; they are skipped on macOS only (the known failures noted in `recipe-catalog.md`) and still run on Ubuntu.

- The first remote run happens on the PR: only local commands were run here. The macOS matrix job cannot be executed on this Linux host; `setup-go` picks the latest available Go from `go.mod` on the runner.
- `actionlint` is not installed and was not installed for this work; workflow syntax was checked with PyYAML only.
- `lint.yml` (agy) runs golangci-lint v2.15.0 on Ubuntu and macOS, plain and with `--build-tags lab`, plus `govulncheck` and, on pull requests, `dependency-review` (fails on high severity). `.github/dependabot.yml` opens at most one grouped monthly PR each for Go modules and Actions, targeting `development`. Locally both lint runs report 0 issues; `govulncheck` was not available locally, so its first run is on the PR.
- Reviewed and merged by Claude, who reran the release-bundle and link-check steps locally (TestReleaseBundle pass; 71 files, all links resolve).
- No ADR: CI wiring and a docs-check script change no architecture, file format, durable product behaviour or tradeoff; ADR 0048 and A12 are exercised, not amended.
- Security: no secrets, no `pull_request_target`, pinned action SHAs, no persisted checkout credentials, read-only top-level permissions, and release-bundle verification enforces rule A12. The link checker reads local files only and makes no network calls.
