# OpenCode recipe

Sources checked 2026-10-19 and recorded in `docs/Sources/recipe-research.md`:

- Image, tags, server port, health endpoint, config file and MCP shape:
  OpenCode documentation at <https://opencode.ai/docs/> and the image
  `ghcr.io/anomalyco/opencode`.

## Checked facts

- Image: `ghcr.io/anomalyco/opencode`. Tags `1.0.177` to `1.0.196` exist for
  amd64 and arm64; the recipe pins `1.0.196`, a concrete tag (A7).
- The image runs as root with `/` as the working directory, so paths under
  `/root` are inside the named volumes.
- Server: `opencode serve --hostname 0.0.0.0 --port 4096` (4096 is the default).
  The recipe publishes 4096; Olympus binds it to `127.0.0.1` unless the stack
  requests otherwise (A4).
- Health: `GET /global/health` returns `{"healthy":true,...}`.
- Config: `~/.config/opencode/opencode.json` (JSON). OpenCode rewrites parts of
  its config and its data directory, so the config file is listed under
  `writable` and the data directory is a named volume.
- MCP: top-level key `mcp`; local servers are `{type: "local", command: [...],
  environment: {...}}`, remote servers are `{type: "remote", url, headers}`.
  Environment references use `{env:VAR}`.
- Skills: one folder per skill with `SKILL.md` under
  `~/.config/opencode/skills`. OpenCode only reads skills, so they are mounted
  read-only (`mode: mount`).
- Channels: none for the local server.
- Licence: MIT.
- Secrets are delivered as environment variables and referenced as `{env:VAR}`;
  no secret value is written into the config file (A1).

## UNVERIFIED

- `resources.min_ram: 2GB`: upstream documents no minimum; 2 GB is a
  conservative estimate from the single-binary (Bun) footprint.
- The docs describe v1 while the site advertises a v2 CLI. The pinned tag is v1;
  re-check the `serve` flags and config shape when moving to v2.
- Headless auth provisioning (writing `auth.json` without `/connect`) is
  documented only for GitLab, and the exact per-credential JSON is unverified.
  The recipe therefore expects `ANTHROPIC_API_KEY` in the environment.
- Whether `OPENCODE_SERVER_PASSWORD` is needed for a localhost-only port. It is
  not set here; add it if the port is made public.

## Lab observations, 2026-10-20

The real CLI lifecycle passed on the 6144 MiB Ubuntu 24.04 lab VM with Compose
v5.6.0. The pinned image contains `/bin/sh`, `mkdir` and `cp`; init copied its
JSON config successfully. `serve --hostname 0.0.0.0 --port 4096` ran and the
server's `127.0.0.1:4096/global/health` returned `healthy: true` without a real
provider key or server password. Only the server's loopback port is published.
MCP config kept `{env:GITHUB_TOKEN}` rather than the sentinel value. The shared
skill was readable. A no-op preserved every container ID. Removing only this
tool's MCP list recreated only OpenCode and its init service; rollback succeeded.

The 2 GB resource estimate, future v2 compatibility, headless provider auth JSON
and public-port authentication policy are not settled by this no-inference,
localhost-only run. Real provider requests were not attempted.
