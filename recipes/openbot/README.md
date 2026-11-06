# OpenBot recipe

Sources checked 2026-11-06 against release `v0.62.0` (published 2026-11-05):

- `docs/docker.md`, `docs/self-hosted-server.md`, `docker/compose.yaml`,
  `docker/seccomp.json` and `LICENSE` at
  <https://github.com/nightly-labs/openbot/tree/v0.62.0>.
- Image `ghcr.io/nightly-labs/openbot` tag `0.62.0`, read from the GHCR
  registry on 2026-11-06: index digest
  `sha256:e0673c02cf6a9e576a613f2fe2220d76fda64cacc2c563385e460450419dadac`,
  with per-architecture manifests
  `linux/amd64 sha256:c79be0df4d03206c8752d05ac8ce99cc43456f8337a72eb4a628c3bead6d8533`
  and `linux/arm64 sha256:2641189ed6e093c3ecbfbbc133bc9217ea259bed386f274357179702429406c9`.
  The lock file resolves and pins the digest (A7).
- License: PolyForm Noncommercial 1.0.0 (`LICENSE`,
  <https://polyformproject.org/licenses/noncommercial/1.0.0>). Commercial use
  needs a separate license from the vendor.

## Checked facts

- Image: `ghcr.io/nightly-labs/openbot:0.62.0`, `linux/amd64` and
  `linux/arm64`. Tags are `latest`, `<version>`, `v<version>` (releases after
  0.31.0) and `sha-<commit>`. The recipe pins the exact release and never
  `latest`, because OpenBot refuses to open a database that a newer release
  migrated, so a rollback to an older tag does not start.
- State: everything lives in `/data`, the home folder of the image user
  `openbot` (UID 1000, GID 1000): the database and provider runtimes under
  `/data/.config/OpenBot`, agent workspaces under `/data/OpenBot`, the keyring
  and its password file, provider sign-ins and agent tools. Keep `/data` as one
  unit: the encrypted data cannot open without the keyring, and the keyring
  cannot open without its password file. Use one volume for one container at a
  time.
- Network: the container publishes no port. OpenBot connects out to the
  account server, the providers and Signal/TURN; the Team API listens on
  `127.0.0.1` inside the container. Use it from the desktop app, the iPhone app
  or `https://openbot.run/app` after signing in. Bridge networking is enough on
  a public server; `network_mode: host` is faster on a LAN or tailnet but
  removes the container's network isolation, so the recipe does not set it.
- Runtime: upstream `docker/compose.yaml` and the `docker run` example both
  set the same four options, which this recipe renders from `runtime`:
  `security_opt: [seccomp=seccomp.json, no-new-privileges:true]`,
  `shm_size: 1g` (Chromium shared memory; the Docker default of 64 MB is too
  small) and `stop_grace_period: 60s` (OpenBot stops its agents and closes the
  database before it exits).
- Seccomp: `seccomp.json` is copied verbatim from `docker/seccomp.json` at
  `v0.62.0` (SHA-256
  `665289e6a1c41045f55166d0b01993c929f71df78e3a3b264f3252bb8e0b3d9e`). It is
  the Docker default profile from `moby/profiles` with one extra rule that
  allows `clone` and `unshare`, so the Electron sandbox can make user, PID and
  network namespaces. Without it the container stops. Do not replace it with
  `seccomp=unconfined`, `--privileged` or `--no-sandbox`.
- Health: `openbot status` answers over the container's control socket and
  needs no sign-in. It reports the version, the account and the server state.
- Secrets: `XAI_API_KEY`, `CURSOR_API_KEY` and `CLINE_API_KEY` are the only
  provider keys `docs/docker.md` lists as environment variables. Agents inherit
  the environment of OpenBot, so they can read these values. Other providers
  sign in interactively instead of taking a key.
- No shared MCP servers, skills or chat channels: the pinned docs define
  OpenBot's own local skills library and its own channel chats inside the app,
  but no file or folder where Olympus could install shared MCP servers or
  skills, and no channel environment variables. The recipe therefore declares
  no `supports` block, and `plan` warns if a stack file gives it MCP servers,
  skills or channels.

## First sign-in

Signing in is a one-time interactive step and cannot be declarative:

```sh
ssh <server>
docker exec -it olympus-openbot-1 openbot login
```

`openbot login` asks for an email and the code OpenBot sends to it; the server
then appears in that account's server list. `openbot login --claim` is the
alternative for an operator who signs the server in for somebody else. To
connect the phone app, run `docker exec -it olympus-openbot-1 openbot mobile-connect`
and scan the code. Olympus does not automate the sign-in, and the recipe does
not persist credentials outside the `/data` volume.

## Security boundary

The container is the boundary. OpenBot is a development preview: agents run
with `danger-full-access` and `approvalPolicy: never`. They run commands,
change all of `/data`, use the network and control the embedded browser
without per-action confirmations. A recipe or stack file must not mount the
Docker socket, the host home folder or any other host folder that agents must
not change, because nothing inside the container stops them.

The image runs as UID 1000 and carries no SUID file; `no_new_privileges` keeps
it that way. Root owns the release, so agents cannot change the program.

## UNVERIFIED

- No lab run: `lab/lab.sh` needs QEMU, which is not available on this machine,
  so nothing in this recipe has been started against a disposable server. The
  compose rendering is covered by `internal/eval` golden test
  `testdata/golden-openbot`.
- Not tested by upstream either (`docs/docker.md`, "Not confirmed"): `openbot
  login` inside a container, remote use of a container from the app, Podman,
  Kubernetes, rootless Docker, hosts that restrict user namespaces with
  AppArmor (Ubuntu 23.10 or newer), Computer Use and remote desktop in the
  container, and agents with Workspace-only access (Codex bubblewrap may fail
  under the default AppArmor profile).
- The image tag was confirmed through the GHCR registry, not by pulling it;
  Docker is not installed here.
