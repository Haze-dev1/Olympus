# Lint baseline

- **Status:** In progress. Config added; existing findings not fixed
- **Milestone:** none (quality gate)
- **Config:** [`.golangci.yml`](../../../.golangci.yml), adapted from Caddy's

`golangci-lint` v2.15.0 runs with Caddy's linter set, minus the web-server-only linters (`bodyclose`, `canonicalheader`, `promlinter`, `sloglint`, `sqlclosecheck`, `zerologlint`). Unlike Caddy, `gosec` G204 stays on, because Olympus builds remote commands and rule A2 needs every case reviewed. `AGENTS.md` runs it with `--new-from-merge-base=origin/development`, so only changed lines are checked until this backlog is cleared. When it is, drop that flag so the whole tree is checked.

## Findings on `development` at `3e103ef` (2026-11-21), updated in OLY-64 and round 3 lint cleanup

17 findings remain (down from 36): `golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ./...`. With `--new-from-merge-base=origin/development` the tree reports 0.

All 17 remaining findings are in packages actively edited by concurrent branches (`internal/activate`: 4, `internal/deploy`: 8, `internal/eval`: 2, `internal/remote`: 3). All findings outside those packages are resolved.

### Review first (security-relevant) — ALL RESOLVED (OLY-64)

- `internal/catalog/catalog.go:337` G110: **Fixed**. Added per-file (64 MiB) and total tree (256 MiB) caps in `extract` using `io.LimitReader` and early header inspection; tested with crafted tarballs exceeding caps.
- `internal/catalog/catalog.go:210` G119: **Fixed**. Enforced that `Authorization` bearer token is deleted and never forwarded to a different host or scheme on redirect; tested with dual-host `httptest` servers.
- `internal/activate/activate.go:283` and `:373`: **Fixed**. Errors from `clearConfirm` are checked immediately after confirmation and before publishing/pointing current, rolling back safely so an uncleared confirmation cannot be accepted by a later activation; tested with a failing fake.
- `internal/remote/remote.go:193` G704 and `internal/activate/commands.go:37`, `:118`, `compose.go:55`, `:108` G204: **Reviewed**. Verified against Rule A2 (commands built from discrete argument lists, no shell interpolation; `remote.go` dials local ssh-agent UNIX socket only); added `//nolint:gosec` with explicit reasons.

### Resolved in round 3 cleanup

- errcheck: `cli/commands.go:224` (handled error from `json.Encoder.Encode`), `cli/commands.go:256` (handled error from `out.Write` in `gc`).
- unused: `cli/plan.go:31` (`readPlan` moved to `internal/cli/plan_test.go` where it serves as a test helper for fake reader plan comparisons).
- prealloc: `schema/schema.go:59` (preallocates slice capacity in `Variables` from submatch count).
- errname: `schema.Diagnostic` retained and documented in `.golangci.yml` exclusions (represents a positioned parse/validation diagnostic rather than an ordinary Go error, avoids churn across `eval`, `cli`, and `stack`).
- goimports: `cmd/olym/main.go`, `internal/cli/terminal_linux.go` formatted.
- modernize: 12 suggestions resolved in in-scope packages outside concurrent branches:
  - `cli/cli.go:175`: replaced string concatenation in diagnostic loop with `strings.Builder`.
  - `cli/server.go:162, 167, 172`: simplified `errors.As` type checks with `errors.AsType`.
  - `catalog/catalog.go:86`: `strings.SplitSeq`.
  - `lock/registry.go:126, 245`: `strings.SplitSeq`.
  - `recipe/recipe.go:144, 299`: `strings.SplitSeq`; `:268`: `slices.Contains`.
  - `stack/stack.go:56, 59`: `maps.Copy`.

### Remaining in concurrent packages (17 findings)

- `internal/activate` (4):
  - `activate.go:120`: slicescontains
  - `activate.go:141`: omitzero
  - `compose.go:170`: stringsseq
  - `gc.go:162`: prealloc
- `internal/deploy` (8):
  - `deploy.go:484`: stringsseq
  - `preflight.go:182, 226, 227, 250`: stringsseq
  - `preflight.go:315`: slicescontains
  - `deploy.go:340`: staticcheck QF1001 (De Morgan simplification)
  - `preflight.go:94`: wastedassign
- `internal/eval` (2):
  - `eval.go:192`: mapsloop
  - `eval.go:480`: slicescontains
- `internal/remote` (3):
  - `osrelease.go:30`: stringsseq
  - `remote.go:168`: errorsastype
  - `remote.go:270`: stringsseq

## Verification

- `go test -race -short ./...` passes on Linux amd64 with Go 1.27.4.
- `golangci-lint run --new-from-merge-base=origin/development ./...` reports 0 issues.
- `golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ./...` reports exactly 17 issues, all in `activate`, `deploy`, `eval`, or `remote`.
- `TestLabM1` passes on fresh Ubuntu 24.04 lab VM (ports 2272/8149) in 93.7 s, verifying check, plan, apply, status, autonomous rollback, gc and destroy cycles.

### Orchestrator review of OLY-64 (2026-11-22)

- Simplified `extract`: removed a second tree-cap check and an `n > limit` branch that the earlier header checks and the existing `n != hdr.Size` check already make unreachable. The stream is still bounded by `io.LimitReader(tr, hdr.Size)`.
- Red-before confirmed for `TestRedirectNeverSendsTokenToDifferentHost`: against the `development` version of `catalog.go` it fails with `got "Bearer sentinel-token-12345"`.
- Behaviour note: the catalog token now follows a redirect only to the same scheme and host. GitHub's tarball redirect to `codeload.github.com` carries its own short-lived token in the URL, so it does not need ours.
- Full offline set passes (build, vet, test, race, gofmt, lint 0 issues). `TestLabM1` passes on a fresh Ubuntu 24.04 lab VM in 120.2 s, covering activate, confirm and rollback with the new `clearConfirm` handling.

### Orchestrator review of the round-3 cleanup (2026-11-22)

Reviewed the diff: changes are mechanical. `hasDotDot` (A3 path confinement) is equivalent to the old loop. `status --json` and `gc` now fail on a write error instead of ignoring it. Narrowed the new `errname` exclusion to `internal/schema/`. After merging `development` (OLY-63 and OLY-25), the full offline set passes, the whole tree has 17 findings, and `TestLabM1` passes on a fresh Ubuntu 24.04 lab VM in 125.6 s.
