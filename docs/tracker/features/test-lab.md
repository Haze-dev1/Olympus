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

## Not done

- The CLI has no setting yet for a custom `known_hosts` path; it lands with the `olym plan` wiring.
- Port exposure from outside, SSH lockout and arm64 still need a VPS run before release.
