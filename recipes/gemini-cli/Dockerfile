# Gemini CLI ships a versioned sandbox image for its own `--sandbox` feature,
# not as a maintained base for running the CLI itself, and its documented
# install is an exact npm release. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# package.json pins the exact npm release (verified against registry.npmjs.org,
# 2026-11-06); package-lock.json pins every transitive package by integrity
# hash, and `npm ci` refuses to install anything else (OLY-62, ADR 0051).
COPY package.json package-lock.json /opt/gemini-cli/
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN cd /opt/gemini-cli && npm ci --omit=dev --no-audit --no-fund --cache /tmp/gemini-cli-npm-cache \
 && rm -rf /tmp/gemini-cli-npm-cache \
 && find /opt/gemini-cli -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/gemini-cli/node_modules/.bin:${PATH}"

WORKDIR /root
