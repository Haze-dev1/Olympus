# Aider recipe

Sources checked **2026-11-06** against Aider's official documentation at
<https://aider.chat>, Docker Hub, and PyPI.

## Checked facts

- **Official container image.** Aider publishes official container images on
  Docker Hub: `paulgauthier/aider` (core image) and `paulgauthier/aider-full`
  (includes Playwright browser and extras). The recipe uses `paulgauthier/aider`
  pinned to exact tag `v0.86.2`.
  Sources: <https://aider.chat/docs/install/docker.html> (Docker install guide),
  <https://hub.docker.com/r/paulgauthier/aider/tags>.
- **Version pin.** `v0.86.2` on Docker Hub and `0.86.2` on PyPI (`aider-chat`),
  released 2026.
  Verified with Docker Hub registry API:
  index digest `sha256:0d54037f6c97c85a926ebc3c903e27446f5bca96823d9f68902f1e623289b556`
  covering `linux/amd64` (`sha256:764924922f1f9a47e1185ebaa72e6435027af657deaff3febf0f20a176403c1e`)
  and `linux/arm64` (`sha256:cd375f1ecbd1e0ac6752bba40de5f69f3b36b75cd3d24b2c41fcca502d054ad3`).
  Verified on PyPI: `python3 -m pip index versions aider-chat` and
  `https://pypi.org/pypi/aider-chat/json` → `0.86.2`.
- **Resources.** 4 GB+ RAM is recommended for LLM coding workflows. The recipe
  sets `resources.min_ram: 4GB`.
- **Health.** `aider --version` prints the version (`aider 0.86.2`) and exits 0
  without network access or API credentials (<https://aider.chat/docs/options.html>).
  There is no HTTP endpoint for the CLI, so the recipe declares a command check.
- **Model key.** Aider reads provider credentials from environment variables,
  including `OPENAI_API_KEY` and `ANTHROPIC_API_KEY`
  (<https://aider.chat/docs/config/dotenv.html>). Declared as the `model-key`
  secret delivered as `OPENAI_API_KEY`.
- **State & Workspace.** The upstream image sets `WORKDIR /app`, `HOME=/app`, and
  runs as `USER appuser` (UID 1000). The recipe mounts one named volume at `/app`.
- **No MCP support.** Aider does not natively implement the Model Context Protocol
  as a client (<https://aider.chat>). Context is supplied via built-in slash
  commands (`/read`, `/web`, `/run`) or external wrapper scripts.
- **No skills support.** Aider does not discover or load skills from `.agents/skills`
  or `SKILL.md` directories (<https://aider.chat/docs/usage/conventions.html>).
  Custom instructions are loaded as convention files via `.aider.conf.yml` or `/read`.
- **No supports block.** Because Aider supports neither MCP servers nor skills,
  no `supports` block is declared. `plan` warns if a stack file attempts to assign
  MCP servers or skills to Aider.

## Format gap: image entrypoint override

The official image sets `ENTRYPOINT ["/venv/bin/aider"]` with `WORKDIR /app`.
In Docker Compose, `command: [sleep, infinity]` appends arguments to the image's
entrypoint (`/venv/bin/aider sleep infinity`), which Aider parses as positional
file arguments before attempting an interactive terminal session on `/app`.
Because the Olympus recipe format does not support an `entrypoint` field to
override the container entrypoint to `sleep` or `/bin/sh` for detached idle
execution, this limitation is noted as a format gap and not worked around in Go.

## UNVERIFIED

- **Interactive model request.** No live model inference or session run was
  executed against an active provider API key.
- **Container execution on a real server.** No QEMU lab VM was available on this
  machine, so `docker pull` and container execution were not run against a live daemon.
  Offline generation rendering was verified.
