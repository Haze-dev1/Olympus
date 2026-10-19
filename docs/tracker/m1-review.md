# M1 scoped security review, 2026-10-20

Baseline: development `b59ee7a`, including first contract commit `505519a` and the M1 integration diff. This is a source review of the changed CLI/server workflow, not release approval or an external scan. The review uses `.agents/skills/review-app-security/SKILL.md` and the Olympus threat model in AGENTS.md. The in-flight QA uses `.agents/skills/olympus-test/SKILL.md`.

## Progress

- [x] Set scope, inventory and data flow.
- [x] Trace four threat-model questions and critique the answers.
- [x] Check changed files and cross-file guards.
- [x] Recheck project rules and A1–A12.
- [x] Record scanner and exposure limits.
- [x] Produce findings and follow-up limits.

## Data flow and threat model

There is no separate security data-flow file. This note records the scoped flow: stack/locked recipes pass existing validation into pure evaluation; only the CLI reads values from the adjacent .env; verified SSH transports argv and a per-tool payload on stdin; a root-only detached activator takes flock, installs values/fingerprints, runs the Olympus Compose project and health checks, then waits for its request's nonce on fresh verified SSH. A saved plan carries ids, target and keyed fingerprints, never values/key. Before a server key exists it carries no secret hashes. Docker and retained generations are server state, not a remotely callable web API.

The caller is the server owner, authenticated as its SSH user and required to be root or have passwordless sudo. Internal server commands check euid 0. Recipes are trusted executable inputs, not tenant content. Malicious inputs considered include path traversal and invalid ids, undeclared secret/tool names, shell syntax, changed host keys, stale plans/nonces, competing operations, partial secret writes and secret text in errors. The root trust boundary is unchanged: a malicious server root can already read or replace its own deployment.

Self-critique: a client-only expected-current check would race, so the server repeats it under flock. A new nonce alone would not identify a rollback contender, so state also carries the request id. A unique secret file alone would not prove SSH survived, so confirmation still uses a fresh verified connection. Values in environment files need Compose escaping, which the live sentinel includes dollars, backslashes and quotes to exercise. A terminal rolled-back status is not proof about a later run; the CLI and lab test correlate request ids. A missing initial key cannot safely produce persisted secret hashes; the final code omits them.

## Inventory and verdicts

Targets reviewed: 46 of 46 inventory rows. Critical: 0. High: 0. Medium: 0. Low: 0. All production targets in the diff were reviewed. Test/fixture and documentation targets are listed separately because they introduce no new production entry point. Line references identify the boundary checks; this inventory describes the reviewed source, not every pre-existing behavior in the package.

| File | Target / boundary | Verdict |
| --- | --- | --- |
| internal/cli/cli.go | Command parsing and stack/lock boundary | Safe in scope: validated stack before dispatch at line 53; image_tag comparison at line 127; interspersed argv parser at line 217 |
| internal/cli/commands.go | Saved plan, approval, fingerprint and connection flow | Safe in scope: snapshot equality before preparation at line 88; expected current supplied at line 140; declaration-filtered secrets precede SSH at line 39 |
| internal/cli/secrets.go | .env to per-tool runtime payload | Safe in scope: name parsing at line 46, declaration lookup at line 90, duplicate target conflict check near line 104; errors omit values |
| internal/cli/plan.go | Shared host policy | Safe in scope: known_hosts selection at line 21 and terminal-only unknown-key approval in connectionConfig at line 50 |
| internal/cli/server.go | Root-only command dispatch, bounded identifiers and output suppression | Safe in scope: payload/request validation at line 90, root check at line 96, quiet Compose methods at line 44 |
| internal/deploy/deploy.go | Privileged argv, verified publication, secrets stdin and request confirmation | Safe in scope: sanitized status-only errors at line 65; hash/id validation at line 117; regular tar paths at line 172; checksum/version at line 454; request matching at line 428 |
| internal/deploy/commands.go | Privilege detection, HMAC metadata reads and read-only binary verification | Safe in scope: unsudoed uid detection at line 14; bounded key/fingerprint reads at line 44; selected-byte verification at line 73 |
| internal/activate/activate.go | Server transaction and secret restoration | Safe in scope: flock before expected-current check at lines 216/227; deadline at line 297; full payload validation before replacement at line 735; restoration on partial install at line 706; Compose env escaping at line 912 |
| internal/activate/fingerprints.go | HMAC key and domain separation | Safe in scope: standard crypto/hmac at line 16; CSPRNG and exclusive 0600 key creation at line 28 |
| internal/activate/commands.go | Status/readiness and scoped destruction | Safe in scope: fixed project Compose argv at line 37; destroy flock at line 107; labelled volume selection at line 128; production root fixed by server dispatcher |
| internal/activate/compose.go | Selective recreation | Safe in scope: argv scoped through existing Compose args, Recreate at end of file; no shell evaluation |
| internal/activate/gc.go | Retention and deletion | Safe in scope: resolve kept generations before deletion at line 63; retain valid manifest hashes at line 80; deletion stays in store/bin |
| internal/plan/plan.go | Current metadata and tool paths | Safe in scope: current-link/canonical-manifest checks at line 91; tool-hash validation at line 148; file path confined to fixed store prefix |
| internal/lock/lock.go | Additive image override field | Safe in scope: strict JSON decoder and digest validation remain at lines 75/45; CLI compares the override before using the digest |
| internal/remote/remote.go | Host-key algorithm preference | Safe in scope: hostKeyCallback still required at line 203; preference at line 301 accepts only keys already verified for that host and never disables verification |
| internal/eval/eval.go | Authorized mount/layout-only contract | Safe in scope: Confined recheck before ToolHash at line 339; absolute source derived from tool hash at line 350; no new template/secret behavior |
| internal/activate/activate_test.go | Secret I/O failure, fingerprints and GC regressions | Tests only: temporary roots and fake runners; sentinel/old-new fixture values, no real credentials |
| internal/deploy/deploy_test.go | Upload reuse/tamper, sudo and exit status regressions | Tests only: fake SSH and temporary fake-server filesystem; diagnostic sentinel is not a live credential |
| internal/deploy/lab_test.go | Existing tagged deployment fixture compatibility | Tests only: adds valid tool hash required by the new contract; opt-in environment guard remains |
| internal/cli/catalog_test.go | Image override mismatch regression | Tests only: temporary lock/stack and offline catalog inputs |
| internal/cli/cli_test.go | Store-layout rebuild regression | Tests only: verifies old tool entry disappears from local output after replacement |
| internal/cli/commands_test.go | Saved positional plan followed by flags | Tests only: flag parsing, no network or secrets |
| internal/cli/lab_test.go | Full CLI acceptance cycle | Tests only: OLYMPUS_LAB=1 gate; explicit canonical lab credentials/cache; only generated sentinel values; fixed image digest and labelled cleanup |
| internal/plan/plan_test.go | New store path and secret-change counts | Tests only: fake Reader; no remote writes |
| internal/eval/eval_test.go | Contract golden/purity and stable mount regression | Tests only: existing sample, temporary sentinels, deterministic artifacts |
| internal/cli/testdata/lab-one/recipe.yaml | Test service declarations | Fixture: nginx image pinned by test lock, no published ports, one declared environment secret, in-container nginx readiness |
| internal/cli/testdata/lab-two/recipe.yaml | Independent test service | Fixture: same restrictions, separate named volumes and secret mapping |
| internal/cli/testdata/lab-one/default.conf.tmpl | Read-only config source | Fixture: static nginx response, no secret interpolation |
| internal/cli/testdata/lab-two/default.conf.tmpl | Independent config source | Fixture: static nginx response, no secret interpolation |
| testdata/golden/compose.yaml | Authorized mount-source migration | Fixture: absolute Olympus tool store paths, existing localhost ports and environment-file reference |
| testdata/golden/manifest.json | Expected content identity | Fixture: secret names only, updated hashes; no values |
| testdata/golden/store/.../files/opt/observatory/SOUL.md | Existing fictional template output moved to tool entry | Fixture: bytes unchanged |
| testdata/golden/store/.../files/opt/observatory/config.yaml | Existing fictional managed output moved to tool entry | Fixture: bytes unchanged, runtime variable reference only |
| README.md | User command examples and limits | Documentation only; reviewed for current behavior |
| CHANGELOG.md | Actual M1 changes | Documentation only; no generated/secret values |
| docs/ADR.md | Contracts 0030–0033 | Documentation only; records trust/failure tradeoffs |
| docs/TRD.md | Implemented protocol and command contract | Documentation only; no safety bypass or no-confirm mode |
| docs/specs/stack-file.md | Lock and saved-plan formats | Documentation only; no values/key in saved plans |
| docs/edgecases/apply.md | Saved/concurrent/deadline expectations | Documentation only; checked against implementation/tests |
| docs/plans/v1.md | M1 build-order status | Documentation only; refers to actual lab scope |
| docs/tracker/features/core-plan-apply.md | Full evidence, failures and release limits | Documentation only; no blanket release certification |
| docs/tracker/features/deploy.md | Protocol integration evidence | Documentation only |
| docs/tracker/features/activator.md | Server integration evidence | Documentation only |
| docs/tracker/features/remote-ssh.md | Shared connection integration evidence | Documentation only |
| docs/tracker/features/lock-file.md | Image override follow-up | Documentation only |
| docs/tracker/progress.json | Milestone/feature status | Metadata only; mark done only after full lab result |

## Project rules drift

Generic rules 1, 2, 3, 5, 7, 9 and 10 about web tenants, RLS, sessions, CSRF, uploaded web files and browser output are N/A: no web app, database or multi-user API. Applicable shell/path/input/secret rules are covered above. Standard-library HMAC and CSPRNG are used, not custom cryptography. No package was added. File contents are data, not instructions. Reading .env values, lab key authentication, server bootstrap/deletion, commits and this branch's push/PR are explicitly authorized by the user's M1 task; no real developer .env was inspected.

| Rule | Result / evidence |
| --- | --- |
| A1 | Values on stdin only; no values/key in generation or saved plan; diagnostics suppress remote/container output; live sentinel checks |
| A2 | remote.Run quotes each argv; local Compose uses exec.CommandContext; no new raw-shell API |
| A3 | Existing Confined validation, hash-addressed fixed store prefix, regular-file-only upload |
| A4 | Existing localhost rendering and golden tests unchanged; lab fixtures publish no ports |
| A5 | CLI filters by recipe declarations and server checks manifest secret names; 0600 per-tool files and fingerprint backups; current delivery is environment-based |
| A6 | Initial and fresh confirmation SSH both use host verification; pinned-algorithm preference does not bypass callbacks |
| A7 | Lock digests supplied to evaluation; apply refuses tag-only images; image_tag drift requires update |
| A8 | Health commands execute inside scoped Compose services; no host hook path was added; production hook execution belongs to recipe work |
| A9 | Existing strict safe YAML parsing reused, no new YAML loader |
| A10 | Existing allow-list checks reused; no channel implementation changed |
| A11 | Compose operations and volume deletion scoped to project olympus; server files under Olympus root. Explicit Docker bootstrap additionally installs apt/Docker system configuration as required by the task |
| A12 | Local Linux binary bytes and version verified before activation; read-only status also verifies the selected binary. Release download/signature/checksum embedding remains a release limitation |

## Findings and scan limits

No unresolved security finding was identified in this scoped source review. This does not certify the whole tool or production recipes. The initial publicly keyed first-server fingerprint was corrected before this final review; the regression assertion is in the final lab test.

No dependency was added or changed. govulncheck and gitleaks are not installed in the current environment, and no scanner/package was installed. Run a Go-aware vulnerability scan and a repository/history secret scan before release, then review any results as a separate fix list. No public endpoint, bucket, CORS or browser-header change was made, so HTTP Observatory is inapplicable. A real VPS external port scan, arm64 runtime, macOS client and interactive terminal host-prompt test remain unverified.

If later scans find Critical/High issues, fix those first with apply-security-fixes or an isolated fix session, verify both rejection and legitimate command flows, and repeat this review. Current review is not deployment approval. The PR still needs human review of the server trust, retention and release limits.
