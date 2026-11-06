# Goose has no documented official container image, so this recipe builds one
# on the server with Compose. BASE_IMAGE is supplied by the generation from the
# digest-pinned recipe image; the Dockerfile itself is pinned by content hash.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact GitHub release verified against api.github.com/repos/aaif-goose/goose/releases/tags/v1.57.0, 2026-11-06.
ARG GOOSE_VERSION=1.57.0
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
 && ARCH="$(dpkg --print-architecture)" \
 && case "${ARCH}" in \
      amd64) GOOSE_ARCH="x86_64" ;; \
      arm64) GOOSE_ARCH="aarch64" ;; \
      *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/aaif-goose/goose/releases/download/v${GOOSE_VERSION}/goose-${GOOSE_ARCH}-unknown-linux-musl.tar.gz" \
    | tar -xz -C /usr/local/bin \
 && chmod 0755 /usr/local/bin/goose \
 && rm -rf /var/lib/apt/lists/* \
 && touch -d @0 /usr/local/bin/goose

WORKDIR /root
