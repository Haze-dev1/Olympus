# Pi publishes no official container image; its containerization documentation
# (packages/coding-agent/docs/containerization.md) recommends installing the exact
# npm package in your own image. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Exact npm release verified against registry.npmjs.org, 2026-11-06.
ARG PI_VERSION=1.5.0
# BuildKit uses this for image/config timestamps. Normalize installed file
# timestamps too, so a clean rebuild does not depend on the build clock.
ARG SOURCE_DATE_EPOCH=0
RUN npm install -g --prefix /opt/pi --cache /tmp/pi-npm-cache "@earendil-works/pi-coding-agent@${PI_VERSION}" \
 && rm -rf /tmp/pi-npm-cache \
 && find /opt/pi -exec touch -h -d @0 {} + \
 && touch -d @0 /opt /tmp
ENV PATH="/opt/pi/bin:${PATH}"

WORKDIR /root
