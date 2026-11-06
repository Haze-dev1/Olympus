# Olympus

Sets up a whole AI system on your own server from one YAML file: personal agents (Hermes, OpenClaw), coding agents (Claude Code, Codex, OpenCode, pi) and workspaces, with MCP servers, skills and chat channels defined once and shared. A lock file makes the setup reproducible, the way Terraform and Nix are. The command-line tool is `olym`.

**Not** a hosting service, an agent framework or an orchestrator. It installs and connects existing tools on a server you control.

## Quickstart

### 1. Install from a Release

Download the tarball matching your operating system and architecture from [GitHub Releases](https://github.com/Haze-dev1/Olympus/releases):

```sh
# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
VERSION="0.1.0" # or desired version

# Download release archive and checksums
curl -fsSL -O "https://github.com/Haze-dev1/Olympus/releases/download/v${VERSION}/olym_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL -O "https://github.com/Haze-dev1/Olympus/releases/download/v${VERSION}/SHA256SUMS"

# Verify checksum
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum --ignore-missing -c SHA256SUMS
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 --ignore-missing -c SHA256SUMS
fi

# Extract and install binary to PATH
tar -xzf "olym_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo install -m 0755 olym olym-activator_linux_amd64 olym-activator_linux_arm64 /usr/local/bin/
```

Verify the installation:

```sh
olym version
```

See [docs/install.md](docs/install.md) for full installation and verification details, or build from source with Go 1.26+ via `go build -o /usr/local/bin/olym ./cmd/olym`.

### 2. Write a `stack.yaml`

Define your server, tools, and shared MCP servers in `stack.yaml`:

```yaml
server:
  host: your-server.example.com
  user: root

mcps:
  github:
    command: npx
    args: ["@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: ${GH_TOKEN}

agents:
  hermes:
    recipe: hermes
    mcps: [github]
    secrets:
      model-key: ${ANTHROPIC_KEY}
```

Reference all secrets with `${VARIABLE}`. Store secret values in a `.env` file beside `stack.yaml` and never commit it:

```sh
echo "GH_TOKEN=ghp_your_secret_token" > .env
echo "ANTHROPIC_KEY=sk-ant-your_secret_key" >> .env
```

### 3. Deploy: Check &rarr; Plan &rarr; Apply

Validate your stack and generate `stack.lock`:

```sh
olym check
```

Inspect planned changes on your remote server without applying:

```sh
olym plan
```

Deploy the generation to your server under health-checked, rollback-protected supervision:

```sh
olym apply
```

See [docs/usage/commands.md](docs/usage/commands.md) for all command options and [docs/usage/stack-reference.md](docs/usage/stack-reference.md) for the complete configuration reference. A comprehensive sample is available in [examples/full-stack/stack.yaml](examples/full-stack/stack.yaml).

## Status

M1 runs with Go 1.26 or newer. The core supports check, build, plan, apply, status, rollback, GC and destroy:

```sh
go build -o /tmp/olym ./cmd/olym/
/tmp/olym check -f stack.yaml
/tmp/olym build -f stack.yaml -o .olympus/build
/tmp/olym plan -f stack.yaml -out plan.json
/tmp/olym apply plan.json --yes -f stack.yaml
/tmp/olym status -f stack.yaml --json
/tmp/olym rollback -f stack.yaml
/tmp/olym gc -f stack.yaml -keep 5
/tmp/olym destroy -f stack.yaml
```

`check` verifies secret names in `.env` beside the stack file. `check` or `update` pins catalog revisions and image digests in stack.lock. An explicit `-recipes` selects local development recipes. `build` writes Compose, tool files and a hashed manifest without resolving secrets. The sample is in [testdata/sample/](testdata/sample/); provide `GH_TOKEN` in a local `.env` beside its stack file. Its image is fictional and is never pulled.

Rebuilds stage a complete generation in a sibling directory and atomically replace output owned by Olympus. An existing directory without a valid Olympus manifest is refused. Native atomic publication is supported on Linux and macOS, on amd64 and arm64; an unsupported operation fails without replacing the previous output. Apply uploads verified tool entries and starts a detached server activator, which rolls back unless the CLI reconnects and confirms. `--yes` skips only local approval. Secret rotations recreate only the affected tools. Destroy asks first and keeps volumes and server state unless `--delete-data` is supplied. Unknown SSH hosts require terminal approval; use `-known-hosts` or `OLYM_KNOWN_HOSTS` for an isolated trust file. An explicit `-activator <Linux binary>` selects a development build.

The complete command cycle is tested on the disposable amd64 Ubuntu lab with a pinned test image. Production recipes and the later milestones remain separate work. See [M1 evidence](docs/tracker/features/core-plan-apply.md) for failures, limits and the tagged lab command.

## Docs

- [CLI Commands](docs/usage/commands.md): command-by-command reference
- [Stack Reference](docs/usage/stack-reference.md): field-by-field `stack.yaml` reference
- [Installation](docs/install.md): precompiled release binaries and building from source
- [Release Pipeline](docs/release.md): cross-compilation and release artifacts
- [PRD](docs/PRD.md): what Olympus is for, who it serves, v1 requirements and release criteria
- [TRD](docs/TRD.md): how it is built: architecture, file formats, activation, security, testing
- [Design doc](docs/architecture/design.md): architecture, example files, security defaults, future ideas
- [Architecture decisions](docs/ADR.md)
- [Landscape research](docs/Sources/landscape.md)
