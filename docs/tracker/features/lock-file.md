# Lock file

- **Status:** Not started
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Pin the recipe input's git revision and hash, and each tool's image digest, in `stack.lock` ([ADR 0008](../../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).

## Acceptance

- The same `stack.yaml` and `stack.lock` give identical digests on two servers.
- `update` rewrites the lock and nothing else.
