# Core: check, build, plan, apply, rollback

- **Status:** Not started
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Evaluate the stack locally into a generation, then activate it on the server over SSH ([ADR 0014](../../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server), [0015](../../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)). Requirements in [PRD.md](../../PRD.md) sections 7.1, 7.4, 7.5 and 8; design in [TRD.md](../../TRD.md) sections 6–8.

## Acceptance

- A one-tool stack applies on a fresh Ubuntu server.
- A second `apply` with no changes reports no changes.
- Two `build` runs of the same stack produce byte-identical generations.
- `rollback` after an upgrade restores the previous images.
- A failing health check leaves the previous generation current.
- Killing the CLI after activation starts ends with the server on the previous generation within `confirm_timeout`.
- The activator refuses to run with a wrong checksum or version.
- `destroy` removes the Compose project and keeps volumes.
- Every case in [edgecases/apply.md](../../edgecases/apply.md) has a test.
