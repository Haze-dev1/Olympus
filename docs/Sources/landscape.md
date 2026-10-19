# Landscape research

- **As of:** 2026-10-19
- **Method:** web searches and page reads during brainstorming. Most hosting details come from vendor pages and affiliate reviews, so treat prices and claims as unverified.

No existing tool sets up a mix of different AI tools on one server from one declarative file. The closest projects each cover one slice: hosting one agent, installing one agent reproducibly, or orchestrating agents on infrastructure that already exists.

## Closest prior art

### aideploy

[github.com/belongnet/aideploy](https://github.com/belongnet/aideploy)

- Deploys OpenClaw or Hermes to the user's own cloud and handles backups, diagnostics and teardown.
- CLI commands: `up`, `doctor`, `down <deploy-id>`. Local state in `~/.aideploy/<deploy-id>/access.json`.
- Downloads and runs OpenTofu during `up`. The only live public module is DigitalOcean; other providers are reference modules or web-wizard betas.
- Fleet features (1 to 16 agents, shared Postgres memory, a "Command Center") are only on the paid aideploy.co service.
- Licenses: stack and modules Apache-2.0; CLI FSL-1.1-ALv2 (blocks competing managed services for two years per release).
- Public beta (v0.4.2-beta.5 at the time of reading), 0 stars.

**Takeaway:** validates the idea; the public tool supports one agent type per deploy and has no recipe catalog.

### ClawHost

[github.com/antoinersx/clawhost](https://github.com/antoinersx/clawhost)

- Open-source (MIT, about 360 stars) one-click hosting for OpenClaw or Hermes.
- Calls the Hetzner Cloud API directly, generates a cloud-init script per server, creates a Cloudflare DNS record, and uses Let's Encrypt.
- TypeScript monorepo: Bun, Hono, React, PostgreSQL (Neon), SSH2 for a browser terminal.
- No Terraform.

**Takeaway:** good reference for cloud-init generation and server bootstrap; one agent per server, driven by a dashboard rather than a file.

### Pulumi guides

- [OpenClaw on AWS or Hetzner](https://www.pulumi.com/blog/deploy-openclaw-aws-hetzner/): one VM, cloud-init installs Docker, Node and OpenClaw, then joins Tailscale; secrets in Pulumi ESC.
- [Hermes on Render](https://www.pulumi.com/blog/deploy-a-hermes-agent-with-pulumi/).

Security lessons from the OpenClaw guide, which apply directly to this project:

- Exposed OpenClaw gateways with no auth were found on Shodan; keep gateways private.
- The guide's own script writes an API key into `.bashrc` in plain text, a weakness to avoid.
- Docker group membership is effectively root.

## Managed and one-click hosting

| Provider | Offer |
| --- | --- |
| [OpenClaw Launch](https://openclawlaunch.com/hermes-hosting) | Managed Hermes or OpenClaw, live in about 30 seconds |
| [MyClaw](https://myclaw.ai/) | Managed OpenClaw, Hermes, Claude Code and Codex |
| [Flux](https://hermes.runonflux.com/managed-hermes-hosting) | Managed Hermes |
| [Railway](https://railway.com/deploy/hermes) | One-click Hermes templates |
| [Hostinger](https://www.hostinger.com/ca/tutorials/?p=143805), Contabo, DigitalOcean, Bluehost, NameHero | One-click VPS images for OpenClaw and Hermes; you maintain the server |
| [Dokploy](https://dokploy.com/templates?tag=agents) | Self-hosted PaaS with templates for Hermes, Agent Zero, OpenHands, Paperclip and others |

**Takeaway:** setting up a single agent is a solved, commoditised problem. Connecting several and reproducing the result is not.

## Reproducible installs (Nix)

- Hermes ships an official NixOS module ([docs](https://hermes-agent.nousresearch.com/docs/getting-started/nix-setup)), supported on a best-effort basis.
- OpenClaw: `nix-openclaw` (Home Manager, aimed at personal machines) and the community [openclaw-nix](https://discourse.nixos.org/t/openclaw-nix-hardened-nixos-flake-for-openclaw-deployment/75354) for servers.

**Takeaway:** closest to the reproducibility goal, but one module per agent, nothing connects them, and users must know Nix.

## Agent orchestration and definition

| Project | What it is |
| --- | --- |
| [Orloj](https://www.orloj.dev/) | YAML manifests for agents, policies and workflows; `orlojctl apply`; Apache-2.0. Does not provision servers. |
| [kagent](https://jimmysong.io/en/ai/kagent) | Kubernetes custom resources for agents; CNCF Sandbox. |
| [Docker cagent](https://docs.docker.com/ai/docker-agent/configuration/overview/) | YAML or HCL agent definitions, often described as Compose for agents. |
| Microsoft Agent Framework, Dapr Agents | Frameworks with declarative agent definitions. |

**Takeaway:** these define how agents behave, not where they run. They are candidates to deploy as recipes, not competitors ([ADR 0002](../ADR.md#0002-deploy-and-connect-tools-leave-runtime-orchestration-out)).

## Deployment models we borrow from

Not competitors: tools from other ecosystems whose models Olympus copies. Described from general knowledge on 2026-10-19, not re-checked against current docs.

### Helm

[helm.sh](https://helm.sh/). The Kubernetes package manager, written in Go. A chart is a folder of Kubernetes YAML templates (Go `text/template` plus the Sprig functions) with default `values.yaml`. Each install is a release; every upgrade adds a numbered revision stored in the cluster, and `helm rollback` returns to an earlier one. `helm template` renders locally, `helm lint` validates, and `values.schema.json` validates user values.

**Taken:** recipes as charts, release history kept on the target (generations), `build` and `check` as `template` and `lint`, published JSON Schemas, a cap on kept history ([ADR 0018](../ADR.md#0018-build-structured-config-as-data-and-publish-json-schemas)). **Avoided:** templating YAML as text, Helm's best-known source of bugs, and Kubernetes itself ([ADR 0015](../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)).

### deploy-rs

[github.com/serokell/deploy-rs](https://github.com/serokell/deploy-rs). A Rust tool that deploys NixOS systems and Nix profiles from a flake to remote machines over SSH. It builds locally, copies the result to the target, and activates it there as a new profile generation. Its "magic rollback" makes the target roll itself back unless the deployer reconnects and confirms within a timeout, which protects against changes that break networking or SSH.

**Taken:** build locally then activate remotely ([ADR 0014](../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)), server-side activation with confirm-or-rollback ([ADR 0017](../ADR.md#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms)), and its node and profile structure as the model for multiple servers later. **Avoided:** requiring Nix ([ADR 0004](../ADR.md#0004-use-yaml-for-the-stack-file)).

### Also relevant

- **Terraform / OpenTofu:** Go, HCL, provider plugins over gRPC, a state file with remote backends and locking, `.terraform.lock.hcl`. Taken: plan/apply, the lock file, state locking. Avoided: plugins and a local state file.
- **Nix flakes:** pinned inputs in `flake.lock`, pure evaluation, generations and rollback. Taken: all four ([ADR 0014](../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server), [0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).
- **Colmena, NixOps:** other Nix multi-machine deploy tools.
- **Kamal:** Docker deploys over SSH without Kubernetes.

## Tools mentioned as targets

### Meta Muse

A personal agent from Meta, launched September 2026 ([announcement](https://about.fb.com/news/2026/09/introducing-muse-personal-ai-agent/), [ABC News](https://abcnews.com/Business/metas-muse-ai-agent/story?id=136680507)). It runs each user's tasks on a dedicated virtual machine and keeps working in the background. It is a closed, Meta-hosted product; no self-hostable version or public API was found. The author mentioned a possible SDK; not yet checked.

### ChatGPT Spaces alternatives

- [OpenSpaces](https://github.com/Anil-matcha/open-spaces): MIT, self-hostable workspace for shared pages and conversations. Its README says agent execution is not production-ready.
- [AgentTeams](https://github.com/agentscope-ai/AgentTeams): a manager agent coordinates worker agents in Matrix rooms, with humans able to watch and step in.
- LobeChat Agent Groups: multiple agents in one group; the LobeHub Community License limits commercial use.

## Feasibility of connecting agents through MCP

Checked for the v2 delegation idea (Hermes handing coding work to a coding agent).

| Tool | MCP support | Source |
| --- | --- | --- |
| Hermes | MCP client; servers listed under `mcp_servers` in `~/.hermes/config.yaml`, stdio or HTTP | [Hermes MCP docs](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp) |
| Codex | `codex mcp-server` exposes `codex` (start a task) and `codex-reply` (continue it); labelled experimental. `codex exec` runs headless. | [Codex docs](https://mintlify.com/openai/codex/cli/mcp-server), [guide](https://codex.danielvaughan.com/2026/05/18/codex-cli-as-mcp-server-exposing-agent-capabilities-agents-sdk-multi-agent-delegation/) |
| Claude Code | `claude mcp serve` exposes Claude Code's tools (file edits, commands), not "run this whole task". Delegating a whole task needs a wrapper around headless `claude -p`. | [guide](https://www.ksred.com/claude-code-as-an-mcp-server-an-interesting-capability-worth-understanding/) |

Hermes resource needs differ between sources (2 GB to 8 GB RAM recommended); check upstream docs when writing its recipe.
