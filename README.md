# Olympus

Sets up a whole AI system on your own server from one YAML file: personal agents (Hermes, OpenClaw), coding agents (Claude Code, Codex, OpenCode, pi) and workspaces, with MCP servers, skills and chat channels defined once and shared. A lock file makes the setup reproducible, the way Terraform and Nix are. The command-line tool is `olym`.

**Not** a hosting service, an agent framework or an orchestrator. It installs and connects existing tools on a server you control.

## Install

Not released yet. Planned: `brew install <owner>/tap/olym` (Homebrew, own tap) and `olym` on the AUR. See [docs/TRD.md](docs/TRD.md) section 13.

## Status

Design phase. There is no code yet, so nothing here can be installed or run. The commands and file formats in the docs are proposals.

## Docs

- [PRD](docs/PRD.md): what Olympus is for, who it serves, v1 requirements and release criteria
- [TRD](docs/TRD.md): how it is built: architecture, file formats, activation, security, testing
- [Design doc](docs/architecture/design.md): architecture, example files, security defaults, future ideas
- [Architecture decisions](docs/ADR.md)
- [Landscape research](docs/Sources/landscape.md)
