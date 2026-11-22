#!/usr/bin/env bash
# Exercise lab selection without downloading an image or starting a VM.
set -euo pipefail
lab=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$lab")

for os in ubuntu-24.04 ubuntu-22.04 debian-12; do
	for arch in amd64 arm64; do
		state="$root/.lab/$os-$arch"
		if [ "$os-$arch" = ubuntu-24.04-amd64 ]; then
			state="$root/.lab"
		fi
		settings=$(OLYMPUS_LAB_OS="$os" OLYMPUS_LAB_ARCH="$arch" "$lab/lab.sh" env)
		for setting in "OLYMPUS_LAB_KNOWN_HOSTS=$state/known_hosts" "XDG_CACHE_HOME=$state/cache"; do
			if ! grep -Fxq "$setting" <<<"$settings"; then
				echo "expected $setting for $os/$arch" >&2
				exit 1
			fi
		done
	done
done

default=$(env -u OLYMPUS_LAB_OS -u OLYMPUS_LAB_ARCH "$lab/lab.sh" env)
grep -Fxq "XDG_CACHE_HOME=$root/.lab/cache" <<<"$default"

for invalid in OLYMPUS_LAB_OS=../invalid OLYMPUS_LAB_ARCH=invalid; do
	if env -u OLYMPUS_LAB_OS -u OLYMPUS_LAB_ARCH "$invalid" "$lab/lab.sh" env >/dev/null 2>&1; then
		echo "expected rejection of $invalid" >&2
		exit 1
	fi
done
