# Core: check, build, plan, apply, rollback

- **Status:** In progress, offline slice implemented
- **Updated:** 2026-10-19
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

Evaluate the stack locally into a generation, then activate it on the server over SSH ([ADR 0014](../../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server), [0015](../../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)). Requirements in [PRD.md](../../PRD.md) sections 7.1, 7.4, 7.5 and 8; design in [TRD.md](../../TRD.md) sections 6–8.

## Implemented in this slice

- Go module and `cmd/olym/`, with standard `flag` commands `check` and `build`.
- Embedded published stack/recipe schemas, positioned YAML validation, unknown-key rejection, secret-capable interpolation restrictions, tool-name checks, duplicate detection, reference checks and channel allow-lists.
- Adjacent `.env` name scanning without expansion, value storage or process-environment fallback. Every missing variable use is reported without a value.
- Local recipe loading, template rendering without secrets, structured YAML/TOML/JSON managed MCP config, named volumes, localhost ports, file hashes, tool hashes and generation identity.
- The fictional Observatory recipe and MCP sample under `testdata/sample/`, with byte-for-byte expected generation under `testdata/golden/`.
- Build, vet and test commands in `AGENTS.md` and the `olympus-test` skill.

## Decisions and boundaries

[ADR 0020](../../ADR.md#0020-use-goccy-yaml-and-pelletier-toml-for-the-offline-core) records the exact approved YAML and TOML dependencies and the limited schema vocabulary. [ADR 0021](../../ADR.md#0021-keep-the-first-generation-contract-offline) records local artifact layout, canonical hashing, input identity and preservation of differing existing output. TRD section 6.5 describes the implemented boundary.

MCP environment-variable delivery covers only the test recipe. Its `github-token` secret declares `env: GITHUB_TOKEN`; its MCP entry contains `${GITHUB_TOKEN}`, never the source value. Compose references a future per-tool environment file; no such file is generated. Per-tool delivery and syntax remain open under TRD section 16. No wrapper scripts or general runtime secret mechanism were added.

There is no SSH, server access, apply, plan, rollback, catalog fetching, registry resolution or `stack.lock`. Tagged images and relative local mounts make these inspection artifacts, not deployable locked generations. Hooks and health settings are validated but never run. Public port builds and channel rendering fail explicitly. An identical build reuses output; differing existing output requires a fresh `-o` directory and is preserved.

## Verification evidence

Commands and results from 2026-10-19:

- `go test ./internal/stack ./internal/recipe`: initially failed the `08` YAML type-trap case. Added explicit quoting checks for ambiguous plain strings; rerun passed.
- `go test ./internal/stack ./internal/recipe ./internal/encode`: passed.
- `go test ./internal/eval -update`: passed; generated the committed expected generation after intentional canonical hash changes.
- `go test ./...`: passed after CLI, output confinement and hashing tests were added.
- Final `go build -o /tmp/olym ./cmd/olym/`: passed, exit 0, using Go 1.27.2 against the module's Go 1.25 minimum.
- Final `go vet ./...`: passed, exit 0, no diagnostics.
- Final `go test ./...`: passed, exit 0; six test-bearing packages passed, three packages reported no test files, zero package failures.
- `python /tmp/olym-check-docs.py`: passed, exit 0; checked all 157 relative links/anchors in `docs/`, tracker JSON and published schema JSON. The checker is a session-local verification aid, not a new project dependency.
- `git diff --check`: passed, exit 0.

| Scoped QA front | Result | Evidence |
| --- | --- | --- |
| Static and unit | Green | Final build, vet and test commands above |
| Recipe validation | Green for fictional recipe | Recipe parser, confinement, managed-format and golden tests; upstream image verification is inapplicable to the fictional tool |
| Edge cases | Green for offline slice, server gaps recorded | Offline coverage table below |
| Live cycle | Not run | No server commands in this slice |
| Exposure and secrets | Green offline, live checks not run | Localhost golden ports, sentinel output/diagnostic checks, source/output path tests; server modes and exposure untested |

No server or container was started, so no server teardown was required. Final review found and fixed a possible volume-name collision between hyphenated tool/volume pairs; `eval.TestVolumeNamesCannotCollideAcrossTools` now covers it and the complete suite passed afterward.

The first sandboxed dependency download failed because network access was blocked. Both exact approved versions downloaded with approved access. A sandboxed initial test invocation reported installed standard packages unavailable; rerunning with approved toolchain/cache access passed. Updating the protected test skill required filesystem approval and then succeeded.

## Offline edge-case coverage

| Case from apply edge cases | Evidence or boundary |
| --- | --- |
| Missing variable, including every use; missing `.env` | `stack.TestEnvNamesOnly` |
| YAML type traps | `stack.TestValidation` |
| Unknown MCP reference | `stack.TestValidation` |
| Recipe not found, closest suggestion | `recipe.TestMissingRecipeSuggestion`, `eval.TestDiagnosticsLoadedRecipe` |
| Unsupported MCPs or skills | `eval.TestRenderingFailuresAndWarnings`, `eval.TestConfinedSkillsAndUnsupportedWarning` |
| Unsupported channel | Explicit failure, per TRD 6.1; `eval.TestRenderingFailuresAndWarnings` |
| Stack image override versus lock | No lock in this slice; lock reconciliation not run |
| Telegram with empty allow-list | `stack.TestValidation`, `eval.TestRenderingFailuresAndWarnings` |
| Same stack rendered twice | `eval.TestGoldenAndPurity`, `cli.TestOfflineCommands` |
| Secret value changed | `eval.TestGoldenAndPurity`, with two temporary sentinel `.env` values |
| Stack map order changed | `eval.TestGoldenAndPurity` |
| A3 path escape or symlink | `recipe.TestConfinement`, `eval.TestBuildRejectsUnsafeDestination`, `generation.TestWriteConfinement` |
| A1 output leak or diagnostic leak | `eval.TestGoldenAndPurity`, `cli.TestFailuresNeverEchoInputValues`, `cli.TestOfflineCommands` |

## Remaining acceptance and skipped verification

- A one-tool stack applies on a fresh Ubuntu server.
- A second `apply` with no changes reports no changes.
- `rollback` after an upgrade restores the previous images.
- A failing health check leaves the previous generation current.
- Killing the CLI after activation starts ends with the server on the previous generation within `confirm_timeout`.
- The activator refuses to run with a wrong checksum or version.
- `destroy` removes the Compose project and keeps volumes.
- Server, apply, rollback, GC and destroy rows in the edge-case matrix remain untested because those commands do not exist yet.
- No real image was pulled, no provider API was called, and no disposable-server or production-recipe test was run. No vulnerability scanner or independent security review was run.

## Security note

A1 is exercised with sentinel leak tests; the evaluator receives no secret values. A3 rejects traversal, destination escapes and source/output symlinks. A4 keeps all generated ports on localhost. A9 rejects executable tags, aliases and anchors. A10 rejects empty channel allow-lists. A5 is limited to declared secret names and future environment-file references; server permissions and materialization are unimplemented. A2, A6, A7, A8, A11 and A12 have no runtime action in this offline slice: no remote commands, pulls, hooks, activation or server writes occur. Review the exact golden artifacts, new ADRs and test-recipe secret convention before merging; runtime rules still require later server tests and a security review.
