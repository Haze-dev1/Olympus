# Lint baseline

- **Status:** In progress. Config added; existing findings not fixed
- **Milestone:** none (quality gate)
- **Config:** [`.golangci.yml`](../../../.golangci.yml), adapted from Caddy's

`golangci-lint` v2.15.0 runs with Caddy's linter set, minus the web-server-only linters (`bodyclose`, `canonicalheader`, `promlinter`, `sloglint`, `sqlclosecheck`, `zerologlint`). Unlike Caddy, `gosec` G204 stays on, because Olympus builds remote commands and rule A2 needs every case reviewed. `AGENTS.md` runs it with `--new-from-merge-base=origin/development`, so only changed lines are checked until this backlog is cleared. When it is, drop that flag so the whole tree is checked.

## Findings on `development` at `3e103ef` (2026-11-21)

55 findings: `golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ./...`. With `--new-from-merge-base=origin/development` the same tree reports 0.

### Review first (security-relevant)

- `internal/catalog/catalog.go:337` G110: possible decompression bomb. Cap the bytes extracted from a recipe archive.
- `internal/catalog/catalog.go:210` G119: the redirect policy re-adds sensitive headers. Check whether a token can follow a redirect to another host.
- `internal/activate/activate.go:283` and `:373`: the error from `clearConfirm` is ignored, so a failed clear could leave a stale confirmation marker.
- `internal/remote/remote.go:193` G704 and `internal/activate/commands.go:37`, `:118`, `compose.go:55`, `:107` G204: confirm each command is built from an argument list (A2), then fix it or add a `//nolint:gosec` with the reason.

### Mechanical

- errcheck: `activate/compose.go:215`, `activate/gc.go:172`, `catalog/catalog.go:429`, `:436`, `:438`, `:440`, `cli/commands.go:221`, `:253`
- staticcheck: `catalog/catalog.go:318` uses deprecated `tar.TypeRegA`; `deploy/deploy.go:340` De Morgan simplification
- unused: `cli/plan.go:31 readPlan`; wastedassign: `deploy/preflight.go:94`; prealloc: `activate/gc.go:162`, `schema/schema.go:59`
- errname: `schema.Diagnostic` (rename it or add an exclusion with the reason)
- goimports: `cmd/olym/main.go`, `internal/cli/terminal_linux.go`
- modernize: 26 suggestions

## Verification

- `go test -race -short ./...` passes on Linux amd64 with Go 1.27.4.
- The baseline was not fixed in this change. The security-relevant items need their own reviewed branch with tests.
- No Linear ticket filed yet (the tool call was not permitted in this session).
