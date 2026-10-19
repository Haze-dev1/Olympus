# Olympus — Technical Requirements Document (TRD)

| | |
| --- | --- |
| **Status** | Draft for v1; M1 offline check/build implemented |
| **Date** | 2026-10-19 |
| **Owner** | Devansh (Haze) |
| **Companion** | [PRD.md](PRD.md) covers what and why. This document is self-contained: each decision is stated with its reason here. [ADR.md](ADR.md) holds the full decision records. |

Formats, paths and command names in this document are the v1 design. Anything marked **proposed** is a working choice to confirm during milestone M1; anything marked **open** is undecided.

## 1. Scope

This document specifies how Olympus v1 works: its architecture, file formats, the build and activation pipeline, the server layout, the security design, testing and release. It covers one server per stack, Ubuntu and Debian servers, and the commands in PRD section 7.4.

## 2. Design principles

| Principle | Consequence |
| --- | --- |
| **Build first, then touch the server** (Nix) | Everything that can be computed is computed on the user's machine, offline, before any SSH connection. |
| **Pure, content-addressed builds** (Nix) | The same inputs always give byte-identical output; output is stored by its hash; unchanged hashes mean nothing to do. |
| **Plan before apply** (Terraform) | Every change is shown as a diff before it happens. |
| **The server keeps its own history** (Helm, Nix) | Generations live on the server; there is no local state file to lose. |
| **The server protects itself** (deploy-rs) | Activation runs on the server and rolls back unless the CLI confirms it can still connect. |
| **Data, not text templates** (lesson from Helm) | Structured config is built as data and encoded, never produced by templating YAML text. |
| **Reuse, don't rebuild** | Docker Compose reconciles containers; SSH and systemd are used as they are. |
| **Private by default** | No public ports, no secrets in files under version control, no chat bot without an allow-list. |

## 3. Architecture

### 3.1 Components

```text
 USER'S MACHINE                                         SERVER (Ubuntu/Debian)
 ─────────────────────────────────────────────          ────────────────────────────────────────
 stack.yaml ─┐
 stack.lock ─┼─► [stack] parse + validate
 .env ───────┘        │
                      ▼
 catalog@rev ──► [catalog] fetch + verify (cached)
                      │
                      ▼
               [eval] pure build ──► generation
                      │              (compose.yaml, tool files,
                      │               manifest.json, hashes)
                      ▼
               [plan] diff ◄──────── read current manifest ──── /var/lib/olympus/
                      │                                          current → generations/<n>
                      ▼
               [remote] upload ────► store/<hash>/  (content-addressed)
                      │              bin/olym (activator)
                      │              incoming/secrets
                      ▼
               start activator ────► systemd-run: olym activate
                      │                 lock → secrets → compose up → health
                      │                 → wait for confirm → switch or roll back
                      ▼
               [confirm] new SSH ──► olym confirm <id>
```

### 3.2 One binary, two roles

The same Go program is both the CLI on the user's machine and the **activator** on the server. Its Linux build is uploaded to `/var/lib/olympus/bin/` and runs the server-side steps. This keeps the lock, health check, rollback and timeout logic in one codebase, in one language, tested once.

### 3.3 Modules (proposed Go layout)

| Path | Responsibility | Runs on |
| --- | --- | --- |
| `cmd/olym/` | Entry point | both |
| `internal/cli/` | Commands, flags, output formatting | user |
| `internal/stack/` | Parse `stack.yaml`, resolve `${NAME}`, validate | user |
| `internal/schema/` | Embedded JSON Schemas; validation with line numbers | user |
| `internal/catalog/` | Fetch the recipe catalog at a revision, hash it, cache it | user |
| `internal/lock/` | Read, write, update `stack.lock`; resolve image digests | user |
| `internal/recipe/` | Load recipes; apply `supports` mappings | user |
| `internal/encode/` | Deterministic YAML, TOML and JSON encoders | user |
| `internal/eval/` | Build a generation from stack, lock and recipes; pure | user |
| `internal/generation/` | Generation layout, manifest, hashing | both |
| `internal/plan/` | Diff a built generation against the server's | user |
| `internal/remote/` | SSH sessions, uploads, remote commands, sudo | user |
| `internal/activate/` | Lock, secrets, Compose, health checks, confirm, rollback, gc | server |
| `internal/redact/` | Scrub known secret values from all output | both |
| `recipes/` | The official catalog (fetched as an input, not compiled in) | — |
| `schemas/` | `stack.schema.json`, `recipe.schema.json` (published) | — |

`internal/eval` must not import `internal/remote`, read the network, or read secret values. This keeps the build pure and testable offline.

## 4. Technology

### 4.1 Language: Go

A single static binary like Terraform's, built-in cross-compilation for Linux and macOS on amd64 and arm64, a standard library that covers templates, JSON, hashing and archives, and mature libraries for SSH and container registries. Terraform, OpenTofu, Docker and Compose are all Go. Rejected: TypeScript (needs a runtime or a bundled binary), Python (fragile packaging), Rust (slower to develop for one person, fewer libraries in this space).

### 4.2 Libraries (candidates)

Per the project rule against guessed dependencies, each is confirmed as existing and maintained before it is added.

| Need | Candidate | Note |
| --- | --- | --- |
| SSH client | `golang.org/x/crypto/ssh`, `.../ssh/knownhosts`, `.../ssh/agent` | Official Go sub-repository |
| CLI framework | `github.com/spf13/cobra` | Or the standard `flag` package if commands stay simple |
| YAML with line numbers | `github.com/goccy/go-yaml` | Line and column in errors (F-S3); check current maintenance |
| TOML | `github.com/pelletier/go-toml/v2` | For Codex-style configs |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` | Draft 2020-12 |
| Registry digests | `github.com/google/go-containerregistry` | Resolve tags to digests without Docker locally |
| Templates | `text/template` (standard library) | Only for files with no structured form |

### 4.3 Server software

Docker Engine and the Docker Compose v2 plugin (installed by Olympus if missing), systemd (present on all supported distributions), `sha256sum`, `tar`. Nothing else is installed on the server.

## 5. File formats

### 5.1 Stack file (`stack.yaml`)

```yaml
inputs:
  recipes: github:Haze-dev1/Olympus/recipes   # default catalog

server:
  host: 203.0.113.10
  user: root                 # or a sudo user
  port: 22
  ssh_key: ~/.ssh/id_ed25519 # optional; SSH agent otherwise
  keep: 5                    # generations kept by gc
  confirm_timeout: 60s

mcps:
  github:
    command: npx
    args: ["@modelcontextprotocol/server-github"]
    env: { GITHUB_TOKEN: ${GH_TOKEN} }
  docs:
    url: https://mcp.example.com/sse
    headers: { Authorization: "Bearer ${DOCS_TOKEN}" }

skills:
  - ./skills/deploy-checklist

agents:
  hermes:
    recipe: hermes
    mcps: [github]
    skills: all
    secrets: { model_key: ${ANTHROPIC_KEY} }
    channels:
      telegram: { token: ${TG_TOKEN}, allow: ["@haze"] }
  codex:
    recipe: codex
    mcps: [github, docs]
    secrets: { login_token: ${CODEX_TOKEN} }

apps:
  open-spaces:
    recipe: open-spaces
    ports: { public: [] }
```

Rules:

- Tool names (keys under `agents` and `apps`) match `^[a-z][a-z0-9-]{0,30}$`, are unique across both maps, and become Compose service names.
- `${NAME}` may appear only in values, never in keys, and only where the schema marks a field as secret-capable (`mcps.*.env`, `mcps.*.headers`, `secrets`, `channels.*.token`). A `${...}` elsewhere is a validation error, so a secret cannot leak into a rendered file by accident.
- `.env` is parsed as `NAME=value` lines; `#` starts a comment; values are not shell-expanded.
- Unknown keys are errors, not warnings.

### 5.2 Recipe (`recipes/<tool>/recipe.yaml`)

```yaml
name: hermes
kind: harness                     # harness | coding-agent | app
image: nousresearch/hermes-agent:latest
resources: { min_ram: 4GB }
user: "1000:1000"                 # container user; owns its secret files
volumes:
  data: /opt/data                 # named volume olympus_hermes_data
ports: [8642]                     # bound to 127.0.0.1
health:
  http: { port: 8642, path: /health }
  timeout: 120s
env:
  HERMES_HOME: /opt/data
secrets:
  model_key: { env: ANTHROPIC_API_KEY }        # delivered as an environment variable
supports:
  mcps:     { file: /opt/data/config.yaml, format: yaml, key: mcp_servers }
  skills:   { dir: /opt/data/skills }
  channels:
    telegram: { file: /opt/data/config.yaml, format: yaml, key: channels.telegram }
templates:
  - src: SOUL.md.tmpl
    dst: /opt/data/SOUL.md
hooks:
  post_install: ["hermes", "setup", "--non-interactive"]
```

The Hermes values above are illustrative and unverified; the real recipe is written against Hermes's documentation (PRD section 9.2).

Rules:

- `image` has a default tag; the lock pins its digest.
- Every `supports.*.file` and template `dst` must be inside one of the recipe's `volumes` paths (rule A3, section 9.2).
- `hooks.post_install` is an argument list, never a shell string, and runs inside the tool's container (rule A8).
- Recipes have no version field: the whole catalog is versioned by its git revision.

### 5.3 Lock file (`stack.lock`)

JSON, written only by Olympus, keys sorted, one trailing newline:

```json
{
  "version": 1,
  "inputs": {
    "recipes": {
      "url": "github:Haze-dev1/Olympus/recipes",
      "rev": "3f2a9c1e5b7d4e8a9c217d4e5f6a8b90c1d2e3f4",
      "hash": "sha256-…"
    }
  },
  "tools": {
    "codex":  { "image": "ghcr.io/…/codex@sha256:…" },
    "hermes": { "image": "nousresearch/hermes-agent@sha256:…" }
  }
}
```

JSON rather than YAML because nobody edits it by hand and its diffs stay simple. `image` pins the multi-architecture index digest, so one lock works for amd64 and arm64 servers.

### 5.4 Generation manifest (`manifest.json`)

```json
{
  "version": 1,
  "id": "sha256-<generation hash>",
  "inputs": { "recipes": "3f2a9c1e…" },
  "tools": {
    "hermes": {
      "hash": "sha256-<tool hash>",
      "image": "nousresearch/hermes-agent@sha256:…",
      "files": { "/opt/data/config.yaml": "sha256-…" },
      "secrets": ["model_key", "telegram_token"],
      "health": { "http": { "port": 8642, "path": "/health" }, "timeout": "120s" }
    }
  },
  "olym": "1.0.0"
}
```

Optional `health` carries the recipe HTTP check or argument-list command and an optional duration-string timeout. Older version-1 manifests may omit it. The activator manifest adapter validates and converts these values for both activation and rollback ([ADR 0028](ADR.md#0028-store-health-checks-in-the-generation-manifest)). Health changes alter generation identity; tool hashes remain file-and-Compose hashes.

Secret names are listed so `plan` can report added or removed secrets; values and their fingerprints never appear here.

## 6. Build (evaluation)

`check`, `build`, `plan` and `apply` all begin here. The build is a pure function:

```text
build(stack file, lock, catalog at locked rev, CLI version) → generation
```

It reads no secret values, no server state, no clock and no randomness, and sorts every map before output.

### 6.1 Steps

1. **Parse and validate** `stack.yaml` against `stack.schema.json`; report every error with line and column.
2. **Resolve the catalog.** If `stack.lock` exists, use its revision; fetch the catalog tarball for that revision into `~/.cache/olympus/inputs/<hash>/` unless cached; verify the hash. If the lock is missing, resolve the newest revision and write the lock (section 6.4).
3. **Load recipes** for every tool and validate them against `recipe.schema.json`.
4. **Check references:** every MCP name an agent lists exists; every channel is supported by the recipe (else error); every MCP server, skill or channel the recipe cannot accept produces a warning naming the tool.
5. **Render each tool** (section 6.2).
6. **Assemble** `compose.yaml` and `manifest.json`; compute hashes (section 6.3).

### 6.2 Rendering a tool

For each tool, the output is a set of files plus one Compose service.

- **Managed config files.** For each `supports.*.file`: start from the template output for that file if the recipe has one, else an empty document; decode it; set the managed key (`mcp_servers`, `channels.telegram`, …) to the data built from the stack file; encode with the deterministic encoder for its format. The core owns the managed key entirely; everything else in the file comes from the recipe.
- **Secrets inside managed files.** A value such as `${GH_TOKEN}` inside an MCP entry is never written into the file. It is replaced with a reference the tool resolves at runtime: an environment variable placeholder in the tool's own syntax where the tool supports one, otherwise the MCP server is started through a small wrapper that reads `/run/secrets/<name>`. Which mechanism each tool supports is recorded in its recipe (**open** per tool).
- **Skills.** Each listed skill folder is copied into the tool's output under `supports.skills.dir`. (**Open:** copy per tool, or one shared read-only mount.)
- **Plain templates.** `text/template` with the tool's entry, resolved MCP servers and skills as input, with `missingkey=error`. Secrets are not available to templates.
- **Compose service.** Built as a Go struct and encoded (section 7.3).

### 6.3 Hashing

- **File hash:** `sha256` of the file bytes.
- **Tool hash:** `sha256` over a canonical stream: for each file in sorted path order, `path`, NUL, byte length, NUL, bytes; then the canonical JSON of the tool's Compose service.
- **Generation hash:** `sha256` over the canonical JSON of the manifest without its `id`.
- Hashes are written as `sha256-<hex>`.

Because secrets are excluded, a secret change never changes a hash (PRD N-P2). Secret changes are detected separately (section 8.4).

### 6.4 Lock resolution

- **Catalog:** `update` (or the first `check` with no lock) asks the git host for the newest commit of the catalog's default branch, downloads its tarball, and computes a content hash over the extracted tree: SHA-256 over `olympus-catalog-v1` + NUL, then for each regular file sorted by relative slash path, the path, NUL, its permission bits in octal, NUL, size in decimal, NUL, contents and NUL. The result is `sha256-<hex>`; it is both the lock hash and the cache directory name. See [ADR 0023](ADR.md#0023-hash-the-extracted-catalog-tree-in-a-canonical-form).
- **Images:** for each tool, resolve the recipe's image tag (or the stack's `image_tag` override) to its index digest through the registry API, without a local Docker.
- `update` rewrites `stack.lock` beside the selected stack file and nothing else except the catalog cache. `check` creates a missing lock; `build` requires one. Both reject a missing tool entry and direct the user to `olym update`. An existing lock must match the selected catalog URL.
- The catalog source accepts `github:<owner>/<repo>[/<path>]`, defaulting to `github:Haze-dev1/Olympus/recipes`. The GitHub API's `commits/HEAD` endpoint resolves the default branch. `GITHUB_TOKEN`, when set, is used only for GitHub commit and tarball requests, including redirects confined to GitHub origins. It is never persisted or passed to registries, and response bodies and transport errors cannot echo it into diagnostics.
- Explicit `-recipes <directory>` is a development override for `check` and `build`. It announces that the catalog pin is skipped. Existing tool image pins remain mandatory; without a lock this mode permits local tagged artifacts without resolution. It retains the local content identity in the manifest. `update -recipes` is refused because local catalogs have no GitHub revision to pin.

### 6.5 Current offline slice

The M1 CLI implements `check`, `build` and `update` with catalog and lock resolution as specified in section 6.4. The CLI uses standard `flag`; `-f` selects the stack file and `build -o` selects its output directory, defaulting to `.olympus/build`. `check` validates the stack, recipes and rendering and checks names in the adjacent `.env`; `build` writes the rendered artifacts. `update` resolves catalog and images without requiring runtime secret names or writing a generation. Process environment fallback for stack secret references is deferred.

Published stack and recipe schemas are embedded. The core evaluates the schema vocabulary they use, with additional semantic checks for references and confinement. YAML aliases, anchors, tags, multiple documents and duplicate keys are rejected. String values that resemble YAML boolean or leading-zero numeric scalars must be quoted. Input documents and recipe sources are bounded to 4 MiB, and YAML data nesting to 64 levels. YAML integers retain integer types through validation and typed decoding. JSON normalization uses number-preserving decoding and concrete integer conversion before YAML/TOML/JSON encoding, including managed config seeds and int64 values above 2^53.

The local generation contains `compose.yaml`, `manifest.json` and `tools/<tool>/files/<container path>`. Compose uses relative read-only file binds and named data volumes. Catalog builds record the full locked catalog revision in manifest inputs and the locked image digests in Compose and manifest tools. Explicit local recipe overrides retain the hash of canonical recipe data and template source bytes. There is no tool-hash label because including a tool hash in the service being hashed would be circular. This is an offline artifact contract; server store assembly and hash labels belong to activation.

All ports remain bound to localhost. Public port requests and channel rendering fail explicitly in this slice. Unsupported MCPs or skills produce warnings and are omitted. Recipe hooks and health settings are validated and included in recipe input identity. Health checks are also persisted in each manifest tool for the activator; builds execute neither hooks nor checks. Skills, when requested and supported, are copied from confined local folders.

Only the fictional test recipe's MCP secret convention is exercised. Each MCP variable must map to an explicitly supplied recipe secret with an `env` target. Its config references that environment variable name. Compose points to `/var/lib/olympus/secrets/<tool>/environment`; no environment file or secret value is built. Real-tool secret syntax and delivery remain open under section 16. There are no wrapper scripts.

The build directory is tool-owned only when it contains a valid version-1 Olympus manifest whose generation ID matches its canonical contents. A complete new generation is written to a fresh sibling directory and atomically exchanged with prior owned output. The prior directory is then removed, eliminating stale files. An existing directory without a valid manifest, including an empty directory or a symlinked manifest, is refused. Staging and publication failures leave prior output intact. Native exchange and exclusive initial publication use Linux `renameat2` or macOS `renameatx_np`, on amd64/arm64. Other platforms and unsupported filesystems fail explicitly without a non-atomic fallback. This updates the original preservation policy in [ADR 0022](ADR.md#0022-atomically-replace-owned-local-build-directories). See [ADR 0020](ADR.md#0020-use-goccy-yaml-and-pelletier-toml-for-the-offline-core) and [0021](ADR.md#0021-keep-the-first-generation-contract-offline).

## 7. Server

### 7.1 Layout

```text
/var/lib/olympus/                     root:root 0700
  bin/olym -> olym-<ver>    activator binaries, by version
  lock                                    flock target
  lock.json                               who holds the lock: pid, user, command, started
  key                                     32 random bytes, 0600; fingerprints secrets (8.4)
  store/
    <tool hash>/files/<container path>    rendered files for one tool
    gen-<generation hash>/
      compose.yaml
      manifest.json
  generations/
    1 -> ../store/gen-<hash>
    2 -> ../store/gen-<hash>
  current -> generations/2
  secrets/<tool>/<name>                   0600, owned by the tool's container user
  secrets/<tool>/fingerprints.json        HMACs of current secret values
  state.json                              activation in progress: id, status, nonce, deadline
  log/<timestamp>-<command>.log           activator logs
```

Store paths are immutable once written: uploads go to `store/.incoming/<random>/` and are renamed into place only after their hash is verified, so a dropped connection never leaves a half-written entry.

### 7.2 Server preparation

On the first `plan` or `apply`, over SSH:

1. Verify the host key against `-known-hosts <path>`, else `OLYM_KNOWN_HOSTS`, else `~/.ssh/known_hosts`. An unknown host asks `y/N` on a terminal with the key fingerprint and is refused without a terminal; a changed key is always an error (rule A6). Use `server.ssh_key` (expanding a leading `~/`) or, when absent, the SSH agent.
2. Detect the OS from `/etc/os-release`; refuse anything but Ubuntu 22.04, 24.04 and Debian 12.
3. Check for root, or passwordless sudo; with neither, stop and print the exact command that needs it.
4. Install Docker Engine and the Compose plugin from Docker's apt repository if missing (only during `apply`).
5. Create `/var/lib/olympus/` and `key` if missing, only during `apply`.

The implemented `plan` slice only verifies SSH and root/passwordless sudo, then reads generation metadata and file hashes. It does not install software, create server files, or perform the future OS/capacity/port/image-architecture preflight.

### 7.3 Compose project

One project named `olympus`, one service per tool:

- `image:` the locked digest reference.
- `ports:` `127.0.0.1:<port>:<port>` for each recipe port; `0.0.0.0` only for ports listed in `ports.public` (rule A4).
- `volumes:` the recipe's named volumes as `olympus_<tool>_<name>`; rendered files bind-mounted read-only from `store/<tool hash>/files/…`.
- `secrets:` file-based Compose secrets from `secrets/<tool>/`, mounted at `/run/secrets/<name>`; or `env_file` for secrets the recipe delivers as environment variables.
- `restart: unless-stopped`; `labels:` tool name and tool hash.
- One project network; tools reach each other by service name. No host networking.

**Open:** some tools rewrite their own config file at runtime and fail on a read-only mount. Such recipes will mark files `writable: true`; the activator then copies them into the tool's volume instead of bind-mounting them.

## 8. Activation

### 8.1 Sequence of `apply`

```text
CLI                                         SERVER
 │ build generation G (offline)
 │ plan: read current manifest, verify current store hashes (drift)
 │── upload missing store entries ─────────► store/.incoming → store/<hash>
 │── upload activator if version missing ──► bin/olym-<ver> (checksum verified)
 │── secrets payload over stdin ───────────► incoming secrets file, 0600 (never argv)
 │── systemd-run olym activate G ───► activator (survives SSH disconnect)
 │                                            1. take flock; write lock.json; fail fast if held
 │                                            2. install secrets; update fingerprints
 │                                            3. docker compose pull
 │                                            4. docker compose up -d --remove-orphans
 │                                               (+ --force-recreate for tools whose secrets changed)
 │                                            5. health checks, per tool, in parallel
 │                                               fail → re-activate previous generation, exit
 │                                            6. state.json: awaiting-confirm, nonce, deadline
 │◄─ stream log until awaiting-confirm ──────
 │== NEW SSH connection ==
 │── olym confirm G <nonce> ─────────► 7. on confirm: generations/<n+1>, current → it
 │                                               release lock, exit 0
 │                                            7'. on deadline: re-activate previous, exit 3
 │◄─ result ─────────────────────────────────
```

The confirmation must come over a **new** SSH connection: it proves that the user can still reach the server after the change, which is what protects against a change that blocks SSH.

### 8.2 The lock

An exclusive `flock` on `/var/lib/olympus/lock`, held by the activator process for its whole run, and by `rollback` and `gc`. The kernel releases it when the process exits or dies, so a crashed activator never leaves a stale lock. `lock.json` describes the holder for the error message a second caller sees. `plan` takes no lock and reads a consistent snapshot through the `current` symlink.

### 8.3 Rollback

- **Automatic** (health failure or confirm timeout): the activator runs steps 2–5 for the previous generation, with its manifest and secrets fingerprints, then exits with status 3. If there is no previous generation (first deploy), it runs `docker compose down` for the project and leaves no `current`.
- **`rollback` command:** the activator re-activates the generation before `current`, with the same health checks and the same confirm-or-revert protection.
- Data volumes are never touched by a rollback.

### 8.4 Secrets

1. At `apply`, the CLI reads `.env` and builds a payload holding only the secrets each tool's recipe declares.
2. It sends the payload over the SSH session's standard input into a root-only file (`install -m 0600 /dev/stdin …`). Secret values never appear in command lines, environment variables of the SSH session, or logs.
3. The activator writes each secret to `secrets/<tool>/<name>`, mode 0600, owned by the tool's container user, and deletes the payload.
4. Change detection: the activator stores `HMAC-SHA256(server key, value)` per secret. `plan` fetches the key over SSH (root-only file), computes the same HMACs locally and compares, so it can report "secret changed" for a tool without either side storing or sending a plain hash. The server key is random per server, so fingerprints cannot be compared across servers.
5. Tools whose fingerprints changed are recreated with `--force-recreate`; others keep running.

### 8.5 Health checks

Run by the activator from the host, so images do not need `curl` or `wget`: an HTTP GET to `127.0.0.1:<port><path>` expecting a 2xx, or `docker exec <container> <command>` expecting exit 0. Each tool is polled every 2 seconds until it passes or its `timeout` (default 120 s) expires. All tools are checked in parallel; one failure fails the generation.

### 8.6 Garbage collection

`gc` keeps the newest `keep` generations (default 5) plus `current`, deletes older `generations/<n>` links, then deletes store entries no remaining generation references, and old activator binaries no generation was activated with. It reports space freed.

## 9. Security design

### 9.1 Threat model

Developers run the CLI on their own machine. It connects as root or sudo to a server they control and runs AI tools there that hold model API keys, chat bot tokens and code-access tokens. Recipes from the catalog become files and commands on that server. There is no web application, no database and no multi-user login in v1.

| Boundary | Crosses it | Main threats | Controls |
| --- | --- | --- | --- |
| Stack file, recipes → CLI | YAML, templates, hooks | Injection into shell, paths, templates | Schemas; argument lists; path confinement; templates without secrets |
| `.env` → CLI → server | Secret values | Leaks to output, logs, argv, files | Stdin transfer; 0600 files; redaction; no secrets in builds |
| CLI → server (SSH) | Root commands; the activator binary | MITM; command injection; swapped binary | Host key verification; quoted argument lists; checksum and version check |
| Git host → CLI | Catalog tarball | Tampered or moved revision | Content hash in the lock |
| Registry → server | Images | Swapped tags | Digest pinning |
| Server → internet | Tool ports, chat bots | Unauthenticated gateways; open bots | Localhost binding; required allow-lists |
| Container → host | Volumes, files | One tool reading another's secrets | Per-tool secret files; no Docker socket mounts; no host networking |

### 9.2 Rules enforced in code

| # | Rule | Where enforced |
| --- | --- | --- |
| A1 | Secret values never in the stack file, lock, plan output, logs, errors or process arguments | `stack` (secret-capable fields only), `redact` on every output stream, stdin transfer |
| A2 | Remote commands are argument lists, each argument shell-quoted | `remote`: the only way to run a command; no raw string API |
| A3 | Rendered file destinations resolve inside the tool's own volumes | `recipe` validation; `eval` checks the cleaned path |
| A4 | Ports bind to 127.0.0.1 unless `ports.public` lists them | `eval` Compose builder |
| A5 | Each tool gets only its declared secrets; files 0600 | `activate` |
| A6 | SSH host keys verified; never disabled silently | `remote` with `knownhosts` |
| A7 | Images pulled by digest once locked | `eval` refuses an unlocked image |
| A8 | Recipe hooks run inside the tool's container | `activate` uses `docker compose exec` |
| A9 | YAML parsed with a decoder that cannot construct arbitrary types | `stack`, `recipe` decode into typed structs only |
| A10 | Chat channels require a non-empty allow-list | Schema plus `eval` check |
| A11 | Only the `olympus` Compose project and `/var/lib/olympus/` are changed | `activate` scopes every command to the project |
| A12 | The activator runs only after its checksum and version match the CLI's release | `remote` verifies before `systemd-run` |

### 9.3 Redaction

Every secret value read from `.env` is registered with `internal/redact` before any output is produced. All CLI output, logs and error messages pass through it, replacing each value with `«redacted:NAME»`. This is a second line of defence; code must still never format secret values.

## 10. Commands

| Command | Server access | Changes server | Notes |
| --- | --- | --- | --- |
| `check` | no | no | Validates; writes `stack.lock` only if missing |
| `build [-o dir]` | no | no | Writes the generation to `./.olympus/build/` |
| `plan [-known-hosts path] [--detailed-exitcode]` | read | no | Builds in memory and compares the current generation; saved plans (`-out`) are deferred |
| `apply [file] [--no-confirm]` | write | yes | `--no-confirm` skips the confirmation wait and warns that lost access will not roll back |
| `status [--json]` | read | no | Tool state, health, image, generation number |
| `rollback` | write | yes | Previous generation, with confirm-or-revert |
| `update` | no | no | Rewrites `stack.lock` |
| `gc` | write | yes | Section 8.6 |
| `destroy [--delete-data]` | write | yes | `docker compose down`; keeps volumes and `/var/lib/olympus/` unless `--delete-data` |
| `activate`, `confirm` | — | — | Internal; run on the server by the CLI |

### 10.1 Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success; with `--detailed-exitcode`, `plan` found no changes |
| 1 | Error; nothing was changed, or the change was rolled back before it started |
| 2 | `plan --detailed-exitcode` found changes |
| 3 | `apply` or `rollback` failed after activation began; the server rolled back to the previous generation |
| 4 | The server lock is held by another run |

### 10.2 Plan output

`plan` uses the same stack, catalog and lock preparation as `build`, including explicit `-recipes` development overrides. It never writes `.olympus/build`. It does not read runtime secret values or require their presence. Secret value change detection is deferred until server fingerprints exist; changes to declared secret names appear as a generic configuration change.

Resolve `current -> generations/<n>` once and resolve that numbered link to `/var/lib/olympus/store/gen-<manifest-id>`. Read the bounded manifest there, verify its version, identity and canonical hash, then hash each recorded file at `tools/<tool>/files/<container path>` in that same generation with `sha256sum`. An absent current is an empty server; malformed/dangling links, unreadable state and hashing failures are errors. A missing regular file counts as drift. Non-root readers first verify `sudo -n id -u`, then run all generation reads with `sudo -n`. All commands use quoted argument lists; no shell script or write command runs remotely.

Output lists added/removed tools, changed file paths, image changes and drift paths, plus an unchanged count and add/remove/restart totals. Other tool-manifest changes, including health or secret-name changes, use `configuration changed`. File contents, old/new values and semantic managed-key diffs are not printed. Drift makes `--detailed-exitcode` return 2 even when the desired tool manifest is unchanged. Default success is 0; errors are 1.

The example below illustrates the wider v1 output, including future semantic and secret-value diffs:

```text
Plan: stack "stack.yaml" → root@203.0.113.10 (generation 2 → new)

  + opencode     new tool
  ~ hermes       config: /opt/data/config.yaml (mcp_servers: +docs)
  ~ codex        secret changed: login_token
  ! open-spaces  drift: /app/config.json changed on server
  - old-agent    removed

3 to restart, 1 to add, 1 to remove. Volumes are kept.
```

## 11. Failure handling

| Situation | Behaviour | Exit |
| --- | --- | --- |
| Invalid stack file or recipe | Report all errors with line and column; no server contact | 1 |
| Missing secret | Name the variable and every line using it | 1 |
| Unknown or changed host key | Prompt for unknown (show fingerprint); refuse changed | 1 |
| Unsupported OS, no sudo | Stop before changes; print what is needed | 1 |
| Lock held | Print holder from `lock.json` | 4 |
| Connection drops during upload | Nothing activated; partial uploads stay in `.incoming` and are cleaned next run | 1 |
| Image pull fails | Activator rolls back; reports image and registry error | 3 |
| Health check fails | Activator rolls back; reports tool and its last 50 log lines | 3 |
| Connection drops after activation began | Activator continues; rolls back at the confirm deadline | 3 on next contact |
| New generation blocks SSH | CLI cannot confirm; server rolls back by itself | 3 |
| Confirmation after the deadline | `confirm` reports the server already rolled back | 3 |
| Activator checksum or version mismatch | Re-upload; refuse to run a mismatched binary | 1 |
| `rollback` with no previous generation | Refuse | 1 |
| Rolling back to a generation removed by `gc` | Refuse; name missing store entries | 1 |

## 12. Testing

| Level | What | How |
| --- | --- | --- |
| Unit | Parsers, schemas, encoders, path confinement, redaction, plan diff | Go tests, table-driven |
| Golden | `build` output for sample stacks | Committed expected generations under `testdata/`; byte comparison |
| Purity | Same inputs → identical output; different secret values → identical output; shuffled map order → identical output | Build twice in one test and compare hashes |
| Recipe | Every catalog recipe validates and builds; image digests resolve | CI job over `recipes/` |
| Integration | The full cycle on a real server: apply, re-apply, upgrade, rollback, kill-the-CLI, destroy | The disposable lab VM from `lab/lab.sh` ([ADR 0025](ADR.md#0025-test-against-a-local-qemu-vm-from-the-ubuntu-cloud-image)); one real VPS run before release |
| Security | Rules A1–A12; leak search using sentinel secret values (`OLYM_SENTINEL_…`) in all output, logs, lock and server files | Integration suite plus the project's security review skill |

Every case in [edgecases/apply.md](edgecases/apply.md) becomes a test at one of these levels, or is listed as a gap.

## 13. Build and release

- Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, built with `CGO_ENABLED=0`.
- Each release publishes the binaries and a `SHA256SUMS` file. The CLI embeds the checksums of its own Linux builds at build time, so it can verify the activator it uploads (rule A12).
- A CLI running on macOS downloads the matching Linux build of its own version, verifies it against the embedded checksum, and caches it. A CLI on Linux with the server's architecture uploads itself.
- Development builds take `--activator <path>` and skip the release check with a visible warning.
- Release tooling (**proposed:** GoReleaser) is chosen in M1; it can build the archives, `SHA256SUMS`, the Homebrew formula and the AUR package from one config.

### 13.1 Names and paths

| Thing | Name |
| --- | --- |
| Project | Olympus |
| Command and binary | `olym` |
| Server directory | `/var/lib/olympus/` |
| Compose project, volume prefix | `olympus`, `olympus_<tool>_<volume>` |
| Local cache, build output | `~/.cache/olympus/`, `./.olympus/build/` |
| Activator on the server | `/var/lib/olympus/bin/olym-<version>` |

### 13.2 Distribution

| Channel | Package | Notes |
| --- | --- | --- |
| GitHub Releases | `olym_<version>_<os>_<arch>.tar.gz` plus `SHA256SUMS` | The source every other channel downloads from |
| Homebrew | Formula `olym` in the project's own tap (`<owner>/homebrew-tap`), installed with `brew install <owner>/tap/olym` | homebrew-core later, once the project meets its notability rules |
| AUR | `olym` | AUR convention names a prebuilt-binary package `olym-bin` and a build-from-source package `olym`; decide which to publish (**open**) |

Before the first release, check that `olym` is not already taken in Homebrew and the AUR (**open**).

## 14. Performance budget

| Operation | Budget | Main cost |
| --- | --- | --- |
| `check` / `build`, 10 tools, warm cache | < 2 s | Schema validation, encoding |
| First catalog fetch | < 10 s | Tarball download |
| `plan` | < 10 s | One SSH connection, manifest read, store hash check |
| `apply` overhead | < 30 s beyond pulls, starts and health checks | Uploads, confirmation reconnect |

## 15. Decisions summary

| Decision | Reason | Rejected |
| --- | --- | --- |
| Go, single binary | Distribution like Terraform; ecosystem | TypeScript, Python, Rust |
| YAML stack file | Familiar from Compose; low learning cost | HCL, Nix, code |
| Declarative YAML recipes in one catalog | Cheap to add tools; reviewable | Recipes as code; any git URL in v1 |
| Catalog pinned by git revision and hash | One pin for all recipes, like `nixpkgs` | Per-recipe versions |
| Lock file with image digests | Tags move; digests do not | Tags only |
| Local pure build, server activation | Offline testing; rollback; no state file to lose | Terraform-style local state; container labels as state |
| One Compose project | Compose already reconciles containers | Docker API directly; systemd units; Kubernetes |
| Activator on the server with confirm-or-rollback | Survives dropped connections and lockouts | CLI-driven activation; a resident daemon |
| Structured config built as data | No templating bugs; stable hashes | Text templates for everything |
| Secrets over stdin, HMAC fingerprints | Never in argv, files under git or hashes | Secrets in rendered files; plain hashes |
| Existing server over SSH in v1 | Smallest useful scope; works for home servers | Provisioning cloud servers first |
| Name Olympus, command `olym`; Homebrew tap and AUR | Short command; no third-party trademark in the name; channels that need no approval | Keeping "AITerraform"; `olympus` as the command; homebrew-core first |
| Private by default | Exposed agent gateways are a known failure | Public ports behind each tool's own login |

## 16. Open technical questions

- [ ] Per tool: how secrets inside MCP entries are delivered (environment placeholder or wrapper reading `/run/secrets`).
- [ ] Skills: copy into each tool, or one shared read-only mount.
- [ ] Tools that rewrite their own config files: confirm the `writable: true` copy approach.
- [x] Integration test servers: a local QEMU VM from the Ubuntu cloud image ([ADR 0025](ADR.md#0025-test-against-a-local-qemu-vm-from-the-ubuntu-cloud-image)), plus one VPS run before release.
- [ ] The final URL scheme for the catalog input and whether `git` is ever required locally.
- [ ] CLI framework: `cobra` or the standard library.
