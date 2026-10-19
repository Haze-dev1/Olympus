# Test lab

A disposable Ubuntu 24.04 server for testing `olym plan`, `apply`, `rollback` and `destroy`, running as a local QEMU VM. It stands in for a user's VPS: the same Ubuntu cloud image that providers boot, set up by cloud-init, reachable over SSH.

## What it touches

Everything lives in `.lab/` at the repository root (git-ignored): the downloaded image, the VM's disk, a lab-only SSH key, the lab's `known_hosts`, and Olympus's cache. Nothing is written to `~/.ssh`, `~/.cache`, network settings, firewall rules or system services. The VM uses QEMU's private user-mode network and is reachable only at `127.0.0.1:2222` from this machine.

## Requirements

- QEMU: `sudo pacman -S qemu-base` (Arch). The only host install.
- `/dev/kvm` readable and writable by you, for full speed. Without it the lab falls back to software emulation, which is much slower.
- `python3`, `curl`, `ssh`, `ssh-keygen`, `sha256sum`.
- About 4 GB of free RAM and a few GB of disk.

## Use

```bash
lab/lab.sh up       # first run downloads the image (about 600 MB) and boots; later runs boot in seconds
lab/lab.sh status
lab/lab.sh ssh      # a shell on the VM as the sudo user "olympus"
lab/lab.sh reset    # throw the VM away and boot a fresh one
lab/lab.sh down
```

The VM's disk is a copy-on-write layer over the downloaded image, so `reset` discards every change in about 20 seconds without downloading again.

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

## Limits

The lab is close to a VPS but not identical. Before a release, run the release criteria once on a real VPS for: firewall and port-exposure checks from outside, a change that blocks SSH, and arm64.
