# Lock file

- **Status:** Done
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Pin the recipe input's git revision and hash, and each tool's image digest, in `stack.lock` ([ADR 0008](../../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).

## Acceptance

- The same `stack.yaml` and `stack.lock` give identical digests on two servers.
- `update` rewrites the lock and nothing else.

## Progress

- 2026-10-19: the standalone `internal/lock` package reads, validates and writes the lock byte-identically, and resolves image tags to index digests; `internal/catalog` verifies the locked recipe input. See [catalog-lock.md](catalog-lock.md) for implementation and verification evidence.
- 2026-10-20: wired `update`, first `check`, and locked `check`/`build`. The lock remains strict JSON with atomic replacement. Builds use full catalog revisions and image index digests; missing tools and tampered hashes fail. Local overrides skip catalog pins visibly while enforcing existing tool pins. See [catalog-lock.md](catalog-lock.md) for tests, failures, verification and limits.
- ADR review: [0026](../../ADR.md#0026-wire-github-catalogs-and-preserve-local-development-overrides) records source expansion and local development behavior; no lock format change. No live GitHub or registry validation or full security scan was run.
