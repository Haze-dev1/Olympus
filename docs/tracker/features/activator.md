# Activator

- **Status:** In progress (package and tests complete; not yet wired into commands)
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

The server-side activator (`internal/activate`): it takes the lock, installs a generation's secrets, brings it up through Docker Compose, health-checks every tool in parallel, and either makes it current after a confirmation or re-activates the previous generation. It also implements `confirm`, `rollback` and `gc` ([TRD](../../TRD.md) sections 5.4, 7.1, 8.1–8.6, 9.2, 10.1 and 11; [edge cases](../../edgecases/apply.md#during-apply) and [#generations](../../edgecases/apply.md#generations); [ADR 0027](../../ADR.md#0027-activate-a-generation-from-the-store-by-manifest-id)).

## Scope

- The layout under a configurable root (default `/var/lib/olympus`): `store/gen-<manifest id>/`, `generations/<n>` relative symlinks to the store entry, `current` a relative symlink to a numbered generation, `secrets/<tool>/`, `state.json`, `lock`, `lock.json` and `log/<timestamp>-<command>.log`. Directories are created 0700 and files 0600; a symlinked root is refused.
- One exclusive `flock` on `<root>/lock` for the whole of activate, rollback and gc, with `lock.json` naming the holder (pid, user, command, start). A second caller gets `*LockedError` with those fields and does not wait.
- `Activate(ctx, id)` validates the id and the manifest at the trust boundary (`^sha256-[0-9a-f]{64}$`, manifest version 1, `manifest.id == id`, a `compose.yaml`, tool names that pass the stack name rule), installs the secrets payload, runs pull, `up -d --remove-orphans`, and the health checks, then writes `state.json` with `awaiting-confirm`, a 256-bit nonce and a deadline and waits for a matching confirmation. Confirmed: creates the next `generations/<n>` and atomically points `current` at it. Health failure, deadline or a publish error: re-activates the generation `current` points at, or runs `docker compose down` when this was the first generation. A revert that passes returns `*RolledBackError` and records `rolled-back`; one that fails to come back returns `*RollbackFailedError` and records `rollback_failed`, so a caller never reports a rollback that did not happen.
- `Confirm(id, nonce)` writes the confirmation file only when the id and nonce match the waiting activation; otherwise it reports that the server already rolled back or that the confirmation is stale. Stale confirmations are discarded before a new nonce is published.
- `Rollback(ctx)` re-activates the newest generation numbered below `current`, with the same health checks and the same confirm-or-revert protection, then re-points `current` at that existing link. A failure records the generation it tried to activate, not the one it returned to. No previous generation is `ErrNoPrevious`.
- `GC(ctx, keep)` keeps the newest `keep` generations plus `current`, deletes the other links, then deletes only store entries no kept generation references (its own entry plus every `store/<entry>/` path in its Compose file), skipping dot directories such as `store/.incoming`. It reports the removed numbers, entries and bytes. If a kept generation cannot be resolved or its Compose file cannot be read, `GC` deletes nothing and returns an error: collecting on a guess could delete what a kept generation needs.
- Secrets: the payload (`<root>/secrets.incoming`, written by the CLI over stdin) is read and deleted before anything is written; the whole payload is validated first, so a bad entry writes nothing. Each declared secret becomes `secrets/<tool>/<name>` at 0600, and the tool's `environment` file at 0600 (`VAR=value`, sorted) because the offline Compose file's `env_file` points at it. Before a tool's files are replaced, its current values are copied to `secrets/<tool>.previous` (0700/0600); a revert restores that copy before re-activating the previous generation, and a confirmation deletes it. An unknown tool or an undeclared secret name fails the activation before Compose runs (A5). No value appears in `state.json`, the lock file, a command line or a log line (A1).
- `Compose` is the production runner: `docker compose -p olympus -f <file> pull|up -d --remove-orphans|down|exec -T <tool> …`, always an argument list, never a shell string (A2, A11). `Checker` is the production health checker: an HTTP GET to `127.0.0.1:<port><path>` expecting 2xx, or a command run with `docker compose exec` inside the tool (A8), polled every two seconds until the per-tool timeout (default 120 s).
- `Clock`, `Runner`, `HealthChecker` and `HealthSource` are interfaces, so every test runs offline against fakes and a temporary root.

Not in this slice: CLI commands (`activate`, `confirm`, `rollback`, `gc` wiring), uploads, SSH, `systemd-run` and checksum verification (other agents own `internal/cli` and `internal/remote`), secret fingerprints and the server `key` (TRD 8.4), `--force-recreate` for tools whose secrets changed, host-side chown of secret files, fetching a failed tool's last 50 log lines (TRD 11), `bin/` binaries in `gc`, and process supervision outside the process.

## Decisions

- **Health checks are supplied per generation, not read from the manifest.** `generation.Tool` has no health field, so `HealthSource.Health(id)` returns the checks for one generation and `internal/activate` defines `Health` (HTTP port and path, or a command, plus a timeout). Follow-up: add an optional `health` field to `generation.Tool`, filled from `recipe.Health` in `internal/eval`, so `manifest.json` becomes the `HealthSource` and server-side rollback is self-contained. That change amends ADR 0027.
- **The store entry is named after the manifest id** (`store/gen-sha256-…`) and `generations/<n>`/`current` are relative symlinks, which fixes the shorthand in TRD 7.1 for the uploading CLI. Recorded in ADR 0027.
- **`Confirm` writes a file; the activator polls it.** The confirmation is tied to one activation by a random nonce in `state.json`, and the file is deleted before each new nonce is published, so a leftover confirmation can never approve a later generation.
- **A rollback that cannot come back says so.** `RollbackFailedError` wraps both causes (why the new generation failed, why returning failed), `state.json` records `rollback_failed`, and `RolledBackError` is kept for a revert whose health checks passed. `Rollback` records the generation it tried to activate.
- **The previous secret values are kept until the new generation is confirmed.** A copy of each affected tool's secrets lives in `secrets/<tool>.previous` (0700/0600) for the length of the activation: a revert restores it before re-activating the previous generation, a confirmation deletes it. This closes the TRD 8.3/8.4 gap for a tool whose secrets changed; what remains open is a *failed first* activation (no previous values exist) and fingerprint-based change detection, which is what would drive `--force-recreate`.
- **`gc` refuses rather than guesses.** A kept generation that cannot be resolved or whose Compose file cannot be read aborts the collection before anything is deleted.
- **Secrets stay root-owned 0600 on the host.** Chowning them to the tool's container user needs `recipe.User`, which the manifest does not carry; Compose's own secret `uid`/`gid`/`mode` handles the container side.
- **The Compose runner keeps the last 4 KiB of command output** so a pull or start failure names its reason, and health-check commands run with `docker compose exec` rather than `docker exec` so they stay inside the `olympus` project (A8, A11).
- ADR 0027 records the activation contract: activate by manifest id, the server-owned generation links, the lock, the nonce, the health source and the rollback gap.

## Verification

- `go build -o /tmp/olym ./cmd/olym/` — no output.
- `go vet ./...` — no output.
- `go test ./...` — every package `ok`; `internal/activate` `ok … 0.819s` (23 tests).
- `gofmt -l .` — no output.
- `python3 -m json.tool docs/tracker/progress.json` — valid.
- The tests cover: a confirmation in time switches `current` and clears the nonce; deadline expiry rolls back; a health failure rolls back to the previous generation and names the tool; retries until a check passes; a first-generation failure runs `down` and leaves no `current`; a second activation fails with the holder's pid and command; stale, wrong and spent nonces are rejected; rollback with no previous generation is `ErrNoPrevious`; a confirmed rollback re-points `current`; a rollback that fails to come back is `RollbackFailedError` with `rollback_failed`, not a claimed rollback; a rollback records the generation it tried to activate; `gc` keeps referenced entries and generations and reports freed bytes, and deletes nothing when a kept generation is unreadable; a failed activation after a secret change restores the previous values; a confirmed activation keeps only the new values and leaves no copy; secret files are 0600 and the payload is deleted; an undeclared secret is refused before Compose runs; a manifest secret name that leaves its directory is refused; an environment name or a value with a newline is refused before Compose runs; no secret value appears in any file under the root outside `secrets/`; every Compose command is scoped to `-p olympus` and passed as arguments.

## Limits

- Only a tool whose secrets were replaced during the failed activation gets its old values back: a first activation that fails has nothing to restore, and nothing yet detects secret changes for `--force-recreate`.
- No run against a real Docker daemon or the QEMU lab: the Compose runner and the checker are covered by argument-level and fake tests only.
- `HealthSource` has no production implementation until the manifest carries health, so `Activate` and `Rollback` cannot run end to end on a server yet.
- The failure path reports the failing tool and the check's error, not the tool's container logs (TRD 11 asks for the last 50 lines).
- `gc` does not collect old activator binaries from `bin/`, and activation does not write them.
- `--no-confirm` is a CLI decision: the activator always waits, so the CLI must confirm immediately or skip calling it.
- One activation at a time is enforced by `flock`; there is no queue and no timeout for a holder that hangs.

## Merge into development (2026-10-20)

- Reviewed the fixes: `gc` aborts when a kept generation is unreadable; a failed revert returns `RollbackFailedError` with status `rollback_failed`; previous secrets are restored on revert and dropped on confirm; a failed `Rollback` records the generation it tried. Each has a test.
- Merged `development` (catalog wiring, ADR 0026). This ADR is 0027. ADR 0027's consequence about secrets was updated to match the restore behaviour.
- Follow-up: if `installSecrets` fails partway through writing (after validation, so only on I/O errors), the `.previous` copies are not restored before returning. Restore them on that path too.

## Per-tool store retention, 2026-10-20

GC also retains tool hashes referenced by kept manifests (ADR 0030), independent of Compose text references. A regression test protects a manifest-referenced tool entry. Full M1 command/lab integration remains in progress.
