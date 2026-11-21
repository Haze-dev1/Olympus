# Test lab VM

- **Status:** Done
- **Milestone:** M1 (supports slices 3–5)
- **Decision:** [ADR 0025](../../ADR.md#0025-test-against-a-local-qemu-vm-from-the-ubuntu-cloud-image)

`lab/lab.sh` starts a disposable Ubuntu 24.04 server for testing Olympus against: the official cloud image under QEMU with KVM, cloud-init, a lab-only SSH key and a passwordless-sudo user, reachable only at `127.0.0.1:2222`. Everything lives in `.lab/`.

## Evidence (2026-10-19)

- `lab/lab.sh up` on the owner's Arch machine: downloaded `ubuntu-24.04-server-cloudimg-amd64.img` (621 MB), verified it against Ubuntu's `SHA256SUMS`, booted with KVM and answered on SSH.
- Inside the VM: `ID=ubuntu`, `VERSION_ID="24.04"`; user `olympus` in group `sudo`; `sudo -n true` succeeded; `cloud-init status` was `done`; root disk 19 GB; about 3.9 GB RAM; 2 CPUs; `systemctl is-system-running` reported `running`.
- On the host only `127.0.0.1:2222` was listening; the cloud-init seed server on port 8099 had stopped.
- `lab/lab.sh reset` produced a fresh VM in 13 seconds, and a file created before the reset was gone afterwards.
- `lab/lab.sh down` stopped the VM and `status` then reported it not running (exit 1).
- `~/.ssh/known_hosts` was unchanged (last modified 2026-08-25).
- `bash -n lab/lab.sh` passed. `shellcheck` is not installed, so it was not run.

## Fix: restarting a kept disk (2026-10-19)

The first `up` after `down` failed with "VM did not answer on SSH within 3 minutes". Two causes, both in `lab/lab.sh`:

1. Each `up` wrote a new cloud-init instance ID, so cloud-init treated the restarted VM as new and regenerated its SSH host keys. SSH then refused the changed key ("REMOTE HOST IDENTIFICATION HAS CHANGED"). The instance ID now lives in `.lab/instance-id`, kept for the life of a disk and replaced only with a fresh disk.
2. `down` killed QEMU, like pulling the power plug. Host keys generated seconds earlier were not yet on disk, so after a restart the SSH server failed on every connection ("Connection reset by peer"). `down` now presses the VM's ACPI power button through a QEMU monitor socket and waits for a clean shutdown (killing QEMU only after 60 seconds), and `up` runs `sync` once the VM is reachable.

Evidence: `reset`, then `down` immediately after first boot, then `up`, twice. Shutdowns took 4 and 5 seconds, restarts 9 and 7 seconds; the host key was unchanged and SSH worked after each restart.

## Fix: TestLabM1 after the release activator check (2026-11-21)

- `TestLabM1` failed on `development` at `3e103ef`: `status: exit 1 ... deploy: cannot read activator`. Since OLY-34 a development build given no `-activator` looks for a bundled `olym-activator_linux_<arch>` beside the CLI, and the test passed `-activator` to `apply` but not to `status`. OLY-34 and OLY-61 merged without a lab run, so the break went unnoticed.
- With that fixed, `status --json` failed to parse: the test read stdout and stderr together, and the development-build warning on stderr came before the JSON.
- The test now passes `-activator` to `status`, keeps stderr apart from stdout, checks both for sentinel secrets, and prints stderr on a failed exit. No product code changed.
- Run on Arch (amd64, KVM, Go 1.27.4), fresh `lab/lab.sh reset`: `OLYMPUS_LAB=1 go test -tags=lab ./internal/cli -run '^TestLabM1$'` passed in 115.4 s, covering apply, status, no-op and selective changes, health-failure rollback, CLI kill with autonomous rollback, gc and both destroys. `TestLabDeploy` and `TestLabRealStack` were not run (the real stack needs the private catalog credential).

## Full lab pass on merged development (2026-11-21)

After PRs #37, #44, #45 and #46 merged (`development` at `9af4c04`), on a fresh VM each:

- `TestLabDeploy` passed (127.9 s).
- `TestLabM1` passed (118.1 s).
- `TestLabRealStack` first failed at `status --json`: as in `TestLabM1`, the test read stderr together with stdout, so the development-build warning broke JSON parsing. `apply` had already brought up the real stack. With stderr kept apart (still scanned for sentinels), it passed in 1354.5 s on a 6144 MiB VM: update, check, plan, apply, health of Hermes, Codex (`codex-cli 0.116.0`) and OpenCode, shared MCP and skill checks, no-op apply, selective change, rollback, status and delete-data destroy. Most of the time went on image pulls from GitHub's CDN at about 41 KiB/s; an earlier run the same day finished `apply` in about 4 minutes. Model inference and chat were not attempted.

## Not done

- The CLI has no setting yet for a custom `known_hosts` path; it lands with the `olym plan` wiring.
- Port exposure from outside, SSH lockout and arm64 still need a VPS run before release.
