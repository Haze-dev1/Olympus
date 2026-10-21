# Catalog and lock packages

- **Status:** Done
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Standalone `internal/catalog` and `internal/lock` packages for slice 2: fetch and verify the recipe catalog, and read, write and resolve `stack.lock` ([TRD 6.1](../../TRD.md#61-steps), [6.4](../../TRD.md#64-lock-resolution), [5.3](../../TRD.md#53-lock-file-stacklock); [ADR 0008](../../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).

## Scope

- `internal/catalog`: parse a `github:owner/repo[/path]` source, download the tarball for a full commit revision, extract only the catalog subdirectory into `~/.cache/olympus/inputs/<hash>/`, reject paths outside it, and compute and verify the canonical tree hash.
- `internal/lock`: strict JSON read/write of `stack.lock` (sorted keys, one trailing newline), validation of pinned inputs and images, and resolution of an image tag to its multi-architecture index digest through the registry HTTP API.
- CLI wiring now covers `check`, `build` and `update`; `plan`, `apply`, SSH and activation remain outside this feature.

## Acceptance

- The same extracted tree hashes identically regardless of file creation order or mtimes; a mode or content change changes the hash.
- A fetched tarball whose tree does not match the expected hash is rejected, and a damaged cache entry is discarded.
- Tarball entries that escape the extraction directory, absolute paths, symlinks, hardlinks and special files are rejected.
- `stack.lock` round-trips byte-identically: sorted keys, two-space indent, one trailing newline. Unknown fields, unpinned images and malformed hashes are rejected.
- A tag resolves to `name@sha256:<index digest>`; a single-platform manifest, a digest-header mismatch and a non-200 response are rejected.

## Progress

- 2026-10-19: implemented both packages and their offline tests. `go.mod` was created with `module github.com/Haze-dev1/Olympus` and `go 1.27.2`; no dependencies.
- Catalog hash format recorded in [ADR 0023](../../ADR.md#0023-hash-the-extracted-catalog-tree-in-a-canonical-form); registry resolution choice in [ADR 0024](../../ADR.md#0024-resolve-image-digests-with-the-standard-library).
- The registry resolver handles public images with anonymous bearer auth only. Private registries and Docker credential helpers are not supported.
- Dependency proposal, not added: `github.com/google/go-containerregistry` **v0.22.1** (published 2026-09-04, Google-maintained, regular releases). It is a maintained upgrade path for credentials and platform handling, but the standard library covers v1's unauthenticated tag-to-index-digest need, so no module was added. Adding it needs the user's approval.

## Verification

Run from the repository root; no network is used (registry and tarball tests use `httptest`).

| Check | Command | Result |
| --- | --- | --- |
| Build | `go build ./...` | pass |
| Vet | `go vet ./...` | pass |
| Unit tests | `go test -count=1 ./...` | pass: `internal/catalog` 0.010s, `internal/lock` 0.005s |
| Race detector | `go test -race -count=1 ./...` | pass (2026-10-19, before docs changes) |
| Format | `gofmt -l .` | no output |

Tests cover hash determinism and a golden value, tampered tarball rejection, path escape/absolute path/symlink rejection, verified cache reuse and cache repair, lock round-trip against exact bytes, invalid lock rejection, registry digest resolution, bearer-token retry, single-manifest rejection and digest-header mismatch.

## CLI wiring evidence, 2026-10-20

- `check` creates a missing adjacent lock; `update` advances default-branch revision and image index digests without generation output. Existing locks require every stack tool and the selected source URL.
- Catalog builds use verified cached content, locked image digests in Compose and manifest tools, and the full revision in manifest inputs. Explicit local recipes announce the catalog-pin bypass and retain existing image pins; local builds without locks remain development-only. Local `update` is refused.
- GitHub-only optional credentials are isolated from registry requests and removed across foreign redirects. Catalog HTTP errors omit response bodies and transport details. YAML validation remains unchanged, rejecting custom tags and aliases under A9.
- Offline `httptest` integration covers newest revisions, first-check lock creation, build refusing a missing lock, existing-check lock preservation, locked digests and revision, network-free cached builds, missing tools, tampered catalog hashes and local overrides. Catalog tests cover anonymous/private lookups, malformed commits, HTTP and transport errors, and credential-free foreign redirects for lookup and tarballs.
- ADR review: added [0026](../../ADR.md#0026-wire-github-catalogs-and-preserve-local-development-overrides) because arbitrary GitHub sources supersede the official-only restriction and the local development bypass needs an explicit boundary. Existing lock/hash formats and registry choices are unchanged.

## Verification and limits for CLI wiring

Go 1.27.2: `go build -o /tmp/olym ./cmd/olym/`, `go vet ./...` and `go test ./...` passed. `gofmt -l .` and `git diff --check` produced no output. All 201 relative links and anchors under `docs/` resolve; `docs/tracker/progress.json` parses as valid JSON. Initial sandbox package checks could not write the Go cache or bind local test ports; rerun outside the sandbox passed. A test fixture initially used unsupported recipe kind `agent`; corrected to `harness`, then package tests passed.

No live GitHub, private repository or registry requests were made. Private registries and credential helpers remain unsupported under ADR 0024. No full security scan or server QA was requested; `internal/remote` and `internal/activate` are untouched.
