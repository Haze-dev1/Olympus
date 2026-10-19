# Codex has no documented official container image, so this recipe builds one
# on the server with Compose. BASE_IMAGE is supplied by the generation from the
# digest-pinned recipe image; the Dockerfile itself is pinned by content hash.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# UNVERIFIED: no Codex npm version is documented, so the package version is not
# pinned yet. Pin CODEX_VERSION once a version is checked; the generation tag
# already changes when this file or the base digest changes.
ARG CODEX_VERSION=latest
RUN npm install -g "@openai/codex@${CODEX_VERSION}" \
 && npm cache clean --force

WORKDIR /root
