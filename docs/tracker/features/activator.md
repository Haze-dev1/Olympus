# Activator

- **Status:** In progress (package and tests complete; not yet wired into commands)
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

The server-side activator (`internal/activate`): it takes the lock, installs a generation's secrets, brings it up through Docker Compose, health-checks every tool in parallel, and either makes it current after a confirmation or re-activates the previous generation. It also implements `confirm`, `rollback` and `gc` ([TRD](../../TRD.md) sections 5.4, 7.1, 8.1–8.6, 9.2, 10.1 and 11; [edge cases](../../edgecases/apply.md#during-apply) and [#generations](../../edgecases/apply.md#generations); [ADR 0026](../../ADR.md#0026-activate-a-generation-from-the-store-by-manifest-id)).

## Scope

- The layout under a configurable root (default `/var/lib/olympus`): `store/gen-<manifest id>/`, `generations/<n>` relative symlinks to the store entry, `current` a relative symlink to a numbered generation, `secrets/<tool>/`, `state.json`, `lock`, `lock.json` and `log/<timestamp>-<command>.log`. Directories are created 0700 and files 0600; a symlinked root is refused.
- One exclusive `flock` on `<root>/lock` for the whole of activate, rollback and gc, with `lock.json` naming the holder (pid, user, command, start). A second caller gets `*LockedError` with those fields and does not wait.
- `Activate(ctx, id)` validates the id and the manifest at the trust boundary (`^sha256-[0-9a-f]{64}$`, manifest version 1, `manifest.id == id`, a `compose.yaml`, tool names that pass the stack name rule), installs the secrets payload, runs pull, `up -d --remove-orphans`, and the health checks, then writes `state.json` with `awaiting-confirm`, a 256-bit nonce and a deadline and waits for a matching confirmation. Confirmed: creates the next `generations/<n>` and atomically points `current` at it. Health failure, deadline or a publish error: re-activates the generation `current` points at, or runs `docker compose down` when this was the first generation, and returns `*RolledBackError`.
- `Confirm(id, nonce)` writes the confirmation file only when the id and nonce match the waiting activation; otherwise it reports that the server already rolled back or that the confirmation is stale. Stale confirmations are discarded before a new nonce is published.
- `Rollback(ctx)` re-activates the newest generation numbered below `current`, with the same health checks and the same confirm-or-revert protection, then re-points `current` at that existing link. No previous generation is `ErrNoPrevious`.
- `GC(ctx, keep)` keeps the newest `keep` generations plus `current`, deletes the other links, then deletes only store entries no kept generation references (its own entry plus every `store/<entry>/` path in its Compose file), skipping dot directories such as `store/.incoming`. It reports the removed numbers, entries and bytes.
- Secrets: the payload (`<root>/secrets.incoming`, written by the CLI over stdin) is read and deleted before anything is written. Each declared secret becomes `secrets/<tool>/<name>` at 0600, and the tool's `environment` file at 0600 (`VAR=value`, sorted) because the offline Compose file's `env_file` points at it. An unknown tool or an undeclared secret name fails the activation before Compose runs (A5). No value appears in `state.json`, the lock file, a command line or a log line (A1).
- `Compose` is the production runner: `docker compose -p olympus -f <file> pull|up -d --remove-orphans|down|exec -T <tool> …`, always an argument list, never a shell string (A2, A11). `Checker` is the production health checker: an HTTP GET to `127.0.0.1:<port><path>` expecting 2xx, or a command run with `docker compose exec` inside the tool (A8), polled every two seconds until the per-tool timeout (default 120 s).
- `Clock`, `Runner`, `HealthChecker` and `HealthSource` are interfaces, so every test runs offline against fakes and a temporary root.

Not in this slice: CLI commands (`activate`, `confirm`, `rollback`, `gc` wiring), uploads, SSH, `systemd-run` and checksum verification (other agents own `internal/cli` and `internal/remote`), secret fingerprints and the server `key` (TRD 8.4), `--force-recreate` for tools whose secrets changed, host-side chown of secret files, fetching a failed tool's last 50 log lines (TRD 11), `bin/` binaries in `gc`, and process supervision outside the process.

## Decisions

- **Health checks are supplied per generation, not read from the manifest.** `generation.Tool` has no health field, so `HealthSource.Health(id)` returns the checks for one generation and `internal/activate` defines `Health` (HTTP port and path, or a command, plus a timeout). Follow-up: add an optional `health` field to `generation.Tool`, filled from `recipe.Health` in `internal/eval`, so `manifest.json` becomes the `HealthSource` and server-side rollback is self-contained. That change amends ADR 0026.
- **The store entry is named after the manifest id** (`store/gen-sha256-…`) and `generations/<n>`/`current` are relative symlinks, which fixes the shorthand in TRD 7.1 for the uploading CLI. Recorded in ADR 0026.
- **`Confirm` writes a file; the activator polls it.** The confirmation is tied to one activation by a random nonce in `state.json`, and the file is deleted before each new nonce is published, so a leftover confirmation can never approve a later generation.
- **Automatic rollback re-activates the previous generation's services but cannot restore its secret values.** The payload is deleted once installed and nothing keeps fingerprints yet, so a failed generation whose secrets changed leaves the new values on disk. Accepted as a TRD 8.3/8.4 gap for this slice; fingerprints close it.
- **Secrets stay root-owned 0600 on the host.** Chowning them to the tool's container user needs `recipe.User`, which the manifest does not carry; Compose's own secret `uid`/`gid`/`mode` handles the container side.
- **The Compose runner keeps the last 4 KiB of command output** so a pull or start failure names its reason, and health-check commands run with `docker compose exec` rather than `docker exec` so they stay inside the `olympus` project (A8, A11).
- ADR 0026 records the activation contract: activate by manifest id, the server-owned generation links, the lock, the nonce, the health source and the rollback gap.

## Verification

- `go build -o /tmp/olym ./cmd/olym/` — no output.
- `go vet ./...` — no output.
- `go test ./...` — every package `ok`; `internal/activate` `ok … 0.345s` (18 tests).
- `gofmt -l .` — no output.
- `python3 -m json.tool docs/tracker/progress.json` — valid.
- The tests cover: a confirmation in time switches `current` and clears the nonce; deadline expiry rolls back; a health failure rolls back to the previous generation and names the tool; retries until a check passes; a first-generation failure runs `down` and leaves no `current`; a second activation fails with the holder's pid and command; stale, wrong and spent nonces are rejected; rollback with no previous generation is `ErrNoPrevious`; a confirmed rollback re-points `current`; `gc` keeps referenced entries and generations and reports freed bytes; secret files are 0600 and the payload is deleted; an undeclared secret is refused before Compose runs; a manifest secret name that leaves its directory is refused; an environment name or a value with a newline is refused before Compose runs; no secret value appears in any file under the root outside `secrets/`; every Compose command is scoped to `-p olympus` and passed as arguments.

## Limits

- No run against a real Docker daemon or the QEMU lab: the Compose runner and the checker are covered by argument-level and fake tests only.
- `HealthSource` has no production implementation until the manifest carries health, so `Activate` and `Rollback` cannot run end to end on a server yet.
- The failure path reports the failing tool and the check's error, not the tool's container logs (TRD 11 asks for the last 50 lines).
- `gc` does not collect old activator binaries from `bin/`, and activation does not write them.
- `--no-confirm` is a CLI decision: the activator always waits, so the CLI must confirm immediately or skip calling it.
- One activation at a time is enforced by `flock`; there is no queue and no timeout for a holder that hangs.
