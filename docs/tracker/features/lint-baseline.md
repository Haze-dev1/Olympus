# Lint baseline

- **Status:** Done. Baseline cleared (0 issues on `golangci-lint run ./...`).
- **Milestone:** none (quality gate)
- **Config:** [`.golangci.yml`](../../../.golangci.yml), adapted from Caddy's

`golangci-lint` v2.15.0 runs with Caddy's linter set, minus the web-server-only linters (`bodyclose`, `canonicalheader`, `promlinter`, `sloglint`, `sqlclosecheck`, `zerologlint`). Unlike Caddy, `gosec` G204 stays on, because Olympus builds remote commands and rule A2 needs every case reviewed. The entire repository now passes `golangci-lint run ./...` with 0 issues; the `--new-from-merge-base=origin/development` flag in `AGENTS.md` can be dropped.

## History of findings

Initial baseline on `development` at `3e103ef` (2026-11-21) had 55 findings. Reduced to 36 in OLY-64 (security findings resolved), to 17 in round-3 cleanup (packages outside concurrent branches), and to 0 in the final cleanup round.

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

### Resolved in final cleanup (17 findings cleared)

All 17 findings in the concurrent packages (`internal/activate`, `internal/deploy`, `internal/eval`, `internal/remote`) resolved mechanically with zero behavior changes:

- `internal/activate` (4):
  - `activate.go:122`: `slices.Contains` replaces manual loop checking for empty argument (modernize).
  - `activate.go:143`: `Deadline time.Time` uses `json:"deadline,omitzero"` tag (modernize).
  - `compose.go:179`: `splitLines` ranges over `strings.SplitSeq` (modernize).
  - `gc.go:162`: `storeRefs` preallocates `entries` slice with `make([]string, 0, len(matches))` (prealloc).
- `internal/deploy` (8):
  - `deploy.go:340`: De Morgan's law simplification on ELF machine architecture check (staticcheck QF1001).
  - `deploy.go:484`: ranges over `strings.SplitSeq` for systemd property parsing (modernize).
  - `preflight.go:94`: removed redundant initial `arch := ""` assignment (wastedassign).
  - `preflight.go:113, 314`: replaced unexported `contains` loop with `slices.Contains` and removed helper (modernize).
  - `preflight.go:182, 226, 227, 250`: ranges over `strings.SplitSeq` for sockets, owned ports, and meminfo lines (modernize).
- `internal/eval` (2):
  - `eval.go:192`: `maps.Copy(environment, r.Env)` replaces manual copy loop (modernize).
  - `eval.go:480`: `slices.Contains(r.Writable, seccompDst)` replaces manual loop (modernize).
- `internal/remote` (3):
  - `osrelease.go:30`: ranges over `strings.SplitSeq` for `/etc/os-release` parsing (modernize).
  - `remote.go:168`: `errors.AsType[*ssh.ExitError](err)` in `HasPasswordlessSudo` replaces `errors.As` (modernize).
  - `remote.go:270`: ranges over `strings.SplitSeq` in `appendKnownHost` (modernize).

## Verification

- `go build -o /tmp/olym ./cmd/olym/` passes.
- `go vet ./...` reports 0 issues.
- `go test ./...` and `go test -race -short ./...` pass on Linux amd64 with Go 1.27.4.
- `gofmt -l .` prints nothing.
- `golangci-lint run ./...` reports 0 issues on the entire tree.
- `golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ./...` reports 0 issues.
- `TestLabDeploy` passes on fresh Ubuntu 24.04 lab VM (ports 2272/8149) in 136.4 s.
- `TestLabM1` passes on fresh Ubuntu 24.04 lab VM (ports 2272/8149) in 137.1 s, verifying check, plan, apply, status, autonomous rollback, gc and destroy cycles.
- No ADR: mechanical cleanup of code quality findings with no change in architecture or behavior.

### Orchestrator review of OLY-64 (2026-11-22)

- Simplified `extract`: removed a second tree-cap check and an `n > limit` branch that the earlier header checks and the existing `n != hdr.Size` check already make unreachable. The stream is still bounded by `io.LimitReader(tr, hdr.Size)`.
- Red-before confirmed for `TestRedirectNeverSendsTokenToDifferentHost`: against the `development` version of `catalog.go` it fails with `got "Bearer sentinel-token-12345"`.
- Behaviour note: the catalog token now follows a redirect only to the same scheme and host. GitHub's tarball redirect to `codeload.github.com` carries its own short-lived token in the URL, so it does not need ours.
- Full offline set passes (build, vet, test, race, gofmt, lint 0 issues). `TestLabM1` passes on a fresh Ubuntu 24.04 lab VM in 120.2 s, covering activate, confirm and rollback with the new `clearConfirm` handling.

### Orchestrator review of the round-3 cleanup (2026-11-22)

Reviewed the diff: changes are mechanical. `hasDotDot` (A3 path confinement) is equivalent to the old loop. `status --json` and `gc` now fail on a write error instead of ignoring it. Narrowed the new `errname` exclusion to `internal/schema/`. After merging `development` (OLY-63 and OLY-25), the full offline set passes, the whole tree has 17 findings, and `TestLabM1` passes on a fresh Ubuntu 24.04 lab VM in 125.6 s.

### Orchestrator review of the final cleanup (2026-11-22)

Reviewed all 17 changes as behaviour-neutral. The De Morgan rewrite of the activator ELF check is logically identical, and the `state.json` `deadline` tag change (`omitzero`) only drops a zero value that the activator, its sole reader, decodes the same way. After merging `development`, the full offline set passes and `golangci-lint run --build-tags lab ./...` reports 0 issues on the whole tree. On fresh Ubuntu 24.04 lab VMs `TestLabDeploy` passes in 137.9 s and `TestLabM1` in 136.1 s. `AGENTS.md` now requires whole-tree lint.
