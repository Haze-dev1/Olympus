# Olympus — Product Requirements Document (PRD)

| | |
| --- | --- |
| **Status** | Draft for v1 |
| **Date** | 2026-10-19 |
| **Owner** | Devansh (Haze) |
| **Companion** | [TRD.md](TRD.md) covers how it is built. This document is self-contained; links point to sources and deeper detail, not to required reading. |

## 1. Summary

Olympus sets up a whole AI system on a server you own, from one YAML file. You list the tools you want — personal agents such as Hermes and OpenClaw, coding agents such as Claude Code, Codex and OpenCode, workspaces such as OpenSpaces — along with the MCP servers, skills and chat channels they should share. One command makes the server match the file. A lock file makes the result reproducible on any other server, the way Terraform makes infrastructure reproducible and Nix flakes make systems reproducible.

Olympus is **not** a hosting service, an agent framework or an orchestrator. It installs, configures and connects tools that already exist. Once a deploy finishes, it is out of the way: it never sits between a user and their agents.

The command-line tool is `olym` (`olym plan`, `olym apply`, `olym rollback`).

## 2. Background

### 2.1 How people use AI tools now

People who use AI seriously run several tools at once, not one:

- a **personal agent** that lives on a server, remembers context and answers on Telegram, Discord or WhatsApp (Hermes Agent from Nous Research, OpenClaw);
- one or more **coding agents** that edit code and run commands (Claude Code, Codex, OpenCode, Gemini CLI, pi, Goose, Aider);
- sometimes a **shared workspace** where people and agents collaborate (OpenSpaces, an open-source alternative to ChatGPT Spaces).

Each of these needs a machine that stays on, which in practice means a VPS.

### 2.2 What setting this up by hand costs

For every tool, the user repeats the same work:

- Find the install method, the image or binary, the ports and the data directory, which differ per tool and change between versions. Two published guides for Hermes already disagree on its port and start command.
- Write its config in its own format. The same GitHub MCP server must be written four times in four formats: Hermes `config.yaml` under `mcp_servers`, Codex `config.toml`, Claude Code `.mcp.json`, OpenCode `opencode.json`.
- Handle API keys, bot tokens and login tokens, usually by pasting them into config files or shell profiles.
- Remember what was done. Nothing records it, so a second server or a rebuild after a failure starts from zero.

Default setups are often unsafe. Exposed OpenClaw gateways with no authentication have been found on Shodan, and a published deployment guide writes an API key into `.bashrc` in plain text.

### 2.3 What exists today

As of October 2026, no tool sets up a mix of different AI tools from one declarative file.

| Category | Examples | What it lacks |
| --- | --- | --- |
| Managed one-click hosting | OpenClaw Launch, MyClaw, Flux, Railway templates | One agent per deploy; the user does not own the server; nothing is reproducible from a file |
| One-click VPS images and PaaS templates | Hostinger, Contabo, DigitalOcean images; Dokploy templates | Tools run side by side but are not connected; no shared MCP servers or skills |
| Deploy tools for agents | aideploy (OpenTofu, DigitalOcean-only public CLI, beta), ClawHost (Hetzner, MIT) | One agent type per server; no catalog of tools; dashboards rather than a config file |
| Reproducible installs | Hermes NixOS module, openclaw-nix | One module per agent; requires Nix; nothing connects the agents |
| Agent orchestrators | Orloj, kagent, Docker cagent | Define how agents behave, not where they run; assume servers exist |

The gap Olympus fills: **one file, many different tools, connected, reproducible, on a server you own.**

### 2.4 Inspiration

Olympus deliberately copies proven ideas:

| From | Idea taken |
| --- | --- |
| Terraform | `plan` before `apply`; a lock file; one change at a time per target |
| Nix flakes | Pinned inputs; building the whole system from inputs before touching the target; numbered generations and rollback |
| deploy-rs | Activating on the target and rolling back automatically unless the deployer confirms it can still reach the machine |
| Helm | Packages with defaults that users override; release history kept on the target; published schemas for user input |
| Homebrew core, nixpkgs | One reviewed catalog of packages in one repository |

## 3. Goals and non-goals

### 3.1 Goals for v1

| ID | Goal |
| --- | --- |
| G1 | One command turns a YAML file into a running set of AI tools on an existing Linux server. |
| G2 | MCP servers and skills are defined once and given to every tool that supports them, in each tool's own format. |
| G3 | The same stack file and lock file produce the same versions on any server. |
| G4 | Re-running is safe: a change to the file changes only what differs on the server. |
| G5 | Any deploy can be undone with one command, and a failed or interrupted deploy undoes itself. |
| G6 | Adding support for a new tool means writing one recipe file, with no change to the core program. |
| G7 | The default setup is private: no public ports, no secrets in config files, chat bots answer only listed users. |

### 3.2 Non-goals for v1

Each was discussed and left out on purpose. They are candidates for later releases (section 12).

- Creating servers on a cloud provider. v1 sets up a server the user already has.
- Spreading one stack across several servers.
- Agents delegating work to each other.
- An MCP server that lets an AI agent drive Olympus.
- Recipes from any source other than the official recipe catalog.
- Shared memory services, backups and restore of agent data.
- Runtime orchestration: manager and worker agents, task routing.
- A web dashboard, a terminal UI or a hosted service.
- Windows servers; servers other than Ubuntu and Debian.

## 4. Users

| Persona | Situation | Needs | Release |
| --- | --- | --- | --- |
| **The self-hoster** (primary; the author) | Wants Hermes on Telegram, Claude Code and Codex always on, and a workspace, on one VPS, connected | One file, one command, safe defaults, easy rebuild | v1 |
| **The tinkerer** | Already runs one or two agents by hand; comfortable with SSH, YAML and git; tries new agents as they launch | Add or swap a tool in minutes; same MCP servers everywhere | v1 |
| **The recipe author** | Wants Olympus to support a tool they use | A documented recipe format; a way to test a recipe on a throwaway server | v1 |
| **The small team** | Wants the same agent setup for every member | One stack per member from shared presets | Later |
| **The non-technical user** | Wants agents without a terminal | A hosted service or dashboard | Not planned |

Users are assumed to have: a Linux VPS with SSH key access and root or sudo; a terminal on macOS or Linux; API keys or tokens for the tools they choose; git.

## 5. Concepts

| Term | Meaning |
| --- | --- |
| **Stack** | Everything that should run on one server. |
| **Stack file** | `stack.yaml`, written by the user and committed to git. |
| **Recipe** | A small YAML file in the official catalog that says how to install, run and configure one tool. |
| **Lock file** | `stack.lock`, written by Olympus and committed to git. Pins the exact recipe catalog revision and the exact image of every tool. |
| **Generation** | One complete, numbered version of the system on the server. Each successful `apply` makes a new one; `rollback` returns to the previous one. |
| **Plan** | The list of changes `apply` would make, shown before anything changes. |
| **Agent / app** | Entries in the stack file. Agents act on their own (Hermes, Codex); apps support them (OpenSpaces). |
| **Harness** | An agent that talks to users through chat channels (Hermes, OpenClaw). |
| **Channel** | A chat integration for a harness: Telegram in v1, Discord next. |

## 6. User journeys

### 6.1 First setup

1. The user writes `stack.yaml`: the server address, a GitHub MCP server, Hermes with Telegram, Codex and OpenCode.
2. They put the tokens the file refers to (as `${NAME}`) in a git-ignored `.env`.
3. `check` validates the file offline and creates `stack.lock`.
4. `plan` connects to the server and lists what will be created.
5. `apply` installs Docker if needed, starts the tools, checks they are healthy, and confirms the server is still reachable.
6. `status` shows each tool running and healthy. The user opens web UIs through an SSH tunnel and messages Hermes on Telegram.
7. They commit `stack.yaml` and `stack.lock`.

**Time target:** under 15 minutes from a fresh VPS to working agents, excluding image download time.

### 6.2 Change the stack

The user adds an agent or an MCP server to the file, runs `plan` (only the new tool and the changed config files are listed) and `apply` (only those tools restart).

### 6.3 Upgrade and roll back

`update` moves the lock file to the newest recipe catalog and images. The user reviews the lock diff in git, runs `plan` and `apply`. If the new version misbehaves, `rollback` returns the server to the previous generation in one command.

### 6.4 Interrupted deploy

The user's laptop sleeps, or the Wi-Fi drops, during `apply`. Within the confirmation timeout (default 60 seconds) the server returns itself to the previous generation. The next `apply` starts cleanly.

### 6.5 Rebuild on a new server

The user points `server.host` at a new machine, copies `.env`, and runs `apply`. The lock file guarantees the same versions. Agent memory and logins do not move with it in v1 (section 9.6).

### 6.6 Remove everything

`destroy` removes every container Olympus created, after confirmation. Data volumes stay unless the user asks to delete them.

### 6.7 Add a tool to the catalog

A recipe author writes `recipes/<tool>/recipe.yaml`, tests it with a one-tool stack on a throwaway server, and opens a pull request. No change to the core is needed.

## 7. Functional requirements

Priority: **P0** must ship in v1; **P1** should ship in v1; **P2** may slip to the next release.

### 7.1 Stack file

| ID | Requirement | Priority |
| --- | --- | --- |
| F-S1 | The stack file is YAML with the top-level keys `inputs`, `server`, `mcps`, `skills`, `agents`, `apps`. | P0 |
| F-S2 | Secret values are written as `${NAME}` references, resolved from `.env` next to the stack file, then from the environment. | P0 |
| F-S3 | Validation reports every error with file, line, field and the expected value, before connecting to any server. | P0 |
| F-S4 | A published JSON Schema describes the stack file, so editors can validate and autocomplete it. | P1 |
| F-S5 | A missing secret is an error that names the variable and every line that uses it, without printing any value. | P0 |
| F-S6 | Per tool, users can override the image tag (to hold a tool back) and open specific ports publicly; both are off by default. | P1 |

### 7.2 Recipes and the catalog

| ID | Requirement | Priority |
| --- | --- | --- |
| F-R1 | Each supported tool has one recipe in the official catalog, describing its image, resources, volumes, ports, health check, secrets, config files and how it accepts MCP servers, skills and channels. | P0 |
| F-R2 | The whole catalog is pinned as one input at one git revision, recorded in the lock file. | P0 |
| F-R3 | Adding a recipe needs no change to the core program. | P0 |
| F-R4 | Recipes are validated against a published schema. | P1 |
| F-R5 | A recipe may include one post-install command, run inside its own container, for steps nothing else can express. | P1 |

### 7.3 Shared MCP servers, skills and channels

| ID | Requirement | Priority |
| --- | --- | --- |
| F-M1 | MCP servers (local command or remote URL) are defined once; each agent lists the ones it gets. | P0 |
| F-M2 | The core writes each agent's MCP servers into that tool's own config file, in its own format (YAML, TOML or JSON). | P0 |
| F-M3 | Skills (folders with a `SKILL.md`) are listed once and delivered to every agent that accepts them. | P1 |
| F-M4 | A tool that cannot accept an MCP server, skill or channel it was given produces a warning naming it; nothing is silently skipped. | P0 |
| F-C1 | Harnesses can be connected to Telegram with a bot token and a required, non-empty list of allowed users. | P0 |
| F-C2 | Discord, with the same allow-list rule. | P2 |

### 7.4 Commands

| ID | Command | Requirement | Priority |
| --- | --- | --- | --- |
| F-K1 | `check` | Validate the stack file and recipes offline; create the lock file if missing. | P0 |
| F-K2 | `build` | Render the complete system locally for inspection, without a server. | P1 |
| F-K3 | `plan` | Show, per tool and per file, what `apply` would add, change or remove, including changes someone made by hand on the server. Change nothing. | P0 |
| F-K4 | `apply` | Make the server match the file; restart only tools that changed; switch to the new generation only after health checks pass and the CLI confirms it can still reach the server. | P0 |
| F-K5 | `status` | Show each tool's state, health and generation. | P0 |
| F-K6 | `rollback` | Return the server to the previous generation. | P0 |
| F-K7 | `update` | Move the lock file to the newest catalog revision and image versions; change nothing on the server. | P0 |
| F-K8 | `gc` | Delete old generations beyond a configurable number to keep (default 5). | P1 |
| F-K9 | `destroy` | Remove everything Olympus created, after confirmation; keep data volumes unless asked. | P0 |
| F-K10 | All commands | Machine-readable output (`--json`) for `plan` and `status`. | P2 |

### 7.5 Server preparation

| ID | Requirement | Priority |
| --- | --- | --- |
| F-P1 | Connect over SSH with a key or the SSH agent, verifying the server's host key. | P0 |
| F-P2 | Install Docker and the Compose plugin on Ubuntu and Debian if missing. | P0 |
| F-P3 | Refuse unsupported operating systems before changing anything. | P0 |
| F-P4 | Work as root or as a user with sudo. | P0 |
| F-P5 | Warn when the server has less memory than a recipe needs. | P1 |

### 7.6 Installation

| ID | Requirement | Priority |
| --- | --- | --- |
| F-I1 | The command-line tool is called `olym`. | P0 |
| F-I2 | Install on macOS and Linux with Homebrew, from the project's own tap at first: `brew install <owner>/tap/olym`. | P0 |
| F-I3 | Install on Arch Linux from the AUR as `olym`. | P1 |
| F-I4 | Every release is also a plain download from GitHub Releases, with checksums. | P0 |
| F-I5 | `olym --version` prints the version, commit and build date. | P0 |

## 8. Non-functional requirements

### 8.1 Security

| ID | Requirement |
| --- | --- |
| N-S1 | Secret values never appear in the stack file, the lock file, `plan` output, logs, error messages or process arguments visible to other users. |
| N-S2 | Every tool port binds to localhost unless the stack file opens it explicitly. A fresh stack exposes only SSH. |
| N-S3 | Each tool can read only its own secrets; secret files are readable by their owner only. |
| N-S4 | Chat channels without an allow-list are rejected. |
| N-S5 | Olympus changes or removes only what it created; other containers and files on the server are untouched. |
| N-S6 | Any program Olympus runs as root on the server is verified by checksum and version first. |
| N-S7 | A security review of the core and the first recipes happens before the first public release. |

### 8.2 Reliability and safety

| ID | Requirement |
| --- | --- |
| N-R1 | A failed health check, a lost connection, or a change that blocks SSH access ends with the server back on the previous generation, without help from the user's machine. |
| N-R2 | Only one `apply`, `rollback` or `gc` runs on a server at a time; a second one fails and says who holds the lock and since when. |
| N-R3 | Re-running `apply` with no changes makes no changes and restarts nothing. |
| N-R4 | Data volumes survive upgrades, rollbacks and `destroy` unless the user asks otherwise. |

### 8.3 Reproducibility

| ID | Requirement |
| --- | --- |
| N-P1 | The same stack file, lock file and catalog produce byte-identical rendered output every time, on any machine. |
| N-P2 | Changing only a secret value does not change any rendered file; it restarts only the affected tool. |
| N-P3 | Two servers applied from the same stack file and lock file run identical image digests. |

### 8.4 Usability

| ID | Requirement |
| --- | --- |
| N-U1 | Installing `olym` is one package-manager command or one file download, with no language runtime required. |
| N-U2 | Error messages say what went wrong, where (file, line, field or tool), and what to do next. |
| N-U3 | `plan` output fits a terminal: one line per change, details on request. |

### 8.5 Performance

| ID | Requirement |
| --- | --- |
| N-F1 | `check` and `build` finish in under 2 seconds for a stack of 10 tools once the catalog is cached. |
| N-F2 | `plan` finishes in under 10 seconds on a typical VPS. |
| N-F3 | `apply` adds under 30 seconds beyond image downloads, container starts and health checks. |

### 8.6 Compatibility

| ID | Requirement |
| --- | --- |
| N-C1 | CLI runs on Linux and macOS, amd64 and arm64. |
| N-C2 | Servers: Ubuntu 22.04 and 24.04, Debian 12, on amd64 and arm64. |

### 8.7 Maintainability

| ID | Requirement |
| --- | --- |
| N-M1 | A tool's port, path or config-format change upstream is fixed in its recipe alone. |
| N-M2 | Every architecture decision is recorded with the alternatives rejected ([ADR.md](ADR.md)). |

## 9. v1 tool catalog

### 9.1 Order

The first three recipes are three different kinds of tool, chosen to test the recipe format before more are written.

| Order | Tool | Kind | Why |
| --- | --- | --- | --- |
| 1 | Hermes Agent | Harness | Chat channels, MCP in YAML, persistent memory |
| 2 | Codex | Coding agent | MCP in TOML, headless login |
| 3 | OpenCode | Coding agent (open source) | MCP in JSON, skills |
| 4+ | Claude Code, Gemini CLI, OpenClaw, pi, oh-my-pi, Goose, Aider, a Grok bot | Mixed | The major tools users ask for |
| Last | OpenSpaces | Workspace | Its own README says agent features are not production-ready yet |

### 9.2 Recipe acceptance bar

Every catalog recipe must: pin a default image whose digest resolves; bind every port to localhost; declare its secrets and hard-code none; name its data volumes; define a health check; and pass a one-tool deploy on a clean server. Every path, port and image in it must be checked against the tool's own documentation, with the source noted.

### 9.3 Logins for subscription tools

Claude Code and Codex can run on API keys or on a subscription login. API keys are the default. Whether a provider's terms allow a subscription login in an always-on server setup is unverified; each recipe states the question rather than an answer.

### 9.4 Muse and other closed agents

Meta's Muse and similar products run only on their vendors' infrastructure and cannot be deployed. Referencing hosted services that offer an API is a later idea.

### 9.5 Workspace collaboration

Agents appearing inside OpenSpaces or AgentTeams as participants is out of v1.

### 9.6 What reproducibility does not cover

The stack and lock files reproduce the setup, not the data. Agent memory, chat history and coding-agent logins stay on the server they were created on until backups exist (section 12).

## 10. Success metrics and release criteria

### 10.1 v1 release criteria

All must hold on a fresh Ubuntu server:

- [ ] One `apply` brings up Hermes, Codex and OpenCode, all using one shared GitHub MCP server.
- [ ] Hermes answers an allowed Telegram user and ignores everyone else.
- [ ] A second `apply` with no changes reports no changes and restarts nothing.
- [ ] After an upgrade, `rollback` restores the previous images.
- [ ] Killing the CLI during `apply` leaves the server on the previous generation within the confirmation timeout.
- [ ] Two fresh servers applied from the same files run identical image digests.
- [ ] A port scan from outside finds only SSH open.
- [ ] A fourth recipe is added without changing the core.
- [ ] A security review finds no open Critical or High issues.

### 10.2 Metrics after release

| Metric | Target | How measured |
| --- | --- | --- |
| Time from fresh VPS to working stack | Under 15 minutes, excluding downloads | Timed test runs |
| Recipes in the catalog | 10 by end of v1 | Catalog count |
| Recipes contributed by others | 3 within three months of release | Merged pull requests |
| Deploys needing manual repair | 0 in the test suite | Interrupted-deploy tests |

Olympus collects no telemetry; adoption is judged from repository activity and user reports.

## 11. Assumptions and dependencies

- Users have a Linux server with SSH key access and root or sudo.
- Tools publish container images, or can be packaged into one by their recipe.
- Docker and the Docker Compose plugin are available from the distribution's or Docker's package repositories.
- The official recipe catalog is a public git repository the CLI can fetch.
- Container registries hosting the tools' images are reachable from the server.

## 12. Releases

| Release | Contents |
| --- | --- |
| **v1, M1** | Core on one server: check, build, plan, apply with automatic rollback, status, rollback, gc, destroy; lock file |
| **v1, M2** | Recipe format proven on Hermes, Codex and OpenCode |
| **v1, M3** | Shared MCP servers and skills |
| **v1, M4** | Telegram, then Discord |
| **v1, M5** | Remaining catalog recipes |
| **v2** | Agents delegating work to each other (for example Hermes handing a coding task to Codex); a shared folder between agents |
| **v3** | An MCP server for Olympus itself: agents can validate, plan and request an apply that a human approves |
| **Later** | Creating servers on cloud providers; multiple servers per stack; backups and restore; encrypted secrets in git; private network access (Tailscale) and domains; local models; recipes from any git repository; presets; dashboard or terminal UI; hosted version |

## 13. Risks

| Risk | Likelihood | Effect | Mitigation |
| --- | --- | --- | --- |
| Tools change ports, config formats and commands often | High | Recipes break | Lock pins exact versions; fixes land in one recipe; nightly recipe tests later |
| The recipe format is wrong | Medium | Every recipe needs rewriting | Prove it on three different kinds of tool before writing more |
| Scope creep | High | v1 never ships | Non-goals in section 3.2; new ideas go to the later list |
| Competitors add multi-tool deploys | Medium | Less reason to choose Olympus | Differentiate on the catalog, shared MCP servers and skills, rollback, and owning your server |
| Subscription logins break provider terms | Unknown | Users' accounts at risk | Default to API keys; flag it in each recipe |
| A security flaw exposes users' keys or agents | Medium | Real harm | Private defaults, strict secret handling, review before release |
| Server-side activation adds a moving part | Low | Deploys fail in new ways | Covered by interrupted-deploy tests in the release criteria |

## 14. Out of scope, explicitly

Olympus will not: host anyone's agents; decide how agents behave at runtime; store or forward users' prompts or data; modify containers or files it did not create; or phone home.

## 15. Open questions

- [ ] Which chat channels each harness supports beyond Telegram, and their config shape.
- [ ] The exact identity of "pi" and "omp" in the catalog (assumed: the pi coding agent and oh-my-pi).
- [ ] How Claude Code and Codex logins work headless, and what the providers' terms allow.
- [ ] Whether skills are copied into each tool or mounted from one shared folder.

## 16. Sources

- Landscape research with links: [Sources/landscape.md](Sources/landscape.md)
- Decisions and rejected alternatives: [ADR.md](ADR.md)
- Technical design: [TRD.md](TRD.md)
