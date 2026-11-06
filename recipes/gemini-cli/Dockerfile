# Gemini CLI ships a versioned sandbox image for its own `--sandbox` feature,
# not as a maintained base for running the CLI itself, and its documented
# install is an exact npm release. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact npm release verified against registry.npmjs.org, 2026-11-06.
ARG GEMINI_CLI_VERSION=0.67.0
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN npm install -g --prefix /opt/gemini-cli --cache /tmp/gemini-cli-npm-cache "@google/gemini-cli@${GEMINI_CLI_VERSION}" \
 && rm -rf /tmp/gemini-cli-npm-cache \
 && find /opt/gemini-cli -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/gemini-cli/bin:${PATH}"

WORKDIR /root
