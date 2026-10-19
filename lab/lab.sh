#!/usr/bin/env bash
# Olympus test lab: one disposable Ubuntu 24.04 VM, run with QEMU user-mode
# networking. Everything it creates lives in .lab/ at the repository root.
# It never writes to ~/.ssh, ~/.cache or any system configuration.
#
#   lab/lab.sh up       download the image once, then boot the VM and wait for SSH
#   lab/lab.sh down     shut the VM down cleanly (its disk is kept)
#   lab/lab.sh reset    stop the VM, discard its disk and boot a fresh one
#   lab/lab.sh ssh ...  run ssh against the VM with the lab key (extra args are passed through)
#   lab/lab.sh status   say whether the VM is running and reachable
#   lab/lab.sh env      print the connection settings tests should use
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
LAB="$ROOT/.lab"
IMAGE_URL=https://cloud-images.ubuntu.com/releases/noble/release
IMAGE=ubuntu-24.04-server-cloudimg-amd64.img
SSH_PORT=${OLYMPUS_LAB_SSH_PORT:-2222}
SEED_PORT=${OLYMPUS_LAB_SEED_PORT:-8099}
VM_USER=olympus
CPUS=${OLYMPUS_LAB_CPUS:-2}
MEMORY=${OLYMPUS_LAB_MEMORY:-4096}
DISK_SIZE=20G

ssh_opts=(-i "$LAB/id_ed25519" -p "$SSH_PORT"
	-o IdentitiesOnly=yes -o UserKnownHostsFile="$LAB/known_hosts"
	-o StrictHostKeyChecking=accept-new -o ConnectTimeout=5 -o LogLevel=ERROR)

die() { echo "lab: $*" >&2; exit 1; }
running() { [ -f "$LAB/qemu.pid" ] && kill -0 "$(cat "$LAB/qemu.pid")" 2>/dev/null; }

need() {
	for cmd in "$@"; do
		command -v "$cmd" >/dev/null || die "$cmd not found; install it first (QEMU: sudo pacman -S qemu-base)"
	done
}

fetch_image() {
	[ -f "$LAB/$IMAGE" ] && return
	echo "lab: downloading $IMAGE (about 600 MB, once)"
	curl -fL --progress-bar -o "$LAB/$IMAGE.part" "$IMAGE_URL/$IMAGE"
	curl -fsS -o "$LAB/SHA256SUMS" "$IMAGE_URL/SHA256SUMS"
	expected=$(awk -v f="*$IMAGE" '$2 == f {print $1}' "$LAB/SHA256SUMS")
	[ -n "$expected" ] || die "no checksum for $IMAGE in SHA256SUMS"
	actual=$(sha256sum "$LAB/$IMAGE.part" | awk '{print $1}')
	[ "$expected" = "$actual" ] || { rm -f "$LAB/$IMAGE.part"; die "checksum mismatch for $IMAGE"; }
	chmod a-w "$LAB/$IMAGE.part"
	mv "$LAB/$IMAGE.part" "$LAB/$IMAGE"
}

write_seed() {
	[ -f "$LAB/id_ed25519" ] || ssh-keygen -q -t ed25519 -N '' -C olympus-lab -f "$LAB/id_ed25519"
	mkdir -p "$LAB/seed"
	cat >"$LAB/seed/user-data" <<EOF
#cloud-config
hostname: olympus-lab
ssh_pwauth: false
disable_root: true
users:
  - name: $VM_USER
    groups: [sudo]
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    lock_passwd: true
    ssh_authorized_keys:
      - $(cat "$LAB/id_ed25519.pub")
EOF
	# The instance ID must stay the same for the life of a disk: a new ID makes
	# cloud-init treat the VM as a new machine and regenerate its SSH host keys.
	[ -f "$LAB/instance-id" ] || echo "olympus-lab-$(date +%s)" >"$LAB/instance-id"
	printf 'instance-id: %s\nlocal-hostname: olympus-lab\n' "$(cat "$LAB/instance-id")" >"$LAB/seed/meta-data"
	: >"$LAB/seed/vendor-data"
}

up() {
	need qemu-system-x86_64 qemu-img python3 ssh ssh-keygen curl sha256sum
	running && { echo "lab: already running"; return; }
	mkdir -p "$LAB" && chmod 700 "$LAB"
	fetch_image
	fresh=0
	if [ ! -f "$LAB/disk.qcow2" ]; then
		qemu-img create -q -f qcow2 -F qcow2 -b "$IMAGE" "$LAB/disk.qcow2" "$DISK_SIZE"
		rm -f "$LAB/known_hosts" "$LAB/instance-id"
		fresh=1
	fi
	write_seed
	# cloud-init reads its seed over HTTP from the host's loopback (10.0.2.2 inside
	# the VM). The server runs only until the VM is reachable.
	python3 -m http.server "$SEED_PORT" --bind 127.0.0.1 --directory "$LAB/seed" >"$LAB/seed.log" 2>&1 &
	seed_pid=$!
	trap 'kill "$seed_pid" 2>/dev/null || true' EXIT
	accel=(-accel tcg)
	[ -r /dev/kvm ] && [ -w /dev/kvm ] && accel=(-accel kvm -cpu host)
	qemu-system-x86_64 -machine q35 "${accel[@]}" -smp "$CPUS" -m "$MEMORY" \
		-drive if=virtio,file="$LAB/disk.qcow2",format=qcow2 \
		-netdev user,id=net0,hostfwd=tcp:127.0.0.1:"$SSH_PORT"-:22 \
		-device virtio-net-pci,netdev=net0 \
		-smbios type=1,serial="ds=nocloud;s=http://10.0.2.2:$SEED_PORT/" \
		-display none -serial file:"$LAB/console.log" \
		-monitor unix:"$LAB/monitor.sock",server,nowait \
		-daemonize -pidfile "$LAB/qemu.pid"
	echo -n "lab: waiting for SSH on 127.0.0.1:$SSH_PORT"
	for _ in $(seq 1 90); do
		if ssh "${ssh_opts[@]}" "$VM_USER@127.0.0.1" true 2>/dev/null; then
			[ "$fresh" = 1 ] && ssh "${ssh_opts[@]}" "$VM_USER@127.0.0.1" cloud-init status --wait >/dev/null 2>&1 || true
			# Flush first-boot writes such as SSH host keys, so a later power-off cannot lose them.
			ssh "${ssh_opts[@]}" "$VM_USER@127.0.0.1" sync 2>/dev/null || true
			echo " ready"
			return
		fi
		echo -n "."
		sleep 2
	done
	echo
	die "VM did not answer on SSH within 3 minutes; see $LAB/console.log"
}

# down presses the VM's power button (ACPI) through the QEMU monitor so Ubuntu
# shuts down cleanly, waits up to a minute, then stops QEMU as a last resort.
# Killing QEMU directly is like pulling the plug and can corrupt the disk.
down() {
	running || { echo "lab: not running"; rm -f "$LAB/qemu.pid"; return; }
	if [ "${1:-}" != fast ] && [ -S "$LAB/monitor.sock" ]; then
		python3 - "$LAB/monitor.sock" <<'PY' || true
import socket, sys
s = socket.socket(socket.AF_UNIX)
s.connect(sys.argv[1])
s.sendall(b"system_powerdown\n")
s.close()
PY
		for _ in $(seq 1 60); do running || break; sleep 1; done
	fi
	if running; then
		kill "$(cat "$LAB/qemu.pid")"
		for _ in $(seq 1 15); do running || break; sleep 1; done
		running && kill -9 "$(cat "$LAB/qemu.pid")"
	fi
	rm -f "$LAB/qemu.pid" "$LAB/monitor.sock"
	echo "lab: stopped"
}

reset() {
	down fast # the disk is discarded next, so a clean shutdown is not needed
	rm -f "$LAB/disk.qcow2" "$LAB/known_hosts" "$LAB/instance-id"
	up
}

status() {
	if ! running; then echo "lab: not running"; return 1; fi
	if ssh "${ssh_opts[@]}" "$VM_USER@127.0.0.1" true 2>/dev/null; then
		echo "lab: running, SSH reachable on 127.0.0.1:$SSH_PORT"
	else
		echo "lab: running, SSH not reachable yet"
		return 1
	fi
}

env_vars() {
	cat <<EOF
OLYMPUS_LAB_HOST=127.0.0.1
OLYMPUS_LAB_PORT=$SSH_PORT
OLYMPUS_LAB_USER=$VM_USER
OLYMPUS_LAB_SSH_KEY=$LAB/id_ed25519
OLYMPUS_LAB_KNOWN_HOSTS=$LAB/known_hosts
XDG_CACHE_HOME=$LAB/cache
EOF
}

case "${1:-}" in
up) up ;;
down) down ;;
reset) reset ;;
ssh) shift; exec ssh "${ssh_opts[@]}" "$VM_USER@127.0.0.1" "$@" ;;
status) status ;;
env) env_vars ;;
*) sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
