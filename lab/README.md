# Test lab

A disposable local QEMU VM for testing `olym plan`, `apply`, `rollback` and `destroy`. It boots an official cloud image, uses cloud-init for setup, and is reachable over SSH.

## What it touches

Everything lives in `.lab/` at the repository root (git-ignored): the downloaded image, the VM's disk, a lab-only SSH key, the lab's `known_hosts`, and Olympus's cache. Nothing is written to `~/.ssh`, `~/.cache`, network settings, firewall rules or system services. The VM uses QEMU's private user-mode network and is reachable only at `127.0.0.1:2222` from this machine.

## Requirements

- `qemu-system-x86_64` and `qemu-img` for amd64. Arm64 uses `qemu-system-aarch64`, TCG software emulation and AArch64 UEFI firmware. The script checks `/usr/share/edk2/aarch64/QEMU_EFI.fd` and `/usr/share/qemu-efi-aarch64/QEMU_EFI.fd`. Set `OLYMPUS_LAB_UEFI` to the firmware image if it is elsewhere.
- `/dev/kvm` readable and writable by you, for full speed. Without it the lab falls back to software emulation, which is much slower.
- `python3`, `curl`, `ssh`, `ssh-keygen`, and `sha256sum` for Ubuntu or `sha512sum` for Debian.
- About 4 GB of free RAM and a few GB of disk.

## Use

```bash
lab/lab.sh up       # first run downloads and verifies the image before boot
lab/lab.sh status
lab/lab.sh ssh      # a shell on the VM as the sudo user "olympus"
lab/lab.sh reset    # throw the VM away and boot a fresh one
lab/lab.sh down     # clean shutdown through the VM's power button; the disk is kept
```

Set and export `OLYMPUS_LAB_OS` to `ubuntu-24.04` (default), `ubuntu-22.04` or
`debian-12`, and `OLYMPUS_LAB_ARCH` to `amd64` (default) or `arm64`. Use the same
selection for all commands in a run. Shut down the previous guest before changing
the selection when reusing the same ports.

The VM uses a copy-on-write disk over the verified download. `reset` discards
the overlay, and `down` followed by `up` keeps the overlay and SSH host key.
The default guest uses `.lab/`. Other guests use `.lab/<os>-<arch>/` for their
image, overlay, SSH settings and cache. The first boot and deployment can take
substantially longer under software emulation.

## Pointing Olympus at it

`lab/lab.sh env` prints the settings:

```text
OLYMPUS_LAB_HOST=127.0.0.1
OLYMPUS_LAB_PORT=2222
OLYMPUS_LAB_USER=olympus
OLYMPUS_LAB_SSH_KEY=<repo>/.lab/id_ed25519
OLYMPUS_LAB_KNOWN_HOSTS=<repo>/.lab/known_hosts
XDG_CACHE_HOME=<repo>/.lab/cache
```

Tests that use the lab must pass the key and `known_hosts` path to Olympus explicitly and set `XDG_CACHE_HOME`, so no run touches the developer's own SSH files or cache. The VM's user has passwordless sudo and root login is disabled, so tests exercise the non-root `sudo -n` path.

For an arm64 deployment test, export `OLYMPUS_LAB_ARCH=arm64`, reset the VM,
load its settings and use a longer outer timeout.

```bash
lab/lab.sh reset
set -a
source <(lab/lab.sh env)
set +a
OLYMPUS_LAB=1 go test -tags=lab ./internal/deploy -run '^TestLabDeploy$' -v -count=1 -timeout=120m
lab/lab.sh down
```

## Limits

The lab is close to a VPS but not identical. Before a release, run the release criteria once on a real VPS for: firewall and port-exposure checks from outside, a change that blocks SSH, and arm64.

## Real recipe lifecycle

Use the canonical lab checkout when sharing port 2222 between worktrees. The
real-stack test requires 6144 MiB and the private catalog's GitHub credential in
`GITHUB_TOKEN`. Do not put that credential in the stack or the test dotenv.

```bash
~/Projects/Olympus/lab/lab.sh down
OLYMPUS_LAB_MEMORY=6144 ~/Projects/Olympus/lab/lab.sh reset
set -a
source <(~/Projects/Olympus/lab/lab.sh env)
set +a
OLYMPUS_LAB=1 go test -tags=lab ./internal/cli -run '^TestLabRealStack$' -v -count=1 -timeout=50m
```

The test runs the real CLI and catalog update, then explicitly selects a frozen
copy of worktree recipes to exercise unmerged changes with locked registry/base
digests. It creates only sentinel provider/MCP values, a local stdio MCP fixture,
one skill and no channel. It checks all tools' health and config formats, copies
and mounts, no-op and selective container IDs, rollback and delete-data teardown.
It never attempts provider inference or chat login. If it fails before teardown,
inspect the sanitized activation log and reset the VM before a clean rerun.

Both lab tests build a development CLI and activator, so every server command
they run passes `-unsafe-dev-activator`. Do the same for manual runs against the
lab; a development build refuses an unsigned activator without it (rule A12).
`lab/lab.sh` itself never calls `olym`, so it needs no change.
