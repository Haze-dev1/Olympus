# Oh My Pi (omp) publishes no maintained container image in a registry; its releases
# publish standalone prebuilt Linux binaries verified with SHA256SUMS.txt.
# BASE_IMAGE is supplied by the generation from the digest-pinned recipe image;
# this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact GitHub release verified against api.github.com/repos/can1357/oh-my-pi/releases/tags/v18.17.0, 2026-11-06.
ARG OMP_VERSION=18.17.0
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
 && ARCH="$(dpkg --print-architecture)" \
 && case "${ARCH}" in \
      amd64) \
        OMP_ARCH="x64" \
        OMP_SHA256="a4ce6001ab1012ca835f4184d675437531bd1ac477a7ba8424ed0b5476e9e4cb" \
        ;; \
      arm64) \
        OMP_ARCH="arm64" \
        OMP_SHA256="a9423fd7a69d869bf514931226b21d4ff55474225f574e2911fec457fc470e68" \
        ;; \
      *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;; \
    esac \
 && curl -fsSLo /usr/local/bin/omp "https://github.com/can1357/oh-my-pi/releases/download/v${OMP_VERSION}/omp-linux-${OMP_ARCH}" \
 && echo "${OMP_SHA256}  /usr/local/bin/omp" | sha256sum -c - \
 && chmod 0755 /usr/local/bin/omp \
 && rm -rf /var/lib/apt/lists/* \
 && touch -d @0 /usr/local/bin/omp

WORKDIR /root
