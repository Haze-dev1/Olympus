# Security data-flow note

_Last reviewed: 2026-11-08_ (Baseline commit: `61a1e57`)

This note documents security data flow across Olympus, structured from [.agents/skills/review-app-security/reference/data-flow-note.md](../../.agents/skills/review-app-security/reference/data-flow-note.md). It traces sensitive model keys, bot/GitHub tokens, SSH credentials, chat content, trust boundaries, and rules A1–A12 against the implemented codebase.

---

## 1. What this app is

Olympus is a local Go CLI (`olym`, [cmd/olym/main.go](../../cmd/olym/main.go), line 8) that sets up containerized AI tools and agent harnesses on an existing Linux server over SSH from `stack.yaml` and a pinned recipe catalog ([docs/PRD.md](../PRD.md); [docs/TRD.md](../TRD.md); [ADR 0003](../ADR.md#0003-target-an-existing-server-over-ssh-in-v1); [ADR 0013](../ADR.md#0013-write-olympus-in-go)).

The CLI evaluates the stack locally into an immutable generation without connecting to the server or resolving secret values ([internal/eval/eval.go](../../internal/eval/eval.go), lines 63–65; [ADR 0014](../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)). It connects over SSH as root or a passwordless sudo user ([internal/remote/remote.go](../../internal/remote/remote.go), line 59), uploads generation artifacts (`compose.yaml`, configs, seccomp profiles, manifest) to `/var/lib/olympus/store/` ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 176, line 198; [ADR 0030](../ADR.md#0030-address-rendered-files-by-tool-hash-across-generations)), transmits declared secrets strictly over SSH `stdin` ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 420–421), and executes a detached server-side activator binary (`/var/lib/olympus/bin/olym-<version>`, [internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 353) to write `0600` secret files, start Docker Compose services under project `olympus`, and roll back automatically on health check failure or unconfirmed deadline ([internal/activate/activate.go](../../internal/activate/activate.go), line 428; [ADR 0015](../ADR.md#0015-use-one-docker-compose-project-as-the-runtime); [ADR 0017](../ADR.md#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms); [ADR 0027](../ADR.md#0027-activate-a-generation-from-the-store-by-manifest-id)).

There is no web application, database, or multi-user login in Olympus v1 ([ADR 0003](../ADR.md#0003-target-an-existing-server-over-ssh-in-v1); [ADR 0007](../ADR.md#0007-run-each-tool-in-its-own-docker-container); [ADR 0010](../ADR.md#0010-expose-nothing-publicly-by-default)).

---

## 2. Users and roles

- **Operator (Local Developer):** Runs the CLI on a workstation. Controls `stack.yaml`, `<stack-dir>/.env`, local SSH keys/agent, and optional workstation environment (`GITHUB_TOKEN`).
- **Server Root / Docker Administrator:** Privileged execution boundary on the target host ([internal/remote/remote.go](../../internal/remote/remote.go), line 151). Owns `/var/lib/olympus/` and Docker daemon. Under this threat model, anyone with root or Docker socket access on the server can inspect all container environments (`docker inspect`) and volume mounts.
- **Containerized AI Tools / Agent Processes:** Container workloads running in Docker (e.g. Hermes, OpenCode, Codex, OpenClaw, Goose, OpenBot, Claude Code, Gemini CLI, Aider). Each container executes as its declared image user (e.g. `root`, `node`, `appuser`) and reads the secrets passed to it by Docker. Olympus does not configure user namespace remapping or guarantee isolation against malicious container code that escapes.
- **Chat Senders (External Users):** Third parties interacting with deployed chat bots via external messaging platforms (Telegram, Discord). Configured to receive allow-lists in the container environment; actual sender filtering is UNVERIFIED in this Olympus-only review ([internal/eval/eval.go](../../internal/eval/eval.go), line 356, line 384).
- **Multi-tenant?** No. Olympus v1 is single-operator, single-server.

---

## 3. Data we store and process

| Data Element | Sensitivity | Storage Location (Operator / Server) | Who reads it |
| --- | --- | --- | --- |
| **Model API keys & tool secrets** (e.g. `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) | Secret / High | Operator: `<stack-dir>/.env`<br>In-flight: SSH `stdin`<br>Server: `/var/lib/olympus/secrets/<tool>/<name>` and `environment` (mode `0600`) | CLI memory at command execution ([internal/cli/secrets.go](../../internal/cli/secrets.go), line 19); server activator ([internal/activate/activate.go](../../internal/activate/activate.go), line 739); Docker daemon; tool container process via individual files or `env_file`. Use of keys by upstream binaries is UNVERIFIED in this source review. |
| **Bot tokens** (`TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`) | Secret / High | Operator: `<stack-dir>/.env`<br>In-flight: SSH `stdin`<br>Server: `/var/lib/olympus/secrets/<tool>/environment` (mode `0600`) | CLI memory; server activator; Docker daemon; tool container process. Upstream use and transport of bot tokens are UNVERIFIED here. |
| **Channel allow-lists & roles** (`TELEGRAM_ALLOWED_USERS`, `DISCORD_ALLOWED_USERS`, `DISCORD_ALLOWED_ROLES`) | Non-secret / Policy | Operator: `stack.yaml`<br>Server: `/var/lib/olympus/store/gen-<id>/compose.yaml` | CLI evaluator ([internal/eval/eval.go](../../internal/eval/eval.go), line 384); Docker Compose; tool container environment. |
| **Chat message content & prompts** | User Content | Expected flow (UNVERIFIED): chat platform ↔ tool process ↔ model API. Tool memory, volumes and runtime logging depend on the upstream tool. | Expected readers (UNVERIFIED): tool code, chat/model services, and privileged server operators. Olympus configures tools rather than implementing their chat storage; this source review does not establish upstream retention or logging. |
| **SSH private key / agent credentials** | Secret / Critical | Operator: Local key file (`server.ssh_key`) or `SSH_AUTH_SOCK` | Explicit key files are parsed locally; SSH-agent authentication obtains signers from the local agent ([internal/remote/remote.go](../../internal/remote/remote.go), lines 177–197). This flow does not upload private key bytes. |
| **SSH host key verification data** | Trust Anchor | Operator: `~/.ssh/known_hosts` or explicit file | Local CLI process ([internal/remote/remote.go](../../internal/remote/remote.go), line 203). Refuses changed keys; prompts interactively on unknown keys. |
| **Catalog `GITHUB_TOKEN`** | Secret / Medium | Operator: Workstation environment variable `GITHUB_TOKEN` | Local CLI process memory only ([internal/cli/cli.go](../../internal/cli/cli.go), line 22; [internal/catalog/catalog.go](../../internal/catalog/catalog.go), line 190). Not transmitted to the server by the catalog flow. |
| **Container GitHub/MCP tokens** | Secret / High | Operator: `<stack-dir>/.env`<br>Server: `/var/lib/olympus/secrets/<tool>/environment` (mode `0600`) | Tool container process and containerized MCP server process via environment references ([internal/eval/eval.go](../../internal/eval/eval.go), line 247, line 259). |
| **Server HMAC key** (`key`) | Secret / Critical | Server: `/var/lib/olympus/key` (mode `0600`) | Server activator ([internal/activate/fingerprints.go](../../internal/activate/fingerprints.go), line 28); CLI `plan` over verified SSH via `SecretState` ([internal/deploy/commands.go](../../internal/deploy/commands.go), line 45). Used to compute HMAC fingerprints without revealing secrets. |
| **Secret fingerprints** (`fingerprints.json`) | Keyed Hash / Non-secret | Server: `/var/lib/olympus/secrets/<tool>/fingerprints.json` (mode `0600`) | Server activator; CLI `plan` over SSH (`SecretState`). Used to detect secret changes across generations. |
| **Generation artifacts** (`compose.yaml`, configs, seccomp, manifest) | Configuration | Server: generation files under `/var/lib/olympus/store/gen-<manifest-id>/`; per-tool files under `/var/lib/olympus/store/<tool-hash>/files/`; `/var/lib/olympus/generations/<n>` links to the generation ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 154, 176, 198; [internal/activate/activate.go](../../internal/activate/activate.go), line 705) | Server activator; Docker daemon; tool containers where mounted. Managed secret values are transferred separately (A1). |
| **Activator binary** | Executable / Privileged | Operator: Embedded/bundled binary.<br>Server: `/var/lib/olympus/bin/olym-<version>` (mode `0755`) | Server `systemd-run` executing as root. Release builds verified by embedded SHA-256 (Rule A12); dev builds without checksums gated by `-unsafe-dev-activator` (ADR 0049). |

---

## 4. Trust boundaries

### Boundary 1: Stack & Recipe Files → CLI Evaluation
- Input YAML is parsed by a restricted AST parser that enforces a maximum depth of 64 ([internal/schema/schema.go](../../internal/schema/schema.go), line 114), forbids variables in keys (line 128), forbids duplicate keys (line 132), and rejects YAML aliases, anchors, and custom tags (line 162; Rule A9).
- `stack.yaml` secrets must be single `${NAME}` references; inline secret values are rejected ([internal/stack/stack.go](../../internal/stack/stack.go), line 112).
- `CheckEnv` physically scans `<stack-dir>/.env` off disk using `bufio.Scanner`, parses `NAME=value` lines to verify referenced variable names exist, and discards parsed values without storing or returning them to evaluation ([internal/stack/stack.go](../../internal/stack/stack.go), line 172). `CheckEnv` runs for `check`, `build`, `apply`, and `plan` ([internal/cli/cli.go](../../internal/cli/cli.go), line 56).
- Volume paths, template destinations, and managed file destinations must resolve strictly inside declared recipe volume roots ([internal/recipe/recipe.go](../../internal/recipe/recipe.go), line 262; [internal/eval/eval.go](../../internal/eval/eval.go), line 290, line 323; Rule A3).

### Boundary 2: Operator Workstation `.env` → CLI Memory
- Secret values are resolved only when executing deployment commands (`plan`, `apply`) in `toolSecrets` ([internal/cli/secrets.go](../../internal/cli/secrets.go), line 19).
- Values are read directly from `<stack-dir>/.env`; there is **no process-environment fallback** for stack tool secrets ([internal/cli/secrets.go](../../internal/cli/secrets.go), line 18).
- Multi-line values and null bytes are rejected ([internal/cli/secrets.go](../../internal/cli/secrets.go), line 67).
- Reserved secret names (`environment`, `fingerprints.json`) and undeclared recipe secrets are rejected ([internal/cli/secrets.go](../../internal/cli/secrets.go), line 79, line 84).

### Boundary 3: CLI → Remote Host over SSH
- Authenticates using `server.ssh_key` or local SSH agent (`SSH_AUTH_SOCK`) ([internal/remote/remote.go](../../internal/remote/remote.go), line 174). Private keys never leave the workstation.
- Server host keys are verified against `known_hosts` ([internal/remote/remote.go](../../internal/remote/remote.go), line 203). Changed host keys are unconditionally refused ([internal/remote/remote.go](../../internal/remote/remote.go), line 222; Rule A6). Unknown keys require interactive confirmation before being saved with mode `0600` (lines 225–226, 263–285).
- Remote commands are argument lists (`[]string`), shell-quoted with POSIX single quotes ([internal/remote/quoting.go](../../internal/remote/quoting.go), line 8; [internal/remote/remote.go](../../internal/remote/remote.go), line 109; Rule A2).
- In the CLI → server transfer, managed secret values are not placed in command arguments or SSH command environment; they are piped over SSH `stdin` via `install -m 0600 /dev/stdin /var/lib/olympus/secrets-<suffix>.incoming` ([internal/remote/remote.go](../../internal/remote/remote.go), line 106; [internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 420–421; Rule A1).
- Remote errors discard remote stderr to prevent secret leakage in error output ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 72–84, 549–555).

### Boundary 4: Server Filesystem (`/var/lib/olympus/`) → Server Activator
- Secrets arrive in `/var/lib/olympus/secrets-<suffix>.incoming`. The activator reads the payload and **deletes the incoming file immediately** before replacing secret files ([internal/activate/activate.go](../../internal/activate/activate.go), line 757; Rule A1).
- Activator verifies that every secret tool and secret name is declared in the generation manifest ([internal/activate/activate.go](../../internal/activate/activate.go), line 777; Rule A5).
- Secret delivery writes both file-based and environment-mapped formats:
  - Every declared secret is written as an individual file `/var/lib/olympus/secrets/<tool>/<secret-name>` with mode `0600` ([internal/activate/activate.go](../../internal/activate/activate.go), line 786, line 830; Rule A5).
  - If secrets declare an `env` mapping in the recipe, an `environment` file is compiled and written under `/var/lib/olympus/secrets/<tool>/environment` with mode `0600` (line 793, line 830).
- Secret values are hashed with `/var/lib/olympus/key` into `fingerprints.json` ([internal/activate/activate.go](../../internal/activate/activate.go), line 801; [internal/activate/fingerprints.go](../../internal/activate/fingerprints.go), line 16; [ADR 0032](../ADR.md#0032-fingerprint-secrets-with-a-server-key-and-restore-partial-installs)).

### Boundary 5: Activator → Docker Daemon & Containers
- Activator executes Docker Compose strictly scoped to project `olympus` ([internal/activate/compose.go](../../internal/activate/compose.go), line 20; Rule A11).
- Secrets are passed into containers by Docker Compose via `env_file: [/var/lib/olympus/secrets/<tool>/environment]` ([internal/eval/eval.go](../../internal/eval/eval.go), line 247). Only tools with declared secrets receive an `env_file` (Rule A5).
- Published ports bind to `127.0.0.1` unless declared in `ports.public` ([internal/eval/eval.go](../../internal/eval/eval.go), line 207; Rule A4).
- Container health checks execute inside the container via `docker compose exec -T ...` ([internal/activate/compose.go](../../internal/activate/compose.go), line 83; Rule A8).
- **Threat-model limits:** Server root and Docker daemon administrators can inspect all running container environments (`docker inspect`) and volume files. Recipe and container image code read provided secrets directly. Olympus does not isolate secrets from privileged host accounts.

### Boundary 6: Tool Containers → External Network & APIs
- Expected flow (UNVERIFIED): tool processes use supplied keys/tokens to contact model and chat services. Actual protocols, recipients and runtime sender authorization require pinned-upstream or live evidence.
- Olympus injects the configured allow-list into the tool environment ([internal/eval/eval.go](../../internal/eval/eval.go), line 384; A10). This proves delivery, not upstream enforcement.
- Olympus publishes only recipe-declared ports and defaults those bindings to loopback ([internal/eval/eval.go](../../internal/eval/eval.go), lines 202–211). This does not establish whether an upstream binary opens an internal webhook listener.
- **Runtime limit (UNVERIFIED):** Whether upstream tool processes connect via outbound long-polling or WebSockets, whether they open listening ports internally, and how their internal logic enforces sender filtering at runtime are `UNVERIFIED` from Olympus repository source; `eval.go` proves configuration injection only.

---

## 5. Detailed data tracing

### 5.1 Sensitive Model Keys and Tool Secrets

Bundled recipes declare explicit secrets mapped to container environment variables and files. For example, Hermes Agent ([recipes/hermes/recipe.yaml](../../recipes/hermes/recipe.yaml), lines 18–26) declares:

```yaml
secrets:
  model-key:
    env: ANTHROPIC_API_KEY
  telegram-token:
    env: TELEGRAM_BOT_TOKEN
  discord-token:
    env: DISCORD_BOT_TOKEN
  github-token:
    env: GITHUB_TOKEN
```

- **Flow:**
  1. Operator defines variable in `<stack-dir>/.env` and references it in `stack.yaml` (`secrets: { model-key: '${ANTHROPIC_API_KEY}' }`).
  2. `internal/eval/eval.go`, line 247 assigns `service.EnvFile = []string{"/var/lib/olympus/secrets/hermes/environment"}`.
  3. `internal/cli/secrets.go`, line 19 reads the value from `.env` at command execution.
  4. `internal/deploy/deploy.go`, lines 420–421 pipe the payload over SSH `stdin` to `/var/lib/olympus/secrets-<suffix>.incoming`.
  5. `internal/activate/activate.go`, line 757 deletes the incoming file, writes `/var/lib/olympus/secrets/hermes/model-key` with mode `0600` (line 786, line 830), and compiles `/var/lib/olympus/secrets/hermes/environment` with mode `0600` (line 793, line 830).
  6. Docker daemon injects the environment file into the tool container.
  7. Container process reads the environment variable or mounted file.

### 5.2 Bot Tokens, Allow-lists, and Chat Content

- **Bot Tokens:** Referenced in `stack.yaml` under `channels.<name>.token` and matched to a declared recipe secret ([internal/eval/eval.go](../../internal/eval/eval.go), line 363). Transmitted via the secrets pipeline into `/var/lib/olympus/secrets/<tool>/environment`.
- **Allow-lists:** Configured in `stack.yaml` under `channels.<name>.allow`. Rule A10 enforces `len(c.Allow) > 0` ([internal/eval/eval.go](../../internal/eval/eval.go), line 356). Finding F1 enforces that the recipe maps the allow-list to an environment variable (`allowEnv := support.Env["allow"]`, line 380). The list is joined by commas and placed in the container's Compose environment (line 384). For Hermes Telegram channels, numeric user IDs are required ([recipes/hermes/README.md](../../recipes/hermes/README.md), lines 27–38; [docs/specs/stack-file.md](../specs/stack-file.md)).
- **Runtime Transport and Headers (UNVERIFIED):** Upstream network transport (long-polling vs WebSockets), auth headers, and whether agent binaries open internal ports are `UNVERIFIED` from Olympus repository source.

### 5.3 GitHub Tokens

- **Catalog Fetcher Token:** Reads workstation `os.Getenv("GITHUB_TOKEN")` ([internal/cli/cli.go](../../internal/cli/cli.go), line 22). Used exclusively in `internal/catalog/catalog.go`, line 190 to authenticate requests to `api.github.com` and `github.com`. Tokens are stripped on redirects (`Header.Del("Authorization")`, line 199, line 208) unless redirected to allowed GitHub origins. This token is **not sent to the server by the catalog flow**.
- **Tool / MCP GitHub Token:** If a containerized tool requires GitHub access, the operator defines a secret reference in `stack.yaml`. This travels through the secrets pipeline into `/var/lib/olympus/secrets/<tool>/environment`. MCP servers reference it as a tool-native environment variable (e.g. `${GITHUB_TOKEN}` in Hermes/Claude Code, `{env:GITHUB_TOKEN}` in OpenCode, `env_vars` in Codex; [internal/eval/eval.go](../../internal/eval/eval.go), line 259; [ADR 0037](../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references)).

### 5.4 SSH Access Credentials and Host Verification

- `stack.yaml` defines `server.host`, `server.user`, `server.port`, and optional `server.ssh_key` ([internal/stack/stack.go](../../internal/stack/stack.go), line 17).
- SSH authentication uses the local private key file or SSH agent ([internal/remote/remote.go](../../internal/remote/remote.go), line 174). Keys are held in local memory and never transmitted.
- Host key verification against `known_hosts` is mandatory ([internal/remote/remote.go](../../internal/remote/remote.go), line 203). Changed host keys fail unconditionally ([internal/remote/remote.go](../../internal/remote/remote.go), line 222; Rule A6). Unknown keys require explicit operator approval (line 225).
- Remote commands are executed as quoted argument lists ([internal/remote/quoting.go](../../internal/remote/quoting.go), line 8; Rule A2).

### 5.5 Activator Binary Verification (Release vs Development)

Rule A12 requires verifying the server activator binary before execution ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 282; [internal/deploy/commands.go](../../internal/deploy/commands.go), line 74; [ADR 0048](../ADR.md#0048-cross-compile-release-binaries-and-verify-activator-checksum-against-release-build); [ADR 0049](../ADR.md#0049-require-an-explicit-opt-in-for-unsigned-development-activators)):

1. **Release Builds (`eval.Version != "0.1.0-dev"`):**
   - Embed release SHA-256 checksums compiled via `-ldflags` ([scripts/build-release.sh](../../scripts/build-release.sh), lines 40–47).
   - `UploadActivator` and `VerifyActivator` check the binary's SHA-256 against the embedded checksum ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 344; [internal/deploy/commands.go](../../internal/deploy/commands.go), line 100).
   - Missing or mismatched checksums **fail closed** ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 272, line 286, line 346).

2. **Development Builds (`eval.Version == "0.1.0-dev"`):**
   - A development build that embeds release checksums uses normal verification without any opt-in flag ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 283).
   - A development build with no release checksum for the server architecture **refuses by default** ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 289; OLY-61 / F3).
   - An operator must explicitly pass `-unsafe-dev-activator` to allow an unsigned development activator ([internal/cli/cli.go](../../internal/cli/cli.go), line 49; [internal/cli/commands.go](../../internal/cli/commands.go), line 62, line 189; [internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 289).
   - Release builds ignore `-unsafe-dev-activator` and always fail closed ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 286; [internal/deploy/release_test.go](../../internal/deploy/release_test.go), lines 256–263).

---

## 6. Security rules crosswalk (A1–A12)

| Rule | Requirement | Source Citation (`file:line`) |
| --- | --- | --- |
| **A1** | Secret values never in stack file, lock file, plan output, logs, errors, or process arguments; passed via files/stdin | [internal/stack/stack.go](../../internal/stack/stack.go), line 112 (stack requires `${NAME}`);<br>[internal/cli/secrets.go](../../internal/cli/secrets.go), line 17 (errors omit values);<br>[internal/generation/generation.go](../../internal/generation/generation.go), line 32 (manifest stores only secret names);<br>[internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 72–84, 549–555 (remote errors sanitized);<br>[internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 420–421 (stdin transport);<br>[internal/activate/activate.go](../../internal/activate/activate.go), line 757 (payload deleted immediately). |
| **A2** | Remote commands built from argument lists with every argument quoted | [internal/remote/quoting.go](../../internal/remote/quoting.go), lines 8–20 (`quote`, `joinArgs`);<br>[internal/remote/remote.go](../../internal/remote/remote.go), line 109 (`Run` takes `[]string`);<br>[internal/activate/compose.go](../../internal/activate/compose.go), line 20, line 88 (Compose argument lists). |
| **A3** | Template destinations and volume paths resolve inside tool volume roots; reject `..` and outside paths | [internal/recipe/recipe.go](../../internal/recipe/recipe.go), lines 253–271 (`Confined`, `hasDotDot`);<br>[internal/eval/eval.go](../../internal/eval/eval.go), line 290 (template destination confinement);<br>[internal/eval/eval.go](../../internal/eval/eval.go), line 323 (managed file confinement). |
| **A4** | Tool ports bound to `127.0.0.1` unless `ports.public` declared | [internal/eval/eval.go](../../internal/eval/eval.go), line 207 (loopback port binding). |
| **A5** | Each container gets only declared secrets; secret files mode `0600` | [internal/recipe/recipe.go](../../internal/recipe/recipe.go), line 215 (recipe declarations);<br>[internal/cli/secrets.go](../../internal/cli/secrets.go), line 82 (reject undeclared);<br>[internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 391–405 (deploy manifest validation);<br>[internal/activate/activate.go](../../internal/activate/activate.go), line 777 (activator declaration check);<br>[internal/activate/activate.go](../../internal/activate/activate.go), line 830 (mode `0600` files);<br>[internal/eval/eval.go](../../internal/eval/eval.go), line 247 (per-tool `env_file`). |
| **A6** | SSH host keys verified; host key checking never disabled silently | [internal/remote/remote.go](../../internal/remote/remote.go), line 55, lines 203–233 (known_hosts verification; changed key always refused; unknown key requires confirmation). |
| **A7** | Once lock file exists, pull images by digest, never by moving tag | [internal/lock/lock.go](../../internal/lock/lock.go), line 72 (digest required in lock);<br>[internal/cli/commands.go](../../internal/cli/commands.go), line 138 (apply requires digest-pinned images);<br>[internal/activate/compose.go](../../internal/activate/compose.go), line 67, line 74 (`--pull never`).<br>_Supply-chain gap (F2):_ Built-image Dockerfile inputs (npm/apt packages) remain mutable at build time. |
| **A8** | Recipe hooks and health checks run inside container, never on host | [internal/activate/compose.go](../../internal/activate/compose.go), line 83 (`docker compose exec -T ...` for container health checks).<br>_Note:_ Recipe lifecycle hooks (`hooks.post_install`) are declared in recipes but are **currently not executed** by Olympus core. |
| **A9** | Safe YAML loader (no arbitrary types, anchors, or aliases) | [internal/schema/schema.go](../../internal/schema/schema.go), lines 112–164 (AST parsing, depth <= 64, forbidden aliases, anchors, and custom tags). |
| **A10** | Chat channels require non-empty allow-list | [internal/eval/eval.go](../../internal/eval/eval.go), line 356 (rejects empty allow-lists);<br>[internal/eval/eval.go](../../internal/eval/eval.go), line 380 (requires allow environment mapping; F1);<br>[internal/recipe/recipe.go](../../internal/recipe/recipe.go), lines 163–164 (recipe allow mapping check). |
| **A11** | Touch only `olympus` Compose project and `/var/lib/olympus/` | [internal/activate/activate.go](../../internal/activate/activate.go), line 34 (`Project = "olympus"`);<br>[internal/activate/activate.go](../../internal/activate/activate.go), line 1040 (`/var/lib/olympus/` root directories);<br>[internal/activate/compose.go](../../internal/activate/compose.go), line 20, line 88 (`-p olympus`).<br>_Bootstrap exception:_ Host provisioning intentionally modifies system package manager configuration (`/etc/apt/sources.list.d/docker.list`, `/etc/apt/keyrings/docker.asc`) when installing Docker ([internal/remote/osrelease.go](../../internal/remote/osrelease.go), line 100). |
| **A12** | Verify activator checksum and version against release before running | [internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 270–293 (`checkReleaseChecksum`, release fail-closed, dev opt-in);<br>[internal/deploy/deploy.go](../../internal/deploy/deploy.go), line 344 (`UploadActivator` verification);<br>[internal/deploy/commands.go](../../internal/deploy/commands.go), lines 80–103 (`VerifyActivator` verification);<br>[internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 537–544 (`verifyActivator` remote checksum/version verification). |

---

## 7. Known public surface and network exposure

1. **Loopback Default (Rule A4):**
   - Every published container port binds to `127.0.0.1` by default ([internal/eval/eval.go](../../internal/eval/eval.go), line 207). Public exposure requires explicit configuration in `stack.yaml` under `ports.public`.
2. **Outbound Network Traffic:**
   - CLI connects outbound to the server over SSH (default port 22).
   - CLI connects outbound to GitHub (`api.github.com`, `github.com`) for catalog downloads ([internal/catalog/catalog.go](../../internal/catalog/catalog.go), line 176) and to registries for image digests ([internal/lock/registry.go](../../internal/lock/registry.go), line 38).
   - Expected upstream tool egress to model/chat platforms is UNVERIFIED in this Olympus-only source review.
3. **Inbound Chat Surface:**
   - Olympus injects allow-list environment variables ([internal/eval/eval.go](../../internal/eval/eval.go), line 384). Actual listening ports and sender enforcement inside the tool are UNVERIFIED here.

---

## 8. Verification status (Verified vs UNVERIFIED)

### Verified in Olympus Source Code
- Managed secret values are resolved at the command boundary and sent separately from the rendered generation; the manifest records secret names and Compose records an env-file path ([internal/cli/secrets.go](../../internal/cli/secrets.go), lines 17–19; [internal/generation/generation.go](../../internal/generation/generation.go), line 32; [internal/eval/eval.go](../../internal/eval/eval.go), lines 244–247). This is not a scan for operator-written secret literals elsewhere in config or upstream logs.
- Stack `.env` values are read strictly at the CLI command boundary; there is no process-env fallback for tool secrets ([internal/cli/secrets.go](../../internal/cli/secrets.go), lines 17–19).
- Secrets travel over SSH `stdin` to `/var/lib/olympus/secrets-<suffix>.incoming` and are deleted immediately upon read ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 420–421; [internal/activate/activate.go](../../internal/activate/activate.go), line 757).
- Secret files and `environment` files are written with mode `0600` under `/var/lib/olympus/secrets/<tool>/` ([internal/activate/activate.go](../../internal/activate/activate.go), line 830).
- Remote commands are strictly quoted argument lists ([internal/remote/quoting.go](../../internal/remote/quoting.go), lines 8–20).
- Changed SSH host keys always abort the connection ([internal/remote/remote.go](../../internal/remote/remote.go), line 222).
- Channel allow-lists must be non-empty and mapped to an environment variable ([internal/eval/eval.go](../../internal/eval/eval.go), line 356, line 380).
- Release builds fail closed if release activator checksums are missing or mismatched; development builds without checksums require `-unsafe-dev-activator` ([internal/deploy/deploy.go](../../internal/deploy/deploy.go), lines 270–293).

### UNVERIFIED (Outside Olympus Source Scope)
- **Container Internal Memory & Logging:** Whether upstream agent harnesses (Hermes, Codex, OpenCode, OpenClaw, Goose, OpenBot, Claude Code, Gemini CLI, Aider) log plain-text prompts, model keys, or chat messages to standard output or internal files is `UNVERIFIED` from Olympus repository source.
- **Upstream Network Protocols & Headers:** The exact HTTP headers, auth schemes (e.g. Bearer vs custom headers), and transport protocols used by containerized binaries to contact model providers or chat servers are `UNVERIFIED` from Olympus repository source.
- **Upstream Platform Retention:** Message retention, privacy, and logging policies of Telegram, Discord, Anthropic, OpenAI, Google, and other external APIs are `UNVERIFIED`.
- **Privileged Host Access:** Server root and Docker daemon administrators can inspect container environments and volume files. Olympus does not prevent access by a compromised root account.
