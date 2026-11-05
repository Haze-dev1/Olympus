# Shared MCP servers and skills

- **Status:** Done with rendering and real deployment verified in [first-recipes](first-recipes.md).
- **Milestone:** M3
- **Plan:** [plans/v1.md](../../plans/v1.md)

Define MCP servers and skills once and write them into each tool's config ([ADR 0012](../../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)).

## Acceptance

- One `mcps` entry appears in Hermes's YAML, Codex's TOML and OpenCode's JSON config.
- A tool without support produces a warning naming it.

## Evidence

- 2026-10-19 — MCP config facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): Hermes `mcp_servers` in `~/.hermes/config.yaml` (YAML, `${VAR}`/`${env:VAR}`); Codex `[mcp_servers.*]` in `~/.codex/config.toml` (TOML, secrets only by env-var name through `env_vars`, `bearer_token_env_var`, `env_http_headers`); OpenCode `mcp` in `~/.config/opencode/opencode.json` (JSON/JSONC, `{env:VAR}` and `{file:…}`).
- 2026-10-20 — `eval.translateMCP` renders the same stack entry into all three shapes, selected by `supports.mcps.style`. `testdata/golden-real` shows `github` with `env: GITHUB_TOKEN: ${GITHUB_TOKEN}` (Hermes), `env_vars = ['GITHUB_TOKEN']` with no interpolation (Codex) and `environment: {GITHUB_TOKEN: '{env:GITHUB_TOKEN}'}` (OpenCode). A recipe without `supports.mcps` still yields the warning `recipe does not support MCP servers; skipped`.
- 2026-10-20 — Skills are rendered once and delivered by `supports.skills.mode`: Hermes copies into `/opt/data/skills` via the init service because it writes that directory, Codex and OpenCode bind the same source read-only at `/root/.agents/skills` and `/root/.config/opencode/skills`. The golden build checks the copied skill reaches the Hermes init script and the mounted skill stays a read-only bind.
- Decisions: [ADR 0037](../../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references) and [0038](../../ADR.md#0038-mount-shared-skills-read-only-where-supported-and-copy-them-where-the-tool-writes).
- Original limit, resolved for ledger-aware generations by OLY-29 below: copied skills were not removed when deleted from the stack.

- 2026-10-20: real three-tool `TestLabRealStack` passed in 70.501 s. Writable
  config init, shared skill copy/mount, native MCP references, selective MCP-list
  recreation, rollback and delete-data teardown were exercised. Full evidence,
  prior failures and authentication limits are in [first-recipes](first-recipes.md).
  ADRs 0040–0041 cover this run's changed catalog/build behavior; no additional
  format decision was needed for native MCP or skill delivery.

## OLY-29, 2026-10-22

- Added a generated, hashed `.olympus-managed-files` ledger for copy-mode skills.
  Init removes only previously recorded files, retains unrecorded files and
  directories, and copies the new ledger last. Empty selections still get init.
- Security: shell paths stay quoted, Compose dollar signs are escaped, ledger
  traversal and symlink ancestors fail before pruning. No host-side deletion,
  new dependencies, runtime credentials or auth changes. Concurrent writes can
  race path checks under the existing init protocol; see ADR 0042.
- ADR review: added [0042](../../ADR.md#0042-track-and-prune-only-previously-copied-skill-files),
  amending 0038. Old untracked copies, older rollback generations and recipe
  directory/mode migrations require separate cleanup; no ownership is guessed.
- Regression executes rendered init with `/bin/sh` in a temporary filesystem.
  It fails against development because obsolete files survive. It covers skill
  removal, file removal, empty selections, repeat runs, restoring a selection,
  preserving agent files, malformed ledgers and symlink escape refusal.
- Iteration failures: corrected an unavailable test decoder and unused import;
  golden regeneration initially retained the old content-addressed directory.
  Relocated those obsolete fixture files to ignored `.olympus/oly29-old-golden`
  and regenerated the new fixture. A cross-filesystem rename to `/tmp` failed
  harmlessly before that relocation.
- Verification: Go 1.27.3 build, vet, full tests and evaluator tests pass.
  Tracked Go files are formatted; `gofmt -l .` lists only pre-existing ignored
  reference clone files. Docs relative links/anchors and tracker JSON pass.
  This initial check did not run Docker, a live VM or a full security scan;
  the targeted live follow-up below supersedes the pruning validation limit.

## Targeted lab validation, 2026-10-22

- Branch `fix/oly-29-prune-copied-skills`, product commit `d117c43`.
  Scope: real copy-mode skill removal and preservation of unrecorded volume files.
  Before startup, `ss -ltnp 'sport = :2222'` showed no listener and the process
  inventory showed no QEMU lab. No OpenCode worktree lab was running on that port.
- Started the canonical worktree VM with `lab/lab.sh up`. The VM had Docker
  29.9.1 and Compose v5.6.0 already installed, no current Olympus generation,
  and no running containers. No reset or package installation was needed.
- Loaded all connection/cache settings from `lab/lab.sh env`: loopback port
  2222, user `olympus`, key and known_hosts under this worktree's `.lab/`, and
  `XDG_CACHE_HOME` at `.lab/cache`. Additional inspection commands passed these
  same SSH settings with `-F /dev/null`, `IdentityAgent=none` and strict host-key
  checking. No developer SSH credentials or real dotenv files were used.
- Disposable fixture and command logs: `/tmp/oly29-lab/`. The local `lab-copy`
  recipe uses `supports.skills: {dir: /opt/data/skills, mode: copy}`, a named
  `/opt/data` volume, `nginx -t` health and no secrets, templates or published
  ports. Image pinned to
  `nginx@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10`,
  the existing CLI lab fixture digest. A synthetic lock plus explicit `-recipes`
  override bypassed catalog fetching visibly while retaining the image pin.
- Built this branch with `CGO_ENABLED=0 go build -o /tmp/oly29-lab/olym ./cmd/olym/`.
  CLI commands used `-f /tmp/oly29-lab/stack.yaml`, the fixture recipe directory,
  lab `-known-hosts`, and the same binary as explicit `-activator` for apply/status.
  `plan` exited 0 with one new tool. First `apply --yes` exited 0 with generation
  `sha256-5716e44337daeb3021e31dfbbef4e127ba30e12c97c509b878f8944ae65f7647`.
  `status --json` exited 0 and reported `copycheck` running and healthy.
  Both `alpha/SKILL.md` and `beta/SKILL.md` exactly matched their source files;
  the ownership ledger contained exactly those two paths.
- Created four unrecorded files using `docker exec` in the project container:
  `skills/personal/SKILL.md`, `skills/alpha/custom.txt`, `skills/beta/custom.txt`
  and `/opt/data/notes.txt`. Captured SHA-256 hashes of all four plus the retained
  `alpha/SKILL.md` before editing the stack.
- Changed only the top-level stack selection from
  `skills: [./skills/alpha, ./skills/beta]` to `skills: [./skills/alpha]`.
  Second `plan` exited 0, reporting one tool to restart and changes only to
  the ledger and `beta/SKILL.md`. Second `apply --yes` exited 0 with generation
  `sha256-6d17f650e5dba999b4bb679c32f139060ae189e6bc8963563d800de3282c9872`.
  `status --json` again exited 0, running and healthy. `beta/SKILL.md` was absent;
  all five retained files had byte-identical SHA-256 output before and after.
  The unrecorded file inside the removed skill survived, so its directory
  intentionally remained. The ledger then contained only `alpha/SKILL.md`.
  A third `apply --yes` exited 0 with `No changes.`
- Teardown: confirmed `destroy --delete-data` exited 0; inspection verified no
  Olympus containers, project volumes or `/var/lib/olympus/` remained. Stopped
  the VM started by this run with `lab/lab.sh down`.
- Static/unit front: Go build, `go vet ./...` and `go test ./...` exited 0.
  `gofmt -l .` listed only pre-existing files in ignored reference clones;
  tracked Go files remain formatted. Docs links/anchors and tracker JSON checked.
  Recipe front: the disposable copy-mode recipe validated through real CLI plan
  and apply; no shipped recipe changed. Edge-case front: scoped removal and
  unrecorded-file preservation proved live; remaining cases retain earlier unit
  evidence and were not expanded into a full edge-case pass. Live front: green.
- Security front: no provider credentials, real secrets or provider requests;
  the fixture declares no secrets, so sentinel leakage and secret-mode tests are
  not applicable to this targeted run. Docker reported container port `80/tcp`
  with no host publication. No external exposure probe or full security scan.
  Existing explicit development activator checksum/version verification was
  exercised by apply; release checksum embedding remains outside this test.
- ADR review: no new architecture or behavior decision; observed behavior
  matches ADR 0042. That ADR's legacy-copy and concurrent-writer limits remain
  for review and were not exercised live. No product-code changes in this run.
  Corrected the stale recipe-spec statement about copy-mode deletion.
