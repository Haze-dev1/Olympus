# Release Process (OLY-34)

Olympus uses a tag-triggered GitHub Actions workflow (`.github/workflows/release.yml`) to cross-compile CLI and activator binaries, compute activator checksums, embed them into CLI binaries according to security rule A12, package release archives, and publish GitHub Releases.

## Manual Steps to Release

Follow these steps to create a release:

1. **Verify integration tests and formatting:**
   ```sh
   go build -o /tmp/olym ./cmd/olym/
   go vet ./...
   gofmt -l . # must print nothing
   TMPDIR=/private/tmp/olym-oc go test ./...
   ```

2. **Verify cross-compilation locally (optional check):**
   ```sh
   ./scripts/build-release.sh 0.1.0
   rm -rf dist
   ```

3. **Ensure working directory is clean and on the integration/main branch.**

4. **Create a version tag:**
   ```sh
   git tag -a v0.1.0 -m "Release v0.1.0"
   ```

5. **Push the tag to GitHub:**
   ```sh
   git push origin v0.1.0
   ```

6. **Monitor the GitHub Actions run:**
   - The workflow `.github/workflows/release.yml` will trigger on the pushed `v*` tag.
   - It will build Linux activator binaries for `amd64` and `arm64`.
   - It computes the SHA-256 checksums of the activator binaries.
   - It cross-compiles the CLI binaries (`darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`), injecting the activator checksums via `-ldflags` (rule A12).
   - Each CLI archive bundles both Linux activators next to `olym`; release clients reject missing embedded checksums.
   - It packages `olym_${VERSION}_${OS}_${ARCH}.tar.gz` and `olym-activator_${VERSION}_linux_${ARCH}.tar.gz`.
   - It emits `SHA256SUMS` and attests each archive and the checksum file using GitHub build provenance.
   - It publishes the GitHub Release with the tarballs, standalone binaries, and `SHA256SUMS`.

7. **Verify the published release:**
   - Browse to `https://github.com/Haze-dev1/Olympus/releases/tag/v0.1.0`.
   - Check that `SHA256SUMS` and all archives are attached.

   - Download each archive and `SHA256SUMS`, then verify each file with the GitHub CLI (authentication required):
     ```sh
     gh attestation verify <file> --repo Haze-dev1/Olympus
     ```
   - Also check the archive against the verified `SHA256SUMS`.
