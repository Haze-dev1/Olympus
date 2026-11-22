# The official Aider image sets ENTRYPOINT ["/venv/bin/aider"], which runs
# aider on container start and causes "command: [sleep, infinity]" to be treated
# as arguments to aider (exiting immediately without a TTY). Resetting ENTRYPOINT
# allows the idle sleep command to run. BASE_IMAGE is supplied by the generation
# from the digest-pinned recipe image; this Dockerfile is content-addressed.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

ENTRYPOINT []
