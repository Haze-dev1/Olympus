# Pi publishes no official container image; its containerization documentation
# (packages/coding-agent/docs/containerization.md) recommends installing the exact
# npm package in your own image. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# package.json pins the exact npm release (verified against registry.npmjs.org,
# 2026-11-06); package-lock.json pins every transitive package by integrity
# hash, and `npm ci` refuses to install anything else (OLY-62, ADR 0051).
# Upstream's containerization guide installs with --ignore-scripts; keep that
# so no transitive package can run install-time egress.
COPY package.json package-lock.json /opt/pi/
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN cd /opt/pi && npm ci --omit=dev --ignore-scripts --no-audit --no-fund --cache /tmp/pi-npm-cache \
 && rm -rf /tmp/pi-npm-cache \
 && find /opt/pi -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/pi/node_modules/.bin:${PATH}"

WORKDIR /root
