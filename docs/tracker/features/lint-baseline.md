# Lint baseline

- **Status:** In progress. Config added; existing findings not fixed
- **Milestone:** none (quality gate)
- **Config:** [`.golangci.yml`](../../../.golangci.yml), adapted from Caddy's

`golangci-lint` v2.15.0 runs with Caddy's linter set, minus the web-server-only linters (`bodyclose`, `canonicalheader`, `promlinter`, `sloglint`, `sqlclosecheck`, `zerologlint`). Unlike Caddy, `gosec` G204 stays on, because Olympus builds remote commands and rule A2 needs every case reviewed. `AGENTS.md` runs it with `--new-from-merge-base=origin/development`, so only changed lines are checked until this backlog is cleared. When it is, drop that flag so the whole tree is checked.

## Findings on `development` at `3e103ef` (2026-11-21), updated in OLY-64

36 findings remain (down from 55): `golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ./...`. With `--new-from-merge-base=origin/development` the tree reports 0.

### Review first (security-relevant) — ALL RESOLVED (OLY-64)

- `internal/catalog/catalog.go:337` G110: **Fixed**. Added per-file (64 MiB) and total tree (256 MiB) caps in `extract` using `io.LimitReader` and early header inspection; tested with crafted tarballs exceeding caps.
- `internal/catalog/catalog.go:210` G119: **Fixed**. Enforced that `Authorization` bearer token is deleted and never forwarded to a different host or scheme on redirect; tested with dual-host `httptest` servers.
- `internal/activate/activate.go:283` and `:373`: **Fixed**. Errors from `clearConfirm` are checked immediately after confirmation and before publishing/pointing current, rolling back safely so an uncleared confirmation cannot be accepted by a later activation; tested with a failing fake.
- `internal/remote/remote.go:193` G704 and `internal/activate/commands.go:37`, `:118`, `compose.go:55`, `:108` G204: **Reviewed**. Verified against Rule A2 (commands built from discrete argument lists, no shell interpolation; `remote.go` dials local ssh-agent UNIX socket only); added `//nolint:gosec` with explicit reasons.

### Mechanical

- errcheck: `cli/commands.go:224`, `:256` (`activate/compose.go:215`, `activate/gc.go:172`, `catalog/catalog.go:429-440` fixed in OLY-64)
- staticcheck: `deploy/deploy.go:340` De Morgan simplification (`catalog/catalog.go:318` tar.TypeRegA fixed in OLY-64)
- unused: `cli/plan.go:31 readPlan`; wastedassign: `deploy/preflight.go:94`; prealloc: `activate/gc.go:162`, `schema/schema.go:59`
- errname: `schema.Diagnostic` (rename it or add an exclusion with the reason)
- goimports: `cmd/olym/main.go`, `internal/cli/terminal_linux.go`
- modernize: 26 suggestions

## Verification

- `go test -race -short ./...` passes on Linux amd64 with Go 1.27.4.
- `golangci-lint run --enable-only errcheck,gosec,staticcheck ./internal/catalog/... ./internal/activate/... ./internal/remote/...` reports 0 issues.
- `golangci-lint run --new-from-merge-base=origin/development ./...` reports 0 issues.
- Security-relevant items fixed with red-before unit tests in `internal/catalog/catalog_test.go` and `internal/activate/activate_test.go`.

### Orchestrator review of OLY-64 (2026-11-22)

- Simplified `extract`: removed a second tree-cap check and an `n > limit` branch that the earlier header checks and the existing `n != hdr.Size` check already make unreachable. The stream is still bounded by `io.LimitReader(tr, hdr.Size)`.
- Red-before confirmed for `TestRedirectNeverSendsTokenToDifferentHost`: against the `development` version of `catalog.go` it fails with `got "Bearer sentinel-token-12345"`.
- Behaviour note: the catalog token now follows a redirect only to the same scheme and host. GitHub's tarball redirect to `codeload.github.com` carries its own short-lived token in the URL, so it does not need ours.
- Full offline set passes (build, vet, test, race, gofmt, lint 0 issues). `TestLabM1` passes on a fresh Ubuntu 24.04 lab VM in 120.2 s, covering activate, confirm and rollback with the new `clearConfirm` handling.
