# Olympus

Sets up a whole AI system on your own server from one YAML file: personal agents (Hermes, OpenClaw), coding agents (Claude Code, Codex, OpenCode, pi) and workspaces, with MCP servers, skills and chat channels defined once and shared. A lock file makes the setup reproducible, the way Terraform and Nix are. The command-line tool is `olym`.

**Not** a hosting service, an agent framework or an orchestrator. It installs and connects existing tools on a server you control.

## Install

Not released yet. Planned: `brew install <owner>/tap/olym` (Homebrew, own tap) and `olym` on the AUR. See [docs/TRD.md](docs/TRD.md) section 13.

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

- [PRD](docs/PRD.md): what Olympus is for, who it serves, v1 requirements and release criteria
- [TRD](docs/TRD.md): how it is built: architecture, file formats, activation, security, testing
- [Design doc](docs/architecture/design.md): architecture, example files, security defaults, future ideas
- [Architecture decisions](docs/ADR.md)
- [Landscape research](docs/Sources/landscape.md)
