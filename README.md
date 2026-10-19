# Olympus

Sets up a whole AI system on your own server from one YAML file: personal agents (Hermes, OpenClaw), coding agents (Claude Code, Codex, OpenCode, pi) and workspaces, with MCP servers, skills and chat channels defined once and shared. A lock file makes the setup reproducible, the way Terraform and Nix are. The command-line tool is `olym`.

**Not** a hosting service, an agent framework or an orchestrator. It installs and connects existing tools on a server you control.

## Install

Not released yet. Planned: `brew install <owner>/tap/olym` (Homebrew, own tap) and `olym` on the AUR. See [docs/TRD.md](docs/TRD.md) section 13.

## Status

The first offline slice runs with Go 1.25 or newer:

```sh
go build -o /tmp/olym ./cmd/olym/
/tmp/olym check -f stack.yaml
/tmp/olym build -f stack.yaml -recipes recipes -o .olympus/build
```

`check` verifies secret names in `.env` beside the stack file. `build` loads local recipes and writes Compose, tool files and a hashed manifest without resolving secrets. The sample is in [testdata/sample/](testdata/sample/); provide `GH_TOKEN` in a local `.env` beside its stack file. Its image is fictional and is never pulled.

An identical build can reuse its output directory. For changed inputs, choose a fresh `-o` directory; Olympus leaves existing output intact. SSH, deployment, catalog fetching, digest resolution and lock files are not implemented. These artifacts are for offline inspection, not deployment.

## Docs

- [PRD](docs/PRD.md): what Olympus is for, who it serves, v1 requirements and release criteria
- [TRD](docs/TRD.md): how it is built: architecture, file formats, activation, security, testing
- [Design doc](docs/architecture/design.md): architecture, example files, security defaults, future ideas
- [Architecture decisions](docs/ADR.md)
- [Landscape research](docs/Sources/landscape.md)
