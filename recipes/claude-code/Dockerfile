# Claude Code has no maintained official container image; its dev-container
# documentation ships an example Dockerfile and tells you to install the exact
# npm release into your own image. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact npm release verified against registry.npmjs.org, 2026-11-06.
ARG CLAUDE_CODE_VERSION=2.1.318
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN npm install -g --prefix /opt/claude-code --cache /tmp/claude-code-npm-cache "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
 && rm -rf /tmp/claude-code-npm-cache \
 && find /opt/claude-code -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/claude-code/bin:${PATH}"

WORKDIR /root
