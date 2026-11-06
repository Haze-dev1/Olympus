#!/usr/bin/env bash
set -euo pipefail

# scripts/build-release.sh cross-compiles Olympus CLI and Linux activator binaries,
# calculates activator checksums, embeds them into the CLI binaries (rule A12),
# packages release archives and emits SHA256SUMS.

VERSION="${1:-${VERSION:-}}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --exact-match 2>/dev/null || echo "0.1.0-dev")"
fi
VERSION="${VERSION#v}"

DIST_DIR="${DIST_DIR:-dist}"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "Building Olympus release v${VERSION} into ${DIST_DIR}/..."

# 1. Build Linux activator binaries
echo "Building Linux activator binaries..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/Haze-dev1/Olympus/internal/eval.Version=${VERSION}" \
  -o "${DIST_DIR}/olym-activator_linux_amd64" ./cmd/olym

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
  -ldflags="-s -w -X github.com/Haze-dev1/Olympus/internal/eval.Version=${VERSION}" \
  -o "${DIST_DIR}/olym-activator_linux_arm64" ./cmd/olym

# 2. Compute activator SHA-256 digests
if command -v sha256sum >/dev/null 2>&1; then
  SHACMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHACMD="shasum -a 256"
else
  echo "Error: neither sha256sum nor shasum found" >&2
  exit 1
fi

ACT_AMD64_SHA=$($SHACMD "${DIST_DIR}/olym-activator_linux_amd64" | awk '{print $1}')
ACT_ARM64_SHA=$($SHACMD "${DIST_DIR}/olym-activator_linux_arm64" | awk '{print $1}')

echo "Activator SHA-256 (linux/amd64): ${ACT_AMD64_SHA}"
echo "Activator SHA-256 (linux/arm64): ${ACT_ARM64_SHA}"

# 3. Cross-compile CLI binaries embedding the activator release checksums (rule A12)
LDFLAGS="-s -w -X github.com/Haze-dev1/Olympus/internal/eval.Version=${VERSION} -X github.com/Haze-dev1/Olympus/internal/deploy.ReleaseChecksumLinuxAmd64=${ACT_AMD64_SHA} -X github.com/Haze-dev1/Olympus/internal/deploy.ReleaseChecksumLinuxArm64=${ACT_ARM64_SHA}"

echo "Building CLI binaries..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o "${DIST_DIR}/olym_darwin_amd64" ./cmd/olym
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="${LDFLAGS}" -o "${DIST_DIR}/olym_darwin_arm64" ./cmd/olym
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o "${DIST_DIR}/olym_linux_amd64" ./cmd/olym
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="${LDFLAGS}" -o "${DIST_DIR}/olym_linux_arm64" ./cmd/olym

# 4. Package tar.gz archives (TRD section 13.2)
echo "Packaging release archives..."
TMP_ARCHIVE="${DIST_DIR}/.tmp_archive"
rm -rf "$TMP_ARCHIVE"

for os in darwin linux; do
  for arch in amd64 arm64; do
    mkdir -p "${TMP_ARCHIVE}"
    cp "${DIST_DIR}/olym_${os}_${arch}" "${TMP_ARCHIVE}/olym"
    chmod 0755 "${TMP_ARCHIVE}/olym"
    cp "${DIST_DIR}/olym-activator_linux_amd64" "${DIST_DIR}/olym-activator_linux_arm64" "${TMP_ARCHIVE}/"
    tar -czf "${DIST_DIR}/olym_${VERSION}_${os}_${arch}.tar.gz" -C "${TMP_ARCHIVE}" olym olym-activator_linux_amd64 olym-activator_linux_arm64
    rm -rf "${TMP_ARCHIVE}"
  done
done

for arch in amd64 arm64; do
  mkdir -p "${TMP_ARCHIVE}"
  cp "${DIST_DIR}/olym-activator_linux_${arch}" "${TMP_ARCHIVE}/olym-activator"
  chmod 0755 "${TMP_ARCHIVE}/olym-activator"
  tar -czf "${DIST_DIR}/olym-activator_${VERSION}_linux_${arch}.tar.gz" -C "${TMP_ARCHIVE}" olym-activator
  rm -rf "${TMP_ARCHIVE}"
done

# 5. Generate SHA256SUMS file
echo "Generating SHA256SUMS..."
(
  cd "$DIST_DIR"
  $SHACMD olym_* olym-activator_* > SHA256SUMS
)

echo "Release artifacts built successfully in ${DIST_DIR}/:"
ls -lh "${DIST_DIR}"
