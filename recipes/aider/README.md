# Aider recipe

Sources checked **2026-11-06** against Aider's official documentation at
<https://aider.chat>, Docker Hub, PyPI, and GitHub.

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
  without network access or API credentials (<https://aider.chat/docs/config/options.html>, 2026-11-06).
  There is no HTTP endpoint for the CLI, so the recipe declares a command check.
- **Model key.** Aider reads provider credentials from environment variables,
  including `OPENAI_API_KEY` and `ANTHROPIC_API_KEY`
  (<https://aider.chat/docs/config/dotenv.html>, 2026-11-06). Declared as the `model-key`
  secret delivered as `OPENAI_API_KEY`.
- **State & Workspace Layout.**
  - Upstream Dockerfile at tag `v0.86.2` (<https://raw.githubusercontent.com/Aider-AI/aider/v0.86.2/docker/Dockerfile>, 2026-11-06) sets `WORKDIR /app`, `USER appuser` (UID 1000), and `ENV HOME=/app` (with upstream comment: "This puts the container's ~/.aider into the host's project directory (usually host's cwd). That way caches, version checks, etc get stored in the host filesystem not simply discarded every time the container exits.").
  - Config files: `.aider.conf.yml` is searched in CWD (`/app`), git root (`/app`), and user home (`Path.home() / .aider.conf.yml` → `/app/.aider.conf.yml`) (<https://aider.chat/docs/config/aider_conf.html>, 2026-11-06; `aider/main.py`, 2026-11-06).
  - State files: `.aider.chat.history.md` (chat transcript), `.aider.input.history` (input history), and `.aider.tags.cache.v*` (repomap tags cache) default to CWD or git root (`/app`) (<https://aider.chat/docs/config/options.html>, 2026-11-06; `aider/io.py`, `aider/repomap.py`, 2026-11-06).
  - Global state directory: `~/.aider/` stores version checks (`~/.aider/caches/versioncheck`), analytics opt-in (`~/.aider/analytics.json`), and OAuth keys (`~/.aider/oauth-keys.env`). Because `ENV HOME=/app`, `Path.home()` resolves to `/app`, placing `~/.aider/` at `/app/.aider/` (`aider/versioncheck.py`, `aider/analytics.py`, `aider/main.py`, 2026-11-06).
  - All identified state lives inside the `/app` volume; no state lives outside `/app`.
- **Volume Writability.**
  - In the upstream image, `/app` is owned by `appuser:appuser` (`RUN chown -R appuser:appuser /home/appuser /app /venv`).
  - Docker volume initialization copies ownership (UID 1000, GID 1000) and permissions from the target directory in the image into an empty named volume upon creation (<https://docs.docker.com/engine/storage/volumes/>, 2026-11-06).
  - Therefore, the named volume `olympus_aider_app` mounted at `/app` is fully writable by `appuser` (UID 1000).
- **Entrypoint override via build mechanism.**
  - The official image sets `ENTRYPOINT ["/venv/bin/aider"]`. In headless Docker Compose, `command: [sleep, infinity]` is passed as arguments to `/venv/bin/aider`, which Aider parses as positional file arguments before trying to start an interactive chat session on `/app`. Without a TTY, it exits immediately and restarts forever.
  - To fix this, the recipe declares a `build` block (`recipes/aider/Dockerfile`) based on `${BASE_IMAGE}` (`paulgauthier/aider:v0.86.2`) that resets `ENTRYPOINT []` (ADR 0035). This allows `command: [sleep, infinity]` to execute `sleep infinity` directly as the container's PID 1.
- **No MCP support.** Aider does not natively implement the Model Context Protocol
  as a client (<https://aider.chat>, 2026-11-06). Context is supplied via built-in slash
  commands (`/read`, `/web`, `/run`) or external wrapper scripts.
- **No skills support.** Aider does not discover or load skills from `.agents/skills`
  or `SKILL.md` directories (<https://aider.chat/docs/usage/conventions.html>, 2026-11-06).
  Custom instructions are loaded as convention files via `.aider.conf.yml` or `/read`.
- **No supports block.** Because Aider supports neither MCP servers nor skills,
  no `supports` block is declared. `plan` warns if a stack file attempts to assign
  MCP servers or skills to Aider.

## UNVERIFIED

- **Interactive model request.** No live model inference or session run was
  executed against an active provider API key.
- **Container execution on a real server.** No QEMU lab VM was available on this
  machine, so `docker pull` and container execution were not run against a live daemon.
  Offline generation rendering was verified.
- **Runtime Playwright browser installation.** If `--browser` is invoked dynamically
  on the core image, Playwright browser binaries default to `PLAYWRIGHT_BROWSERS_PATH=/home/appuser/pw-browsers`
  (<https://raw.githubusercontent.com/Aider-AI/aider/v0.86.2/docker/Dockerfile>, 2026-11-06),
  which is not mounted in a volume and is lost on container recreate.
