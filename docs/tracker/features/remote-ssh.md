# Remote SSH layer

- **Status:** In progress (package and tests complete; not yet wired into commands)
- **Milestone:** M1
- **Plan:** [plans/v1.md](../../plans/v1.md)

The SSH layer of the core (`internal/remote`): connect to the server, verify its host key, run commands from argument lists, detect the OS and return the Docker install steps ([TRD](../../TRD.md) sections 3.3, 7.2, 9.2 and 11; [edge cases](../../edgecases/apply.md#server)).

## Scope

- `Dial` with a key file or the SSH agent. Host keys are checked against `~/.ssh/known_hosts` (or `Config.KnownHosts`): known connects, unknown goes to `Config.Confirm` with the SHA256 fingerprint, changed is always an error (A6). A confirmed key is appended to the file (mode 0600, no duplicates) so later connections verify silently. Verification cannot be disabled.
- `Client.Run(ctx, []string, stdin)`: every argument is shell-quoted by `quote`; there is no raw-command API (A2). Secrets travel on stdin, never argv (A1). The returned error never embeds stderr.
- `Sudo(args)` wraps with `sudo -n`; `IsRoot` and `HasPasswordlessSudo` implement TRD 7.2 step 3.
- `ParseOSRelease` is pure and accepts only Ubuntu 22.04, 24.04 and Debian 12; `DetectOS` reads `/etc/os-release`.
- `DockerInstallSteps(OS) []Step` returns the apt repository and Engine/Compose plugin steps as argument lists plus optional stdin. Nothing executes in tests.

Not in this slice: uploads (TRD 3.3 names them; deferred until plan/apply needs them), and any CLI wiring.

## Decisions

- Host-key checking uses `x/crypto/ssh/knownhosts`; the wrapped callback distinguishes unknown (`KeyError.Want` empty, goes to `Confirm`) from changed (`Want` non-empty, always an error), so confirmation can never override a changed key.
- A key accepted through `Confirm` is written with `knownhosts.Line(knownhosts.Normalize(host))`, so a later different key for that host is detected as changed rather than as a fresh unknown host. The append keeps the file at 0600, skips an existing identical line and is not locked: one CLI process connects at a time.
- The apt repository line is written with `tee` and `Step.Stdin`; a shell redirect would need a raw command string (A2). `DockerInstallSteps` returns `nil` for an OS outside the allow-list.
- A missing known_hosts file is created empty, so every host is unknown and goes through `Confirm` instead of failing to load the file.
- A key file takes precedence over `$SSH_AUTH_SOCK`, so a stale agent socket cannot break an explicit key.
- No upload primitive (YAGNI for this slice).
- No ADR change: this implements the module layout and rules already decided in ADR 0013 and 0015 and TRD 3.3, 7.2 and 9.2; it introduces no new durable decision.

## Verification

- `gofmt -l internal/remote` clean; `go vet ./...` clean; `go test ./...` passes.
- Quoting round-trips through `/bin/sh` for spaces, both quote kinds, `$(...)`, backticks, newlines, globs, redirects and `;`/`&&`, both directly and end-to-end through `Run` against an in-process server.
- `/etc/os-release` parsing: Ubuntu 22.04/24.04 and Debian 12 accepted; Ubuntu 20.04, Debian 13, Fedora, a missing version and empty input rejected with the supported list.
- Host keys against an in-process `x/crypto/ssh` server: known connects, unknown prompts with the expected SHA256 fingerprint, unknown without confirm is refused, changed key is refused without calling confirm.
- Confirming an unknown host saves it: the second connection does not prompt, the file holds one normalized entry (including non-22 ports) at mode 0600, and appending the same key twice adds no duplicate.
- After a saved key, a different host key on the same address fails as "changed" without calling `Confirm`.
- Docker apt steps run through `env DEBIAN_FRONTEND=noninteractive`.
- `Run` sends stdin to the command and returns stderr separately while the error text omits it.
- No real server and no network in the tests.

## Limits

- Not wired into `cmd/olym` or any command (out of scope for this slice).
- Password-protected key files are not unlocked in-process; without an agent the parse error is returned, matching the server edge case.
- Uploads and the activator's remote session handling are not covered.
