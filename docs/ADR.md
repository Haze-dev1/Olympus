# Architecture decision records

Each section records one decision: the context, what was decided, what it costs, and the alternatives that were rejected and why. Read the rejected alternatives before proposing a change.

New ADRs go at the end, take the next number and follow the [template](#template). ADRs are not deleted; a reversed decision gets a new ADR, and the old one is marked "Superseded by".

| ADR | Decision | Status |
| --- | --- | --- |
| [0001](#0001-record-architecture-decisions-as-adrs-in-this-repo) | Record architecture decisions as ADRs in this repo | Accepted |
| [0002](#0002-deploy-and-connect-tools-leave-runtime-orchestration-out) | Deploy and connect tools; leave runtime orchestration out | Accepted |
| [0003](#0003-target-an-existing-server-over-ssh-in-v1) | Target an existing server over SSH in v1 | Accepted |
| [0004](#0004-use-yaml-for-the-stack-file) | Use YAML for the stack file | Accepted |
| [0005](#0005-describe-each-tool-with-a-declarative-yaml-recipe) | Describe each tool with a declarative YAML recipe | Accepted, amended by 0016, 0018 |
| [0006](#0006-keep-official-recipes-in-one-folder-in-this-repo) | Keep official recipes in one folder in this repo | Accepted, source restriction superseded by 0026 |
| [0007](#0007-run-each-tool-in-its-own-docker-container) | Run each tool in its own Docker container | Accepted |
| [0008](#0008-pin-exact-versions-in-stacklock) | Pin exact versions in stack.lock | Accepted, amended by 0016 |
| [0009](#0009-keep-secrets-in-a-git-ignored-env-referenced-by-name) | Keep secrets in a git-ignored .env, referenced by name | Accepted |
| [0010](#0010-expose-nothing-publicly-by-default) | Expose nothing publicly by default | Accepted |
| [0011](#0011-make-apply-converge-to-the-desired-state-tracked-by-container-labels) | Make apply converge to the desired state, tracked by container labels | Superseded by 0014 |
| [0012](#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool) | Define MCP servers and skills once and translate them per tool | Accepted |
| [0013](#0013-write-olympus-in-go) | Write Olympus in Go | Accepted |
| [0014](#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server) | Evaluate the stack locally into a generation, then activate it on the server | Accepted, amended by 0017 |
| [0015](#0015-use-one-docker-compose-project-as-the-runtime) | Use one Docker Compose project as the runtime | Accepted |
| [0016](#0016-pin-the-recipe-set-as-a-flake-style-input) | Pin the recipe set as a flake-style input | Accepted, source restriction superseded by 0026 |
| [0017](#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms) | Activate on the server, and roll back unless the CLI confirms | Accepted |
| [0018](#0018-build-structured-config-as-data-and-publish-json-schemas) | Build structured config as data, and publish JSON Schemas | Accepted |
| [0019](#0019-name-the-project-olympus-with-the-command-olym) | Name the project Olympus, with the command olym | Accepted |
| [0020](#0020-use-goccy-yaml-and-pelletier-toml-for-the-offline-core) | Use goccy YAML and pelletier TOML for the offline core | Accepted |
| [0021](#0021-keep-the-first-generation-contract-offline) | Keep the first generation contract offline | Accepted, output policy superseded by 0022; catalog boundary amended by 0026 |
| [0022](#0022-atomically-replace-owned-local-build-directories) | Atomically replace owned local build directories | Accepted |
| [0023](#0023-hash-the-extracted-catalog-tree-in-a-canonical-form) | Hash the extracted catalog tree in a canonical form | Accepted |
| [0024](#0024-resolve-image-digests-with-the-standard-library) | Resolve image digests with the standard library | Accepted |
| [0025](#0025-test-against-a-local-qemu-vm-from-the-ubuntu-cloud-image) | Test against a local QEMU VM from the Ubuntu cloud image | Accepted |
| [0026](#0026-wire-github-catalogs-and-preserve-local-development-overrides) | Wire GitHub catalogs and preserve local development overrides | Accepted |
| [0027](#0027-activate-a-generation-from-the-store-by-manifest-id) | Activate a generation from the store by manifest id | Accepted, amended by 0028 |
| [0028](#0028-store-health-checks-in-the-generation-manifest) | Store health checks in the generation manifest | Accepted |
| [0029](#0029-verify-deployment-uploads-and-retain-the-activation-result) | Verify deployment uploads and retain the activation result | Accepted |

## 0001. Record architecture decisions as ADRs in this repo

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The project's first decisions were made in a long brainstorming conversation. Several ideas were proposed, cut and re-added there (for example, server provisioning and agent orchestration). Without a record, those questions get reopened every time someone new joins, or every time the author forgets why something was cut.

### Decision

We will record each significant architecture decision as a numbered section in this file, `docs/ADR.md`, using the template at the end. ADRs are never deleted; a reversed decision gets a new ADR, and the old one is marked "Superseded by".

### Consequences

- Rejected options stay visible, with the reason they were rejected.
- Writing an ADR adds a small cost to each decision.
- ADRs can drift from the code if nobody updates their status.

### Alternatives considered

- **Decisions only in the design doc:** the design doc describes the current design, so old reasoning gets overwritten as it changes.
- **Decisions in issue or chat threads:** hard to find later and not versioned with the code.

## 0002. Deploy and connect tools; leave runtime orchestration out

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

Three levels of ambition were considered:

1. **Co-deploy:** install several tools on one server from one file.
2. **Connect:** also wire them together (shared MCP servers and skills, later agent-to-agent calls).
3. **Orchestrate:** also control how agents work together at runtime (who delegates to whom, routing, policies).

Terraform's model is to set things up, not to run them. Orchestration is already the focus of Orloj, kagent, AgentTeams and agent frameworks.

### Decision

We will build levels 1 and 2. Level 3 (orchestration) is out of scope. Orchestrators can be deployed as recipes like any other tool.

### Consequences

- The tool stays a setup tool with a clear boundary: once `apply` finishes, it is not in the request path.
- Users who want manager and worker agents need a separate orchestrator.
- Connecting tools (level 2) still needs per-tool knowledge, which lives in recipes.

### Alternatives considered

- **Co-deploy only:** it is Docker Compose plus provisioning; Dokploy and Coolify templates already do this, so it adds little.
- **Include orchestration:** competes with dedicated frameworks, and puts the tool in every agent's runtime path, which makes it far harder to build and to keep secure.

## 0003. Target an existing server over SSH in v1

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The tool could either create servers on a cloud provider (like Terraform) or set up a server that already exists. The author's pain is setting up a VPS, not buying one. Home servers also need support, and they have no cloud API.

### Decision

We will take a host address and SSH key, and set up that server. Creating servers is left for later and will run as a step before this one.

### Consequences

- No cloud API tokens, provider plugins or infrastructure state files in v1.
- Works the same for any VPS provider and for home servers.
- Users create the server, firewall and DNS by hand, or with their own Terraform.
- Adding provisioning later means a step that creates a server and passes its address to the existing flow.

### Alternatives considered

- **Provision servers through OpenTofu from day one:** adds cloud accounts, tokens and state handling before the core idea is proven. aideploy took this route and its public CLI supports one provider.
- **Call provider APIs directly (as ClawHost does with Hetzner):** ties the tool to specific providers and still needs state tracking.

## 0004. Use YAML for the stack file

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The stack file is what users write and commit. It needs to be easy to read and edit, and close to what users already copy from the tools' own READMEs.

### Decision

We will use YAML for `stack.yaml`, with secrets referenced as `${NAME}`.

### Consequences

- Familiar to anyone who has used Docker Compose, which most of these tools ship with.
- Many editors can validate it against a JSON Schema.
- No loops or functions: repeated blocks must be written out, or handled by features such as `count:` later.
- YAML has known traps (indentation, unquoted values like `no` becoming booleans); validation must report them clearly.

### Alternatives considered

- **HCL:** familiar to Terraform users, but uncommon elsewhere and less tooling outside the HashiCorp ecosystem.
- **TypeScript or Python code (Pulumi style):** most flexible, but the config becomes a program to review and run, which is harder for agents and people to edit safely.
- **Nix:** the strongest reproducibility, but most users do not know the language, which defeats the goal of being easy to use.

## 0005. Describe each tool with a declarative YAML recipe

- **Status:** Accepted, amended by [0016](#0016-pin-the-recipe-set-as-a-flake-style-input) (recipes no longer carry their own version); amended by [0018](#0018-build-structured-config-as-data-and-publish-json-schemas) (structured config built as data)
- **Date:** 2026-10-19

### Context

The tool should support every major agent and harness. It can only do that if adding one is cheap and does not require changing the core, the way Terraform has providers and Nix has packages.

### Decision

We will describe each tool as a recipe: `recipes/<tool>/recipe.yaml` plus config templates. A recipe declares the image, resources, volumes, health check, secrets, templates, and a `supports` block saying how MCP servers, skills and channels map into the tool's config. An optional shell hook covers steps that templates cannot express.

### Consequences

- Contributors can add tools without learning the core's language.
- Recipes are easy to review for security: they are data, plus at most one hook.
- The core must implement every feature recipes can declare, so the recipe schema becomes a compatibility contract.
- Some tools will need the shell hook, which is less safe to review than declarative fields.

### Alternatives considered

- **Recipes as code (TypeScript or Python modules):** more flexible, but every recipe becomes code to audit and a dependency of the core's language. Can be added later if templates hit a wall.
- **Hard-code each tool in the core:** fastest for the first two tools, but every new tool and every upstream change needs a core release.

## 0006. Keep official recipes in one folder in this repo

- **Status:** Accepted; catalog/local development boundary amended by [0026](#0026-wire-github-catalogs-and-preserve-local-development-overrides)
- **Date:** 2026-10-19

### Context

Recipes could live in this repo or be fetched from any git URL. Both are git; the question is who controls quality and how users find recipes.

### Decision

We will keep all official recipes in `recipes/` in this repository, as Homebrew core and nixpkgs do. Fetching recipes from other git URLs is deferred.

### Consequences

- One place to review, test and keep recipes consistent.
- Recipe changes ship through this repo's review process.
- Third parties cannot publish a recipe without a pull request here.
- The repo grows with every supported tool.

### Alternatives considered

- **Any git URL (like Terraform modules and Nix flakes):** lets anyone publish, but makes quality and security review impossible to enforce early on. Planned as an addition once the format is stable.
- **A separate recipes repository:** adds version coordination between two repos before there is a reason to split.

## 0007. Run each tool in its own Docker container

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

Tools need different runtimes (Node, Python, Rust binaries) and keep state in their own directories. One compromised or misbehaving tool should not be able to read another's secrets.

### Decision

We will run each tool as its own container on the server, with its own volumes and only its own secrets mounted.

### Consequences

- Isolation between tools, and clean removal with `destroy`.
- Most tools already publish images or Dockerfiles.
- Tools that expect to call each other as local programs (for example, an MCP server started as a subprocess) cannot do so across containers. Agent-to-agent calls (v2) will need HTTP MCP endpoints on a private Docker network.
- Docker must be installed, and Docker group membership is effectively root on the host.

### Alternatives considered

- **One container for everything:** simpler wiring, but no isolation between tools and one tool's crash or upgrade affects all.
- **Native installs with systemd:** no Docker dependency, but tools' runtimes conflict and clean removal is hard.
- **NixOS modules:** reproducible and isolated, but requires NixOS on the server and Nix knowledge from users.

## 0008. Pin exact versions in stack.lock

- **Status:** Accepted, amended by [0016](#0016-pin-the-recipe-set-as-a-flake-style-input) (the lock pins the recipe input revision, not per-recipe versions)
- **Date:** 2026-10-19

### Context

Reproducibility is a core goal. Image tags such as `latest` or `0.22` can point to different builds over time, and recipes change too.

### Decision

We will write `stack.lock` on the first `apply`, recording each tool's image digest and recipe version. Later applies use the locked versions until the user runs `update`. Users commit `stack.lock` next to `stack.yaml`.

### Consequences

- The same two files give the same versions on any server.
- Upgrades are explicit and show up as a diff in git.
- Users must remember to run `update` to get new versions.
- The lock file covers the setup, not data: agent memory, chat history and logins still need backups.

### Alternatives considered

- **Pin versions only in stack.yaml:** a tag can move; only a digest is exact.
- **No lock, always pull latest:** simple, but two servers set up a day apart can differ, which defeats the goal.

## 0009. Keep secrets in a git-ignored .env, referenced by name

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

A stack needs model API keys, chat bot tokens, GitHub tokens and coding-agent login tokens. `stack.yaml` is meant to be committed and shared; secrets must not be.

### Decision

We will reference secrets in `stack.yaml` as `${NAME}` and read their values from a local `.env` file that git ignores. On `apply`, each tool receives only the secrets its recipe declares, written to files readable only by that tool. Secret values never appear in `plan` output, logs or `stack.lock`.

### Consequences

- `stack.yaml` and `stack.lock` are safe to commit and share.
- A future AI-driven mode can edit the stack file without ever seeing secret values.
- The `.env` file must be copied to each machine that runs `apply`, by hand.
- A lost `.env` means re-collecting every token.

### Alternatives considered

- **Secrets inline in stack.yaml:** the file could not be committed or shared.
- **Encrypted secrets in git (sops and age):** the better long-term answer, but adds key management to v1. Planned for later.
- **Password manager integration:** depends on which manager each user has; planned for later.

## 0010. Expose nothing publicly by default

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

Agent gateways and dashboards often ship with weak or no authentication. Exposed OpenClaw gateways with no auth have been found on Shodan. Users of this tool will put API keys and code access behind these services.

### Decision

We will bind every tool's ports to localhost on the server. Users reach web UIs through an SSH tunnel. Opening a port publicly requires an explicit setting in `stack.yaml`. Chat channels require an allow-list of users.

### Consequences

- A fresh stack has only SSH reachable from outside.
- Reaching a web UI needs an SSH tunnel command, which is less convenient than a URL.
- Webhook-based integrations that need inbound traffic will need an explicit setting.

### Alternatives considered

- **Public ports behind each tool's own login:** relies on every tool's auth being correct, which is the failure seen on Shodan.
- **Tailscale from day one:** gives friendlier URLs, but adds an account and auth key to v1. Planned for later.
- **A reverse proxy with HTTPS and a shared login:** needs a domain and certificate handling in v1.

## 0011. Make apply converge to the desired state, tracked by container labels

- **Status:** Superseded by [0014](#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)
- **Date:** 2026-10-19

### Context

A one-shot installer would be simpler, but users need to add, remove and upgrade tools over time. Terraform tracks what it created in a state file; on a single server, the server itself can hold that record.

### Decision

We will make `apply` compare the file with what is running and change only the differences. Olympus labels every container and volume it creates; those labels are the record of what it manages. `plan` shows the same comparison without changing anything.

### Consequences

- Re-running `apply` is safe and makes no changes when nothing changed.
- No separate state file to lose or keep in sync in v1.
- Containers the user started by hand are left alone, because they lack the labels.
- This does not extend to multiple servers or cloud resources; those will need a real state file.

### Alternatives considered

- **One-shot installer script:** cannot remove or upgrade cleanly, and is not "Terraform-like".
- **Local state file (Terraform style):** needed later for multiple servers, but in v1 it can drift from the server and adds a file to keep safe.

## 0012. Define MCP servers and skills once and translate them per tool

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

Each tool stores MCP servers in a different place and format: Hermes in `config.yaml` under `mcp_servers`, Codex in `config.toml`, Claude Code in `.mcp.json`, OpenCode in `opencode.json`. Skills also live in tool-specific folders. Configuring the same server five times is the main manual work this tool removes.

### Decision

We will define MCP servers under a top-level `mcps:` block and skills under `skills:` in `stack.yaml`. Each agent lists which ones it gets. Each recipe's `supports` block says where and in which format its tool expects them, and the core writes them there.

### Consequences

- One change in `stack.yaml` updates every tool's MCP config.
- Recipes carry the knowledge of each tool's config format, so format changes upstream are fixed in one recipe.
- Tool-specific MCP options that do not map to the shared format need an escape hatch per recipe.
- A tool that does not support MCP or skills produces a warning, so users know it was skipped.

### Alternatives considered

- **Configure MCP servers inside each agent's own block:** repeats the same server per tool, which is the problem being solved.
- **One shared MCP gateway that all tools connect to:** fewer config entries, but adds a running service in the request path; may come later alongside the model gateway idea.

## 0013. Write Olympus in Go

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The tool is a CLI that runs on developers' machines, talks SSH to servers, parses YAML, renders templates and drives Docker. Terraform, OpenTofu, Docker and Docker Compose are all written in Go. The earlier lean towards TypeScript rested on its MCP SDK, which matters only for the v3 MCP mode.

### Decision

We will write Olympus in Go and ship it as a single static binary, as Terraform does.

### Consequences

- Users install one file; no Node or Python runtime on their machine.
- Mature libraries for what the core needs: `golang.org/x/crypto/ssh`, YAML parsing, `text/template`, and the Docker and Compose ecosystem.
- Cross-compiling for Linux, macOS and Windows is built in.
- Contributors need Go; recipes stay YAML, so recipe authors do not.
- The v3 MCP mode will use a Go MCP SDK, which is less established than the TypeScript one.

### Alternatives considered

- **TypeScript:** the best MCP SDK, but users need a Node runtime or a bundled binary, and its SSH and Docker libraries are less used for this kind of tool.
- **Python:** fast to write, but packaging a CLI with its runtime is fragile across user machines.
- **Rust:** single binary too, but slower to write for one developer and fewer libraries in the Docker and Compose space.

## 0014. Evaluate the stack locally into a generation, then activate it on the server

- **Status:** Accepted, amended by [0017](#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms) (activation runs on the server with confirm-or-rollback) and [0021](#0021-keep-the-first-generation-contract-offline) (first offline artifact contract)
- **Date:** 2026-10-19
- **Supersedes:** [0011](#0011-make-apply-converge-to-the-desired-state-tracked-by-container-labels)

### Context

The project takes its reproducibility model from Nix flakes and its workflow from Terraform. ADR 0011 used container labels as the only record of what is installed. That gives no rollback, and `plan` cannot show config-file changes without rendering on the server. Terraform's alternative, a local state file, can drift from the server and has to be kept safe.

### Decision

We will split every change into two phases:

1. **Evaluate (local, pure):** read `stack.yaml`, `stack.lock` and the locked recipes, validate, and render everything into a **generation**: a Compose file, each tool's config files and a manifest. Secret values are not read in this phase, only their names. Each tool's rendered output gets a content hash. The same inputs always produce the same generation.
2. **Activate (server):** under a server-side lock, upload tool outputs whose hash the server lacks to `/var/lib/olympus/store/<hash>/`, write `generations/<n>/` pointing at those store paths, write secret files, run Compose, and run health checks. If they pass, `current` points to `<n>`; if not, `current` stays on the previous generation.

`plan` compares the newly built generation's hashes with the server's `current` manifest. The server's generations are the state. New commands follow from this: `build` (evaluate only), `rollback` (point `current` at the previous generation and re-activate) and `gc` (delete store entries no kept generation uses).

### Consequences

- Rollback is one command, like `nixos-rebuild --rollback`.
- `check` and `build` run without a server, so most logic is testable offline.
- `plan` shows file-level diffs and changes made by hand on the server (drift), because both sides are concrete files.
- A tool restarts only when its hash changes.
- The server keeps old generations and store entries, which use disk until `gc` runs.
- State lives on the server: losing the server loses its generation history, though `stack.yaml` and `stack.lock` still rebuild the current setup.

### Alternatives considered

- **Container labels as the only state (ADR 0011):** no rollback, and config-file changes are invisible until applied.
- **Terraform-style local state file:** can drift from the server, must be stored and locked separately, and gives no rollback. Considered and rejected on 2026-10-19.
- **Render on the server:** `plan` would need the server for everything, and evaluation could not be tested offline.

## 0015. Use one Docker Compose project as the runtime

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

ADR 0007 runs each tool in its own container. Something has to create, recreate and remove those containers when a generation changes. Docker Compose already recreates only the services whose configuration changed, removes services that are no longer listed, and labels its containers.

### Decision

We will render each generation as one Compose project (`docker compose -p olympus`). Activation runs `docker compose up -d --remove-orphans` against the generation's Compose file. Recipes describe services that the core turns into Compose service entries.

### Consequences

- We do not write our own container reconciler.
- Users can inspect a running system with ordinary `docker compose` commands.
- The server needs the Docker Compose v2 plugin; the core installs it with Docker.
- Features Compose cannot express (some health and ordering rules) need workarounds in the core or a recipe hook.
- Only containers in the `olympus` Compose project are touched, which keeps rule A11.

### Alternatives considered

- **Call the Docker API directly:** full control, but we would rebuild what Compose already does: diffing, recreate on change, orphan removal.
- **systemd units per tool:** no Docker Compose dependency, but loses Compose's change detection and makes container networking manual.
- **Kubernetes (k3s):** handles everything, but is heavy for one small server and adds a cluster to operate.

## 0016. Pin the recipe set as a flake-style input

- **Status:** Accepted; catalog/local development boundary amended by [0026](#0026-wire-github-catalogs-and-preserve-local-development-overrides)
- **Date:** 2026-10-19
- **Amends:** [0005](#0005-describe-each-tool-with-a-declarative-yaml-recipe), [0008](#0008-pin-exact-versions-in-stacklock)

### Context

ADR 0005 gave each recipe its own semver, and ADR 0008 locked each recipe version separately. Nix flakes instead pin a whole input, such as `nixpkgs`, to one git revision and a content hash, which is simpler and pins every recipe at once.

### Decision

We will add an `inputs:` block to `stack.yaml`. In v1 it has one input, `recipes`, pointing at this repo's `recipes/` folder. `stack.lock` records the input's git revision and content hash, alongside each tool's image digest. `update` moves the input to a newer revision. Recipes no longer carry their own `version` field. Per ADR 0006, v1 accepts only the official repo as the recipes input.

### Consequences

- One revision pins every recipe, so recipes that change together stay consistent.
- Upgrading one tool's recipe upgrades the whole recipe set; users who need to hold one tool back pin its image instead.
- The CLI must fetch a git revision, so evaluation needs network access the first time and a local cache afterwards.
- Allowing other git URLs later (deferred by ADR 0006) only needs more inputs, not a new mechanism.

### Alternatives considered

- **Per-recipe semver (ADR 0005 as first written):** many versions to coordinate and a registry to resolve them.
- **Recipes bundled into the CLI binary:** no fetching, but every recipe fix needs a CLI release.
- **Vendoring recipes into the user's repo:** fully offline, but users stop getting fixes.

## 0017. Activate on the server, and roll back unless the CLI confirms

- **Status:** Accepted
- **Date:** 2026-10-19
- **Amends:** [0014](#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)

### Context

ADR 0014 left the decision to switch generations or roll back with the CLI on the user's machine. If the SSH connection drops mid-`apply`, or the new generation breaks the user's access to the server, the CLI cannot finish either path and the server is left between generations. [deploy-rs](https://github.com/serokell/deploy-rs) solves the same problem for NixOS with "magic rollback": the server rolls itself back unless the deployer reconnects and confirms in time.

### Decision

We will run activation on the server. `apply` uploads the matching Linux build of the Olympus binary to `/var/lib/olympus/bin/` (if that version is not already there) and starts it as `olym activate <n>`, detached from the SSH session. The activator:

1. takes the server lock, writes secret files, runs `docker compose up` for generation `<n>`, and runs health checks;
2. on a failed health check, re-activates the previous generation and exits;
3. on success, waits for confirmation. The CLI opens a **new** SSH connection and runs `olym confirm <n>`, which proves SSH still works after the change;
4. without confirmation before the timeout (default 60 seconds), re-activates the previous generation;
5. with confirmation, points `current` at `<n>` and releases the lock.

`rollback` uses the same activator. `apply --no-confirm` skips step 3 for unattended runs.

### Consequences

- A dropped connection, a crashed laptop or a change that locks the user out ends in a known state: the previous generation, restored by the server.
- The activator's logic runs in one place, which the future multi-server mode can reuse.
- The server runs a copy of the Olympus binary; releases must ship Linux builds for amd64 and arm64, and its version must match the CLI's.
- Every `apply` takes at least as long as the health checks plus one reconnect.
- Activation logs live on the server (`/var/lib/olympus/log/`); the CLI streams them while connected.

### Alternatives considered

- **CLI-driven activation (ADR 0014 as first written):** leaves the server between generations when the connection drops.
- **A shell script as the activator:** no binary to ship, but the lock, health checks, rollback and timeout logic would exist twice, once in Go and once in shell.
- **A long-running agent daemon on the server:** handles confirmations too, but adds a service that must stay up, be updated and be secured, for something needed only during deploys.

## 0018. Build structured config as data, and publish JSON Schemas

- **Status:** Accepted
- **Date:** 2026-10-19
- **Amends:** [0005](#0005-describe-each-tool-with-a-declarative-yaml-recipe)

### Context

Helm generates Kubernetes YAML by templating text, and its best-known problems come from that: indentation bugs, `nindent` workarounds and quoting errors. Most of what Olympus renders is structured: the Compose file and MCP server entries in YAML, TOML or JSON. Helm's `values.schema.json` also shows the value of a published schema for user input.

### Decision

We will build the Compose file and every config written through a recipe's `supports` block as data structures in Go, then encode them with a YAML, TOML or JSON encoder. Recipe `templates` remain for files that have no structured form; templates never produce YAML, TOML or JSON for keys the core already manages. We will publish JSON Schemas for `stack.yaml` and `recipe.yaml`, use them in `check`, and reference them so editors can validate and autocomplete. The server keeps the last N generations (`keep`, default 5); `gc` removes older ones.

### Consequences

- No indentation or quoting bugs in generated config, and output is byte-stable, which ADR 0014's hashes need.
- The core needs an encoder per format (YAML, TOML, JSON), and recipes must describe structure (`supports`) rather than paste raw config.
- Users get editor validation and autocomplete from the schemas.
- Schemas must be kept in step with the specs; `check` uses them, so drift shows up as failing validation.

### Alternatives considered

- **Text templates for everything (Helm's approach):** flexible, but brings its known bugs and makes outputs less stable.
- **Validation only in Go code, no published schema:** one less artifact, but users get no editor support and recipe authors have no machine-readable contract.

## 0019. Name the project Olympus, with the command olym

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The project started under the working name "AITerraform". Terraform is a HashiCorp trademark, so a name built on it suggests an affiliation that does not exist. The command name also appears in every user flow, in server paths and in package names, so it had to be settled before code.

### Decision

The project is **Olympus**. The command and binary are **`olym`** (`olym plan`, `olym apply`, `olym rollback`). Server files live in `/var/lib/olympus/`, and the Compose project is named `olympus`. Packages are published as `olym` on Homebrew, through the project's own tap at first, and on the AUR.

### Consequences

- Every path, project name and example in the docs uses the new names; the repository folder name on disk is unchanged and does not matter to users.
- A short command is quick to type in every flow.
- An own Homebrew tap needs no approval but must be maintained; moving to homebrew-core later requires meeting its notability rules.
- The `olym` name still has to be checked as free in Homebrew and the AUR before the first release.

### Alternatives considered

- **Keep "AITerraform":** implies an affiliation with HashiCorp's Terraform and invites trademark trouble.
- **`olympus` as the command:** clear, but long to type in every command.
- **homebrew-core from the first release:** needs a track record the project does not have yet.

## 0020. Use goccy YAML and pelletier TOML for the offline core

- **Status:** Accepted, explicitly approved by the user
- **Date:** 2026-10-19

### Context

Go has no standard YAML or TOML parser. Offline builds need YAML positions for safe diagnostics and deterministic encoders for YAML, TOML and JSON. Dependencies require explicit approval.

### Decision

Pin `github.com/goccy/go-yaml v1.19.2` for YAML syntax trees and encoding, and `github.com/pelletier/go-toml/v2 v2.4.3` for TOML decoding and encoding. Both were approved at these exact versions before installation. Keep JSON, flags, templates, hashing and tests in the standard library.

Maintenance evidence checked on 2026-10-19: [goccy v1.19.2](https://github.com/goccy/go-yaml/releases/tag/v1.19.2) fixes an anchor-reference regression; [pelletier v2.4.3](https://github.com/pelletier/go-toml/releases/tag/v2.4.3), released 2026-07-05, includes correctness fixes and nesting limits. Both modules declare Go 1.21 and no dependencies. Olympus requires Go 1.25 for confined filesystem APIs.

The published input schemas are embedded. A small validator evaluates only their current vocabulary, retaining YAML positions and adding semantic checks in `stack` and `recipe`. It is not a general JSON Schema implementation. New schema keywords require validator support and tests. YAML is decoded as inert data, then validated into typed inputs; aliases, anchors and tags are rejected. Diagnostics do not echo scalar input or parser snippets.

### Consequences

- Only two external modules are needed for this slice.
- Encoded output and hash streams are deterministic and covered by golden tests.
- Schema vocabulary support must evolve with the schemas. A complete schema implementation remains a later dependency decision.

### Alternatives considered

- **Hand-written YAML/TOML parsers:** substantially more code and edge cases than maintained libraries.
- **A third JSON Schema library:** unnecessary for the vocabulary used in this slice and not approved.
- **A CLI framework:** standard `flag` handles two commands.

## 0021. Keep the first generation contract offline

- **Status:** Accepted; catalog/local development boundary amended by [0026](#0026-wire-github-catalogs-and-preserve-local-development-overrides); output preservation policy superseded by [0022](#0022-atomically-replace-owned-local-build-directories)
- **Date:** 2026-10-19
- **Amends:** [0014](#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server), for the first offline slice only

### Context

M1 steps 1–3 must run before catalog fetching, lock files, image resolution or activation exist. The user explicitly scoped this slice to local recipe input and a fictional test recipe. Runtime MCP secret delivery remains open per real tool in TRD section 16.

### Decision

Use local recipes and canonical recipe/template content identity instead of a pinned catalog revision. Emit `compose.yaml`, `manifest.json` and `tools/<tool>/files/<container path>` into `.olympus/build`, or `-o`. File binds are relative to this local directory. Service hashes include canonical service JSON and the TRD 6.3 file stream; generation hashes exclude `id`. Canonical JSON recursively sorts keys. Omit the derived hash label to avoid hashing a label containing itself.

The CLI scans `.env` names without retaining or resolving values; only names enter evaluation. For the test recipe, explicitly supplied secrets map to recipe `env` targets, and MCP config references those target names. Compose references a future per-tool runtime environment file; this slice never creates it. No general MCP secret mechanism or wrapper scripts are introduced. Real-tool delivery stays open.

Keep ports private, reject public requests and channel rendering, and never execute hooks or health checks. Allow identical output directories to be reused; preserve differing existing output and require a fresh directory. This avoids unrequested deletion and stale files. Server artifact assembly, digest enforcement and runtime secret materialization belong to later slices.

### Consequences

- Offline builds are deterministic and testable without credentials, Docker or a server.
- The artifact contains tagged images and future secret-file references. It is not a deployable locked generation.
- Changed builds need a fresh `-o` directory.
- This does not settle per-tool runtime secret syntax or delivery.

### Alternatives considered

- **Implement catalog/lock/activation now:** outside the requested scope.
- **Resolve secrets while rendering:** violates purity and risks leaking values into outputs and hashes.
- **A general wrapper mechanism:** explicitly deferred by the user.
- **Delete the existing output automatically:** risks deleting user files and requires separate confirmation under repository policy.

## 0022. Atomically replace owned local build directories

- **Status:** Accepted, requested in PR #5 review
- **Date:** 2026-10-19
- **Supersedes:** The existing-output preservation policy in [0021](#0021-keep-the-first-generation-contract-offline); all other offline boundaries remain unchanged

### Context

The original offline builder refused changed output at the default path, preventing ordinary build/edit/build use. The user authorized replacement and cleanup of tool-owned build output while requiring preservation of unrelated directories. Ordinary directory rename cannot atomically replace a non-empty destination.

### Decision

Recognize tool-owned output by a regular, non-symlink version-1 Olympus `manifest.json`, with a CLI version, input/tool maps and a generation ID matching canonical manifest contents. Refuse any existing directory without this marker. Stage all files and the manifest in a fresh sibling directory, verify that generation, then atomically exchange it with the owned build directory. Remove the exchanged old directory after publication. Failed staging leaves old output intact, and initial publication is exclusive so it cannot replace a directory created in the meantime.

Use the operating system's native atomic operations through the Go standard library's syscall interface, adding no dependency. Linux amd64/arm64 uses `renameat2` with `RENAME_EXCHANGE` or `RENAME_NOREPLACE`. macOS amd64/arm64 uses `renameatx_np` with `RENAME_SWAP` or `RENAME_EXCL`. Sources: [Linux rename manual](https://man7.org/linux/man-pages/man2/rename.2.html), [XNU syscall definitions](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/syscalls.master), [XNU rename flags](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/stdio.h). Unsupported platforms or filesystem operations fail without a non-atomic fallback. Parent directory descriptors and confined filesystem APIs bound publication and cleanup to the selected local output parent.

### Consequences

- Rebuilding after edits works at the default output path and removes stale artifacts.
- A valid manifest declares the entire build directory tool-owned, including extra files placed there. It is an ownership marker, not authentication against a malicious local user.
- Staged content becomes visible as a complete directory. This promises atomic visibility, not power-loss durability or concurrent-build ordering.
- Linux amd64 is runtime-tested in this run. Other supported targets are cross-compiled; native runtime verification remains outstanding.

### Alternatives considered

- **Require a fresh directory after each edit:** breaks the requested default rebuild workflow.
- **Rename old output away, then rename new output into place:** leaves a window with no build directory.
- **Delete prior output before writing:** risks data loss on failed builds and exposes incomplete output.
- **Add a filesystem dependency:** unnecessary for the native operations needed here; dependency choices remain unchanged.

## 0023. Hash the extracted catalog tree in a canonical form

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

`stack.lock` pins the recipe catalog by commit and by a content hash ([0008](#0008-pin-exact-versions-in-stacklock), [0016](#0016-pin-the-recipe-set-as-a-flake-style-input)). The lock hash decides whether the cached tree is the one that revision produced, and it is the cache's directory name, so it must be reproducible by any Olympus version, on any machine, from the files alone. [TRD 6.4](TRD.md#64-lock-resolution) only said "sorted paths, file modes, contents", like a Nix NAR hash, which leaves the exact byte stream open.

The alternatives each had a problem. Hashing the downloaded tarball depends on gzip headers, entry order and mtimes that GitHub does not guarantee. A git tree hash depends on git's object format and does not hash a subdirectory directly. A NAR is well specified but implements a general serialization (directories, symlinks, executable bits, device nodes) for a use that only has regular files.

### Decision

We will hash the **extracted catalog subdirectory** with SHA-256 over this byte stream:

1. the domain string `olympus-catalog-v1` followed by a NUL byte;
2. for every regular file, sorted by relative path with `/` separators:
   - the relative path, NUL,
   - the permission bits (`0`–`777`) in octal, NUL,
   - the size in decimal, NUL,
   - the file contents,
   - NUL.

The result is `sha256-<hex>`, which is both the lock's `hash` and the cache directory name under `~/.cache/olympus/inputs/`. Directories, empty directories, mtimes, ownership and setuid/setgid/sticky bits are excluded. Symlinks and other non-regular files are rejected before hashing, not hashed.

### Consequences

- The hash can be recomputed from an extracted tree with no git, network or Docker, and cache entries are verified on every use.
- Content or permission changes change the hash; moving files, rewriting them in a different order, or changing mtimes does not.
- The hash is not a commit hash and not a NAR hash. A change to the format needs a new domain string (`olympus-catalog-v2`) so old and new values can never collide.
- Empty directories are not represented; recipes must not rely on them.
- Setuid/setgid/sticky bits are not represented; extraction does not preserve them either.

### Alternatives considered

- **Tarball bytes:** GitHub can regenerate the archive with different compression and metadata, so the same revision would hash differently.
- **Git tree hash:** exact for the repository, but extracting a subdirectory needs git object parsing, and the result would still not cover extracted modes as the cache sees them.
- **Nix NAR serialization:** proven, but far more format than regular files and modes need, and we would own a NAR implementation for one hash.
- **Contents only:** misses the executable bit, which recipe scripts need.
- **A per-file manifest in the lock:** same guarantee with a larger file and more code.

## 0024. Resolve image digests with the standard library

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

The lock stores each tool image as a multi-architecture index digest ([TRD 5.3](TRD.md#53-lock-file-stacklock); security rule A7). Resolution happens by talking to the registry API, because the TRD excludes a local Docker daemon, and it must work for the official public images in v1. The TRD lists `github.com/google/go-containerregistry` as a candidate. That module is maintained (v0.22.1, 2026-09-04), but it brings a dependency tree for auth, transport, platforms and image manipulation that this narrow job does not need.

### Decision

We will resolve tags to index digests in `internal/lock` using `net/http` only:

- `GET /v2/<repository>/manifests/<tag>` with an `Accept` header for the OCI index and Docker manifest list;
- on `401`, follow an anonymous `Bearer` challenge once, then retry;
- reject anything that is not an index/list with at least one manifest;
- compute the SHA-256 of the response body and require the `Docker-Content-Digest` header, when present, to match;
- return `name@sha256:<hex>`. A reference that is already pinned is validated and returned without a network call.

v1 supports public registries with anonymous bearer auth only. Docker credential helpers, private registries, platform selection and fetching a platform manifest are out of scope; the digest is the index digest, as the lock requires.

### Consequences

- One less dependency and supply-chain surface in the binary; tests run against `httptest` with no network.
- We own the registry protocol edge cases: redirects, token challenges, content types and digest checks. They are covered by unit tests, but a real registry is only exercised end to end when `olym update` is wired and tested live.
- Private images fail with a clear registry error until a new ADR adds credentials.
- If private registries or credential helpers become required, adding `go-containerregistry` is the upgrade path, and this decision is superseded rather than patched indefinitely.

### Alternatives considered

- **`github.com/google/go-containerregistry` v0.22.1:** maintained and complete, but a large dependency for one unauthenticated request; not added pending a real need for credentials or platform handling.
- **Shell out to `crane`, `skopeo` or `docker`:** adds an external tool and a version to support, and Docker is explicitly not required.
- **Keep the tag in the lock:** moves on every pull and breaks A7 and reproducible generations.
- **Query the registry's web API or a third-party service:** undocumented, rate-limited, and trusts a third party for a security-relevant value.

## 0025. Test against a local QEMU VM from the Ubuntu cloud image

- **Status:** Accepted
- **Date:** 2026-10-19

### Context

Everything from `olym plan` onward needs a real server to test against: SSH, sudo, systemd, a Docker install and Compose. The owner wants testing to leave their own machine's configuration untouched. Options were a privileged Docker container with systemd and Docker inside, a local VM, or a paid VPS.

### Decision

We will test against one disposable local VM, started by `lab/lab.sh`. It runs the official Ubuntu 24.04 server cloud image under QEMU with KVM, configured by cloud-init with a lab-only SSH key and a passwordless-sudo user `olympus`. QEMU's user-mode network makes it reachable only at `127.0.0.1:2222`. Its disk is a copy-on-write layer over the downloaded image, so a reset discards all changes in about 15 seconds. Everything the lab creates lives in `.lab/` in the repository; the only host requirement is the QEMU package. Tests pass the lab key, the lab `known_hosts` file and `XDG_CACHE_HOME=.lab/cache` to Olympus explicitly.

### Consequences

- Tests run against the same image cloud providers boot, with a real kernel, systemd and a from-scratch Docker install.
- No host configuration changes: no bridges, firewall rules, services, group changes, or writes to `~/.ssh` or `~/.cache`.
- Olympus needs a way to use a `known_hosts` file other than `~/.ssh/known_hosts` (the remote package already accepts one; the CLI must expose it).
- The lab is amd64 and runs on a developer machine. Port exposure from outside, a change that blocks SSH, and arm64 still need one run on a real VPS before release.
- About 4 GB of RAM while the VM runs, and about 600 MB for the image.

### Alternatives considered

- **Privileged Docker container with systemd and Docker inside:** fastest, but Docker-in-Docker storage, networking and systemd differ from a real server, and it needs a privileged container on the host.
- **The owner's Ubuntu 24.04 desktop ISO:** no download, but needs an installer run and produces a desktop system unlike a VPS.
- **A paid VPS for every test:** most realistic, but costs money, is slower to reset and needs network access for every run. Kept for the pre-release check.
- **libvirt or Incus:** convenient management, but they add system services, network bridges and group membership on the host.

## 0026. Wire GitHub catalogs and preserve local development overrides

- **Status:** Accepted
- **Date:** 2026-10-20
- **Supersedes:** The official-source-only restriction in [0006](#0006-keep-official-recipes-in-one-folder-in-this-repo) and [0016](#0016-pin-the-recipe-set-as-a-flake-style-input), and the catalog/lock deferral in [0021](#0021-keep-the-first-generation-contract-offline)

### Context

The requested M1 wiring selects arbitrary GitHub catalogs through `inputs.recipes` while retaining the existing local recipe development workflow. Official-only source restrictions and the first offline slice no longer describe the implemented CLI.

### Decision

Default to `github:Haze-dev1/Olympus/recipes` and accept `github:<owner>/<repo>[/<path>]`. Resolve the GitHub default-branch HEAD on `update` or the first `check`; pin its revision, canonical tree hash and tool index digests. Require an existing lock for catalog builds. Keep the existing lock format and pure evaluator, passing resolved revisions and image references as inputs.

An explicit local `-recipes` override skips the catalog pin with a visible message. Existing image pins remain enforced. Without a lock, local check/build retains the earlier development behavior and content identity. Refuse local `update`, which cannot produce a GitHub catalog revision. These artifacts remain subject to the existing offline runtime limitations.

Read optional `GITHUB_TOKEN` only at the CLI boundary and use it only on GitHub origins for commit lookups and tarballs. Strip it on redirects elsewhere and omit remote response bodies and transport details from catalog HTTP diagnostics.

### Consequences

- Existing locks give reproducible catalog builds without moving tags or resolving registry images again.
- Other GitHub catalogs are supported, but their recipes remain executable deployment inputs that users must trust.
- Local development can still produce unpinned artifacts without a lock and must not be treated as a deployable locked generation.
- Private registries remain unsupported; GitHub credentials never become registry credentials.

### Alternatives considered

- **Keep official-only sources:** conflicts with the requested source syntax and private repository support.
- **Require a synthetic local catalog pin:** would change the lock format and break the existing development workflow.
- **Forward one authenticated client to all downloads:** risks credential exposure to registries and redirect hosts.

## 0027. Activate a generation from the store by manifest id

- **Status:** Accepted, amended by [0028](#0028-store-health-checks-in-the-generation-manifest)
- **Date:** 2026-10-20

### Context

ADR 0017 puts activation on the server: the CLI uploads the generation and starts `olym activate <n>` detached from SSH. TRD 7.1 describes the layout in shorthand (`store/gen-<generation hash>/`, `generations/1 -> ../store/gen-<hash>`, `current -> generations/2`), but it does not say what the activator receives, what it validates, how a confirmation is tied to one activation, or where a rollback gets its health checks. The manifest (TRD 5.4) carries per-tool hashes, image references, file hashes and secret names only; it has no health data, and the server has no recipes.

### Decision

We will activate by manifest id. `internal/activate` takes an id that must match `^sha256-[0-9a-f]{64}$`, reads `store/gen-<id>/manifest.json`, requires manifest version 1 and `manifest.id == id`, requires `compose.yaml` beside it, and validates every tool name with the stack name rule. Nothing in the manifest or the secrets payload may name a path outside the store, the generation or the tool's secrets directory.

The activator realises TRD 7.1 literally: `store/gen-<id>` holds the uploaded generation, `generations/<n>` is a relative symlink to `../store/gen-<id>`, and `current` is a relative symlink to `generations/<n>`. A confirmed activation numbers the generation `max+1`; rollback re-points `current` at an existing lower number instead of creating one.

One exclusive `flock` on `<root>/lock` covers activate, rollback and gc. `lock.json` records pid, user, command and start time, so a second caller fails fast (exit code 4) naming the holder; the kernel drops the lock when the process exits, so it never goes stale.

A confirmation is tied to one activation by a random 256-bit nonce in `state.json`. `Confirm` writes `<root>/confirm` only when the id and nonce match the waiting activation, and the activator deletes any confirmation file before it publishes the nonce, so a stale file can never confirm a later run.

Per-tool health checks come from a `HealthSource` interface keyed by generation id, defined in this package: an HTTP GET to `127.0.0.1:<port><path>` expecting 2xx, or a command run with `docker compose exec` in the tool's container. The manifest cannot supply them yet.

Compose is always `docker compose -p olympus -f <generation>/compose.yaml ...`, built from an argument list, so only the `olympus` project changes (A11) and no shell parses a path (A2).

### Consequences

- The generation is the only input: the activator needs no recipes, no stack lock and no network access of its own.
- A malformed or mismatched id fails before anything is created, and the store and secrets directories are the trust boundary for every uploaded value.
- Health data is a caller responsibility until the manifest carries it, so `olym rollback` on the server has nothing to pass yet. Automatic rollback re-runs the previous generation's Compose services and its health checks only when the caller supplied them. The follow-up below closes this.
- A revert restores the previous secret values: before installing new ones, the activator copies each affected tool's secrets to `secrets/<tool>.previous`, restores them if the new generation fails, and deletes the copy once the new generation is confirmed. If writing the new values fails partway (an I/O error, since the payload is validated first), the copies are not yet restored automatically; that edge is a follow-up.
- Relative symlinks keep the root movable and make `generations/<n>` readable on its own.
- The nonce costs one file and one field, and it is what makes a late `olym confirm` harmless instead of dangerous.

### Alternatives considered

- **Pass the whole manifest on the command line:** puts a generation's content, including secret names, into process arguments and lets a second caller's view diverge from the store.
- **A long-running activation daemon:** rejected in ADR 0017.
- **Health checks from the Compose file's `healthcheck:`:** Docker runs those inside the container and needs curl or a shell in the image; TRD 8.5 checks from the host for that reason.
- **Trust `lock.json` instead of `flock`:** a stale file would block every later run until someone deleted it.
- **A database or one state directory per activation:** more moving parts than one `state.json` and one confirmation file.

Follow-up: add an optional `health` field to `generation.Tool`, filled from `recipe.Health` in `internal/eval`, so `manifest.json` becomes the `HealthSource` and server-side rollback is self-contained. That change amends this decision.

## 0028. Store health checks in the generation manifest

- **Status:** Accepted
- **Date:** 2026-10-20
- **Amends:** [0027](#0027-activate-a-generation-from-the-store-by-manifest-id)

### Context

The activator has no recipes. Rollback needs the previous generation's readiness checks, so a caller-supplied map cannot be the durable source of health data.

### Decision

Each manifest tool may carry `health`, copied from its validated recipe: `http: {port, path}` or an argument-list `command`, and optional `timeout` as a Go duration string. Missing health remains valid for older version-1 manifests. Health changes affect the generation identity. Tool hashes retain their existing file-and-Compose definition.

`activate.ManifestHealthSource` adapts a generation-id manifest loader to `HealthSource`, validates identity/version and readiness checks, and converts timeout strings to activator durations. Loading both the requested and previous manifest makes rollback health self-contained without adding a generation-to-activator import cycle. Wiring that loader into server commands belongs to their implementation.

### Consequences

Generation builds remain pure and secret-free. Version 1 is additive, so old generations have no declared checks. The new field changes golden generation identities. The adapter rejects invalid durations/checks rather than silently ignoring them.

### Alternatives considered

- Fetch recipes on the server: adds network and catalog state to rollback.
- Put health only in Compose: requires container-local HTTP tooling.
- Import activator types into generation: creates an import cycle.
- Persist numeric nanoseconds: harder to inspect than recipe duration strings.

## 0029. Verify deployment uploads and retain the activation result

- **Status:** Accepted
- **Date:** 2026-10-20
- **Amends:** [0017](#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms)

### Context

The client must distinguish a detached activation's result even when it fails before writing state, or systemd unloads a successfully completed transient service. Release builds do not yet embed Linux binary checksums. The current manifest has no health data.

### Decision

Upload regular generation files through a tar stream into a random directory under `store/.incoming`, validate manifest identity and declared file hashes locally, verify every uploaded file against the local bytes using server-side SHA256, then rename the directory to `store/gen-<id>`. Reuse an existing destination only after verifying its files. Clean failed staging directories when the SSH connection permits; a broken connection can leave only an unpublished `.incoming` directory.

Upload a local Linux ELF binary matching the server architecture into a temporary file, verify its SHA256 before executing its `version` command, require the CLI version, then publish `bin/olym-<version>` and its stable symlink. Recheck checksum and version before activation. Explicit development paths are supported. Embedded release checksums and release downloads remain deferred, so this trusts the selected local file rather than a release signature.

Start a uniquely named systemd transient service with `Type=exec` and `RemainAfterExit=yes`. Read its exit result before stopping and resetting the unit. This retains successful results that systemd would otherwise unload. Poll state only to obtain a new confirmation nonce; never reuse a nonce present before launch. Confirm through a new SSH connection with the same host-key policy. If confirmation cannot be sent, continue observing the server's automatic rollback when the original connection remains available.

Server commands use the real Compose runner and health checker. Until the separately developed manifest health field lands, an empty health source performs no readiness checks. Suppress container command output in these commands to prevent it entering activation logs. The SSH configuration may explicitly restrict host-key algorithms while still requiring the normal known-host verification; the lab selects its pinned Ed25519 key type.

### Consequences

- Failed uploads do not publish partial store entries. Interrupted connections may leave unpublished staging directories for later cleanup.
- Pre-activation errors return 1, a completed rollback returns 3 and lock contention returns 4. An unavailable monitoring connection reports an error without claiming a verified rollback.
- Successfully observed transient services are cleaned up. A disconnected client can leave a completed unit requiring later cleanup.
- Manifest health wiring, release checksum embedding, secret fingerprints, container-user ownership and secret-change recreation remain separate work. The existing fixed secrets payload path also requires callers to serialize payload submission with activation; the activator lock alone does not protect competing uploads of secrets.

### Alternatives considered

- Read only `state.json`: misses failures before state publication and can read a previous run's state.
- Default transient service lifetime: loses the success result when systemd unloads the unit.
- Execute an uploaded binary to check its version before hashing it: violates the checksum trust boundary.
- New remote shell scripts or dependencies: existing argument-list commands and standard Go tar support suffice.

## Template

```markdown
## NNNN. <The decision, phrased as a decision>

- **Status:** Proposed | Accepted | Superseded by NNNN
- **Date:** YYYY-MM-DD

### Context

What was true that made this a decision rather than an obvious choice: constraints, goals, existing commitments.

### Decision

We will ...

### Consequences

What becomes easier, and what becomes harder. Include the bad ones.

### Alternatives considered

- **<Alternative>:** the specific reason it was rejected.
```
