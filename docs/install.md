# Installing Olympus

Olympus provides precompiled binaries for macOS and Linux on both x86_64 (`amd64`) and ARM64 (`arm64`).

## 1. Official GitHub Releases

Download the tarball matching your operating system and architecture from [GitHub Releases](https://github.com/Haze-dev1/Olympus/releases):

```sh
# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
VERSION="0.1.0" # or desired version

# Download release archive and checksums
curl -fsSL -O "https://github.com/Haze-dev1/Olympus/releases/download/v${VERSION}/olym_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL -O "https://github.com/Haze-dev1/Olympus/releases/download/v${VERSION}/SHA256SUMS"

# Verify GitHub build provenance before trusting either file (requires gh authentication).
gh attestation verify "olym_${VERSION}_${OS}_${ARCH}.tar.gz" --repo Haze-dev1/Olympus
gh attestation verify SHA256SUMS --repo Haze-dev1/Olympus

# Verify checksum
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --ignore-missing -c SHA256SUMS
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 --ignore-missing -c SHA256SUMS
fi

# Extract and install binary to PATH
tar -xzf "olym_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo install -m 0755 olym olym-activator_linux_amd64 olym-activator_linux_arm64 /usr/local/bin/
```

Verify the installation:

```sh
olym version
```

## 2. Building from Source

Building from source requires Go 1.26 or newer:

```sh
git clone https://github.com/Haze-dev1/Olympus.git
cd Olympus
go build -o /usr/local/bin/olym ./cmd/olym
```

To cross-compile all release targets and produce release archives with embedded activator checksums (rule A12):

```sh
./scripts/build-release.sh 0.1.0
```

Release artifacts and `SHA256SUMS` will be placed in `dist/`.

## 3. Server Deployments & The Linux Activator (Rule A12)

Olympus evaluates your stack locally and manages the target Linux host over SSH using a server-side activator binary (`/var/lib/olympus/bin/olym-<version>`).

Each CLI archive, for both Linux and macOS, includes Linux activators for amd64 and arm64. Keep both `olym-activator_linux_amd64` and `olym-activator_linux_arm64` beside `olym`; Olympus selects the server architecture and checks its embedded release checksum. An explicit `--activator <path>` is also checked. Release builds refuse an activator with no embedded release checksum.

A development build (`olym version` reports `0.1.0-dev`) that does not embed a release checksum has nothing to compare against, so it refuses to upload or run an activator unless you pass `--unsafe-dev-activator`. Only use that flag with a development CLI and an activator you built yourself; it knowingly removes the release trust check (rule A12). Release builds ignore the flag and always verify their embedded checksum.

## 4. Planned Package Managers

- **Homebrew:** `brew install <owner>/tap/olym` (own tap).
- **Arch Linux:** `olym` on the AUR.
