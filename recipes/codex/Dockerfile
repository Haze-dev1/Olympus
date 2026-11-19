# Codex has no documented official container image, so this recipe builds one
# on the server with Compose. BASE_IMAGE is supplied by the generation from the
# digest-pinned recipe image; the Dockerfile itself is pinned by content hash.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# package.json pins the exact npm release (verified against registry.npmjs.org,
# 2026-10-20); package-lock.json pins every transitive package by integrity
# hash, and `npm ci` refuses to install anything else (OLY-62, ADR 0051).
COPY package.json package-lock.json /opt/codex/
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN cd /opt/codex && npm ci --omit=dev --no-audit --no-fund --cache /tmp/codex-npm-cache \
 && rm -rf /tmp/codex-npm-cache \
 && find /opt/codex -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/codex/node_modules/.bin:${PATH}"

WORKDIR /root
