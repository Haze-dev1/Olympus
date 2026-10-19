# Lock file

- **Status:** In progress
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Pin the recipe input's git revision and hash, and each tool's image digest, in `stack.lock` ([ADR 0008](../../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).

## Acceptance

- The same `stack.yaml` and `stack.lock` give identical digests on two servers.
- `update` rewrites the lock and nothing else.

## Progress

- 2026-10-19: the standalone `internal/lock` package reads, validates and writes the lock byte-identically, and resolves image tags to index digests; `internal/catalog` verifies the locked recipe input. See [catalog-lock.md](catalog-lock.md) for implementation and verification evidence.
- Not wired: no command reads or writes the lock yet; `update` is deferred until slice 1 lands. A live registry check is still outstanding.
