# Core: check, build, plan, apply, rollback

- **Status:** Done for M1, full CLI lab acceptance verified
- **Updated:** 2026-10-20
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

[ADR 0020](../../ADR.md#0020-use-goccy-yaml-and-pelletier-toml-for-the-offline-core) records the exact approved YAML and TOML dependencies and the limited schema vocabulary. [ADR 0021](../../ADR.md#0021-keep-the-first-generation-contract-offline) records local artifact layout, canonical hashing and input identity. Its output-preservation policy is superseded by [ADR 0022](../../ADR.md#0022-atomically-replace-owned-local-build-directories), which authorizes atomic replacement of manifest-owned local output. TRD section 6.5 describes the implemented boundary.

MCP environment-variable delivery covers only the test recipe. Its `github-token` secret declares `env: GITHUB_TOKEN`; its MCP entry contains `${GITHUB_TOKEN}`, never the source value. Compose references a future per-tool environment file; no such file is generated. Per-tool delivery and syntax remain open under TRD section 16. No wrapper scripts or general runtime secret mechanism were added.

At the original offline milestone there was no SSH, server access, apply, plan, rollback, catalog fetching, registry resolution or `stack.lock`; subsequent catalog/remote/activator work and the plan slice below supersede that boundary. Tagged images and relative local mounts make these inspection artifacts, not deployable locked generations. Hooks and health settings are validated but never run. Public port builds and channel rendering fail explicitly. Rebuilds stage and atomically replace manifest-owned output. Existing directories without valid Olympus manifests are preserved and refused.

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

## PR #5 review follow-up, 2026-10-19

All three requested findings are addressed on `feat/m1-offline-core`:

1. **High, integer precision:** `schema.read` retains native int64/uint64 values. Integer validation and typed stack/recipe decode no longer receive rounded floats. Encoder JSON normalization and JSON seeds use `UseNumber`, then convert integer tokens into concrete integer types for YAML/TOML encoders. Tests cover nested integers, signed int64 limits, a value above 2^53, typed stack decoding, and a TOML managed seed retaining `port = 8642`.
2. **Medium, changed builds:** stage a complete generation in a fresh sibling directory, then use native atomic exchange for manifest-owned output. Validate the old manifest and refuse unowned directories. Clean up prior owned output, without following its symlinks, after publication. The default-path CLI regression builds, edits the stack and builds again; additional tests cover stale files, failed staging, invalid ownership markers and unrelated output.
3. **Low, ignored errors:** propagate both `encode.Canonical` and `json.Unmarshal` errors at MCP entry conversion. These failure branches cannot be induced through the current string-only MCP struct without adding artificial injection hooks; source inspection verifies both checks, and existing MCP golden/format tests verify the legitimate path.

Verification observed in this follow-up:

- `go test ./internal/schema ./internal/encode ./internal/eval -run 'TestInteger|TestTOMLSeed'`: failed before fixes with `8642.0` and rounding of `9007199254740993` to `9007199254740992`.
- Initial focused rerun exposed the YAML parser's positive uint64 representation. Normalized positive integers within int64 range to int64; signed limits and cross-format round trips then passed.
- `go test ./internal/schema ./internal/encode ./internal/eval ./internal/stack ./internal/recipe`: passed after the integer fix.
- `go test ./internal/generation ./internal/cli`: passed after atomic replacement and default rebuild tests.
- `go build -o /tmp/olym ./cmd/olym/`, `go vet ./...`, and `go test ./...`: passed, exit 0 after all product changes; seven test-bearing packages passed, two packages had no test files. Existing golden output and purity tests pass without regenerating expected files.
- `GOOS=linux GOARCH=arm64 go build -o /tmp/olym-linux-arm64 ./cmd/olym/`: passed, exit 0.
- `GOOS=darwin GOARCH=amd64 go build -o /tmp/olym-darwin-amd64 ./cmd/olym/`: passed, exit 0.
- `GOOS=darwin GOARCH=arm64 go build -o /tmp/olym-darwin-arm64 ./cmd/olym/`: passed, exit 0.
- These cross-target checks compile the syscall implementation; no macOS or Linux arm64 runtime was available. Unsupported platforms/filesystems and power-loss scenarios were not exercised.
- `python /tmp/olym-check-docs.py`: passed, exit 0, checking all 162 relative documentation links/anchors plus tracker and schema JSON.
- `git diff --check`: passed, exit 0.

ADR 0022 records the changed durable output-ownership and replacement policy, superseding that portion of ADR 0021. Integer preservation and error propagation correct the existing contract and require no separate architecture decision. No dependency was added or changed. The feature remains in progress because server behavior is still outside this slice.

Security: output deletion is now authorized only for manifest-owned local build directories or temporary directories created by this invocation. A3 confinement and symlink defenses remain in place. Existing sentinel leak, localhost and YAML-safety tests pass. No real secrets were read, no server was contacted, and no full vulnerability scan or independent security review was performed. Atomic visibility is tested on Linux amd64; other platforms' runtime behavior remains unverified.

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

## Read-only plan and manifest health, 2026-10-20

Implemented on `feat/m1-plan`, cut from `development` at `f890280`. Health data is optional in version-1 generation tools, copied from recipes without secret values, and round-trips through JSON. `activate.ManifestHealthSource` takes a validated manifest loader keyed by generation ID and converts both active/previous checks directly into activator Health values. Existing activator code and server command wiring are untouched. ADR 0028 amends 0027; TRD 5.4, 6.5, 7.2 and 10 pin the format and implemented behavior.

Plan builds in memory, selects known_hosts by flag/environment/home order, authenticates with the stack key or agent, refuses unknown keys without a terminal, and reads one resolved generation snapshot. It verifies the manifest canonical hash and compares file SHA-256 values with the server's recorded hashes. It reports new/removed tools, file/image/configuration changes, drift and unchanged totals. Detailed exit codes are 0/2, errors 1. Passwordless sudo is checked before non-root reads. No remote write commands, lock, package installation, health execution or container operations occur.

### Verification in this run

- Go `1.27.2 linux/amd64` satisfies the 1.26 minimum.
- `go test ./internal/eval -update`: passed after intentional health-field golden regeneration. Inspected the golden manifest: health HTTP path/port/timeout added and generation ID changed; file/tool hashes unchanged.
- `go test ./internal/plan ./internal/cli ./internal/activate ./internal/eval`: passed. New tests cover new/removed tools, changed/added/removed files, changed image, unchanged tools, drift, absent current, permission/error rejection, bounded/validated manifest flow, sudo argument lists, known_hosts precedence, fake-reader CLI output and exit codes, health JSON round-trip and HealthSource compatibility.
- `go build -o /tmp/olym ./cmd/olym/`: passed, exit 0.
- `go vet ./...`: passed, exit 0.
- `go test ./...`: passed, exit 0, 13 test-bearing packages and two packages without tests.
- `gofmt -l .`: passed, exit 0, no output.
- `python3 /tmp/olym-check-docs.py`: passed, 211 relative docs links/anchors, tracker JSON and published schema JSON. `git diff --check`: passed.
- Initial sandboxed checks failed on Go cache writes and loopback test listeners; approved reruns passed.
- Scoped Olympus QA fronts: static/unit green; recipe golden green for the fictional recipe, no production recipe changed; edge cases covered as above, remaining gaps below; live plan red; server exposure/secret modes not run because this slice performs no deployment.
- Read settings from `~/Projects/Olympus/lab/lab.sh env`. Sandboxed `status` said not running because process visibility was restricted; approved `status` confirmed running and SSH reachable on 127.0.0.1:2222.
- Exactly one read-only lab attempt used an isolated `/tmp/olym-plan-lab/stack.yaml`, the lab key explicitly in `server.ssh_key`, explicit lab `-known-hosts`, `XDG_CACHE_HOME=/home/haze/Projects/Olympus/.lab/cache`, and the local fictional recipe override. `olym plan --detailed-exitcode` exited 1 during SSH handshake with connection reset by peer, before any generation read. No successful live diff was verified. The VM was not started, stopped, reset or otherwise changed; its owner retains it.

### Incomplete scope and review limits

Secret value change detection is deliberately absent: server HMAC fingerprints do not exist yet. Saved plans, semantic config-key diffs, port/RAM/OS/architecture preflight and apply/server command integration remain outside this slice. HealthSource loading must be wired by the server-command implementation. The live handshake failure remains unresolved and needs another authorized live check. No full vulnerability scan or independent security review ran. Existing remote tests cover unknown/changed host keys and shell quoting; this run adds known_hosts selection tests, but does not exercise an interactive terminal prompt on the VM.

Security: plan never reads runtime secret values or prints file contents/remote stderr (A1); every remote operation is a quoted argument list (A2); host-key verification is mandatory and noninteractive unknown keys fail closed (A6); no server changes occur (A11). No new dependency was added. No rule was intentionally relaxed. Review the manifest adapter integration and failed live SSH check before treating server behavior as fully verified.

## M1 completion: per-tool store contract, 2026-10-20

ADR 0030 replaces generation-relative config binds with absolute per-tool content-addressed paths. Evaluation changes are limited to mount sources, file layout and the necessary hash ordering. Golden artifacts migrated to the new layout. Uploads validate all declared files, publish tool entries separately, verify existing entries and skip their transfer. Plan hashes tool entries; GC retains kept manifests' tool entries. No new dependencies. The CLI commands and full lab cycle remain incomplete; M1 stays in progress.

Focused eval, deploy and plan tests passed. The initial CLI run failed because sandbox policy blocks localhost test listeners; the full suite is rerun with approved access. Verification and commit evidence follow below. Security: entries are validated before upload; existing corruption is refused, generated mounts stay under the Olympus store, and no secret values enter artifacts. No external scan in this slice.

First-slice validation: `GOCACHE=/tmp/olym-go-cache go build -o /tmp/olym ./cmd/olym/`, `go vet ./...`, and the approved `go test ./...` all passed with Go 1.27.2. `gofmt -l .` printed nothing; the documentation checker passed 220 relative links/anchors plus tracker/schema JSON; `git diff --check` passed. The lab cycle is deferred to the subsequent command-integration slice, not claimed as complete.

## M1 command completion, 2026-10-20

The first contract commit `505519a` was pushed early on `feat/m1-complete`. The subsequent integration implements apply with local approval and saved-plan staleness checks; status text/JSON; detached rollback and fresh-SSH confirmation; GC using flag, server.keep or 5; and confirmed destroy that keeps volumes/state unless explicitly deleting data. All commands use the stack's connection settings and the shared known_hosts policy. Apply checks expected current again under the server lock. Secret values are read only at the CLI boundary from the adjacent .env and delivered per declared tool; HMAC fingerprints and selective recreation preserve generation purity. Partial secret-install I/O errors restore prior tools. Changed image_tag overrides fail with `run olym update`.

ADRs 0030–0033 record stable tool entries, per-request confirmation/saved plans, secret transactions/fingerprints and the additive lock image_tag field. No other architecture decision was needed. No dependencies were added or changed. The evaluator and golden changes remain confined to the separately authorized store layout/mount-source contract in the first commit. Recipe, schema and production-recipe behavior remain owned by the other agent.

### Verification and failures

Go 1.27.2 was used. Full unit suite passed all 13 test-bearing packages; two packages have no tests. Build, vet and formatting passed. The tagged CLI test skips without OLYMPUS_LAB=1. Commands use GOCACHE=/tmp/olym-go-cache; lab tests use the exact key, known_hosts and XDG cache from `/home/haze/Projects/Olympus/lab/lab.sh env`, never the developer's SSH files or cache.

The first full CLI lab cycle passed in 72.951 s. The expanded run passed in 79.365 s, including failed readiness and concurrency. It uses the fixed nginx index digest `sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10` and test recipes under internal/cli/testdata, with no provider API calls. The final source-complete rerun includes the first-server fingerprint regression assertion and passed in 77.902 s.

Failures observed and corrected during integration:

- Sandboxed tests could not bind localhost listeners. Approved full-suite reruns passed.
- Saved plan followed by flags was rejected by Go flag parsing. Interspersed parsing preserves flag/value pairs; a regression test and live saved apply cover it.
- Repeated privilege detection ran id through sudo, then dropped sudo for Docker bootstrap. UseServer now determines the actual SSH uid before wrapping subsequent commands; unit and fresh-VM apply cover it.
- An expanded test read an earlier health failure's rolled-back state while the killed operation still held the lock, then treated legitimate GC contention as failure. The test now requires the killed request's own terminal state. The subsequent expanded run passed.
- Before the final source check, first-server plans computed fingerprints without a server key. They now omit those fingerprints and never persist a publicly keyed hash; the first apply reads current values. Existing-server saved plans bind secret values with the real server key.

### QA fronts and limits

| Front | Result | Evidence / limit |
| --- | --- | --- |
| Static/unit | Green | Full build, vet, unit suite and gofmt; package checks during iteration |
| Recipe validation | Green for test recipes | Two nginx fixtures parse and run; production recipes not changed or certified |
| Edge cases | Green for requested M1 flow; other rows remain gaps | Tests below; no simulated full disk, capacity/port conflict, unsupported remote architecture or real firewall loss |
| Live cycle | Green | Reset Ubuntu VM, actual CLI subprocesses, pinned image; detailed result below |
| Exposure/secrets | Green for exercised boundaries | No sentinel in CLI output, store, activator/container logs; exact special-character delivery; mode 0600; unrelated Docker container/volume survives destroy; external VPS scan not run |
| Review/scanners | Scoped source review | [Review record](../m1-review.md); no dependency or secret scanner installed, none added |

The live test asserts check=0, plan detailed=2, saved apply=0, status=0 with two running healthy tools and image/generation fields, reapply=0 with identical container IDs, stale saved apply=1, config plan=2 and only the configured tool recreated, rollback=0 with the unrelated tool unchanged, secret plan=2 and only that tool recreated, failing health apply=3 with previous current retained, competing apply=4, actual CLI kill after detached start with autonomous rollback, GC=0, destroy=0 with volumes/state retained, and delete-data destroy=0 with state/project volumes removed while unrelated resources survive. Key/digest/version rejection, partial secret-install failure, failed rollback=5, known/unknown/changed host keys, quoting and confinement are additionally covered by unit tests.

Remaining release limits: amd64 Ubuntu only in this lab; no real VPS, arm64 runtime, macOS client or interactive unknown-host-terminal test. A Linux activator is selected locally and byte/version verified; release download/signature/checksum embedding is not implemented. Explicit rollback retains current credentials; automatic rollback restores failed-transaction credentials. Unused incoming payloads/retained units can remain after an interrupted contender and need cleanup or delete-data destroy. Corrupt immutable store entries are refused, not repaired in place. No historical secret archive, container-user file-secret ownership, production hook execution, capacity/port preflight or arbitrary production-recipe certification is claimed. These do not alter the requested M1 lab acceptance; later recipe/release work remains separate.

Security: A1 values remain outside evaluated/saved artifacts, logs and argv; A2 uses quoted argv; A3 uses confined hash-addressed entries; A4/A9/A10 retain the existing evaluator/validation checks. A5 delivers only declared per-tool environment secrets with root-only files. A6 verifies both SSH connections. A7 deploys only digest-pinned images. A8 runs readiness commands inside the project container, and no host-side hooks were added. A11 scopes runtime deletion to Olympus; the explicitly requested Docker bootstrap additionally installs system apt/Docker configuration on the disposable server. A12 verifies the locally selected Linux build's bytes and version. Review the release trust and file-secret ownership limits before release; no external vulnerability scan was run.

### Final source-complete results

- `OLYMPUS_LAB=1` with explicit lab host/port/user/key/known_hosts, `XDG_CACHE_HOME=/home/haze/Projects/Olympus/.lab/cache` and `GOCACHE=/tmp/olym-go-cache go test -tags lab ./internal/cli -run TestLabM1 -count=1 -v -timeout 20m`: **PASS**, 77.902 s on a freshly reset canonical VM. Every acceptance assertion above passed, including initial saved-plan fingerprint omission and the exact special-character secret bytes. Only sentinel values were used.
- Final `GOCACHE=/tmp/olym-go-cache go build -o /tmp/olym ./cmd/olym/`: passed, exit 0.
- Final `GOCACHE=/tmp/olym-go-cache go vet ./...`: passed, exit 0.
- Final approved `GOCACHE=/tmp/olym-go-cache go test ./...`: passed, exit 0, 13 test-bearing packages and two without tests. The I/O restoration regression reaches beta's backup failure after alpha's replacement and proves alpha's old values/fingerprints are restored.
- `gofmt -l .`: no output. `git diff --check`: passed. Documentation checker: all 234 relative links/anchors, tracker JSON and published schema JSON passed.
- `GOCACHE=/tmp/olym-go-cache go test -tags lab ./internal/cli ./internal/deploy -run TestLab -count=1` without OLYMPUS_LAB: passed with live tests skipped as designed. The older package-only deploy test was not run live again; the broader final CLI test exercises the integrated path.
- The canonical VM was shut down with its lab.sh down after verification. The final delete-data step removed Olympus state and project volumes; unrelated test resources were explicitly cleaned up after their survival checks.

This final section supersedes the historical offline/read-only scope statements above. M1 is complete at the requested lab acceptance scope, with the release limits listed above retained explicitly.
