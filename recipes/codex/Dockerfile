# Codex has no documented official container image, so this recipe builds one
# on the server with Compose. BASE_IMAGE is supplied by the generation from the
# digest-pinned recipe image; the Dockerfile itself is pinned by content hash.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact npm release verified against registry.npmjs.org, 2026-10-20.
ARG CODEX_VERSION=0.116.0
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN npm install -g --prefix /opt/codex --cache /tmp/codex-npm-cache "@openai/codex@${CODEX_VERSION}" \
 && rm -rf /tmp/codex-npm-cache \
 && find /opt/codex -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/codex/bin:${PATH}"

WORKDIR /root
