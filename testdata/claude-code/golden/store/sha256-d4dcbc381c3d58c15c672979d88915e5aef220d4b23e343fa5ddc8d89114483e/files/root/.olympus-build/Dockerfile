# Claude Code has no maintained official container image; its dev-container
# documentation ships an example Dockerfile and tells you to install the exact
# npm release into your own image. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# package.json pins the exact npm release (verified against registry.npmjs.org,
# 2026-11-06); package-lock.json pins every transitive package by integrity
# hash, and `npm ci` refuses to install anything else (OLY-62, ADR 0051).
COPY package.json package-lock.json /opt/claude-code/
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN cd /opt/claude-code && npm ci --omit=dev --no-audit --no-fund --cache /tmp/claude-code-npm-cache \
 && rm -rf /tmp/claude-code-npm-cache \
 && find /opt/claude-code -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/claude-code/node_modules/.bin:${PATH}"

WORKDIR /root
