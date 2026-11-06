# Spec: recipe (`recipes/<tool>/recipe.yaml`)

- **Status:** Draft v1 format; local loading and offline rendering implemented
- **Date:** 2026-10-20
- **Decisions:** [ADR 0005](../ADR.md#0005-describe-each-tool-with-a-declarative-yaml-recipe), [0006](../ADR.md#0006-keep-official-recipes-in-one-folder-in-this-repo), [0012](../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool), [0035](../ADR.md#0035-build-tools-without-an-official-image-from-a-recipe-supplied-dockerfile), [0036](../ADR.md#0036-deliver-writable-managed-config-with-a-compose-init-service), [0037](../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references), [0038](../ADR.md#0038-mount-shared-skills-read-only-where-supported-and-copy-them-where-the-tool-writes), [0039](../ADR.md#0039-configure-hermes-channels-as-environment-variables), [0045](../ADR.md#0045-render-recipe-runtime-limits-as-fixed-compose-keys)

A recipe tells the core how to install, run and configure one tool. Recipes live in `recipes/` in this repo and are versioned together, by the git revision pinned in `stack.lock` ([ADR 0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)). The core turns each recipe into one Docker Compose service ([ADR 0015](../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)). The shipped `recipes/hermes`, `recipes/codex`, `recipes/opencode` and `recipes/openbot` are the current examples; each folder's README names its sources and what is still unverified.

The implemented offline subset is described in [TRD section 6.5](../TRD.md#65-current-offline-slice). The fictional `testdata/sample/recipes/observatory` recipe verifies rendering only; it is not a supported production tool.

## Fields

| Key | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Recipe name, matching its folder |
| `kind` | yes | `harness`, `coding-agent` or `app` |
| `image` | yes | Container image with a default tag; the lock pins its digest |
| `resources.min_ram` | no | Minimum memory as a size such as `4GB` or `512MiB` (decimal `KB`/`MB`/`GB`/`TB`, binary `KiB`/`MiB`/`GiB`/`TiB`); the preflight warns if the server has less and `apply` continues only after confirmation |
| `volumes` | no | Map of name → path in the container. Kept across upgrades and by `destroy` unless asked. |
| `ports` | no | Ports the tool listens on; bound to localhost |
| `command` | no | Argument list that replaces the image's default command |
| `env` | no | Non-secret environment variables for the service |
| `health` | no | `port` and `path` for an HTTP check, or `command`; optional `timeout` |
| `secrets` | no | Secret names the tool needs and the environment variable each is delivered as; the stack file provides values |
| `supports` | no | What shared features the tool accepts (below) |
| `writable` | no | Managed files the tool rewrites at runtime; delivered by the init service (below) |
| `build` | no | `src` Dockerfile in the recipe folder and `dst` path inside a volume; the core builds the image with Compose (below) |
| `templates` | no | `source -> destination` pairs for files with no structured form. Never used for YAML, TOML or JSON keys the core manages through `supports` ([ADR 0018](../ADR.md#0018-build-structured-config-as-data-and-publish-json-schemas)). |
| `hooks.post_install` | no | Command run once inside the container after first start. Escape hatch; prefer declarative fields. |
| `runtime` | no | Container runtime limits rendered as fixed Compose keys: `shm_size`, `stop_grace_period`, `no_new_privileges` and `seccomp` (below) ([ADR 0045](../ADR.md#0045-render-recipe-runtime-limits-as-fixed-compose-keys)). |

## `supports`

| Key | Meaning |
| --- | --- |
| `mcps.file` | Path of the config file that holds MCP servers |
| `mcps.format` | `yaml`, `toml` or `json` |
| `mcps.style` | Tool's shape and secret syntax: `hermes`, `codex`, `opencode`, `claude-code` or `gemini-cli` ([ADR 0037](../ADR.md#0037-encode-mcp-secrets-as-tool-native-environment-references), [0047](../ADR.md#0047-carry-an-explicit-remote-transport-for-mcp-servers)) |
| `mcps.key` | Key under which servers are listed, such as `mcp_servers` |
| `skills.dir` | Folder the tool reads skills from |
| `skills.mode` | `mount` for a read-only bind, or `copy` for the init service ([ADR 0038](../ADR.md#0038-mount-shared-skills-read-only-where-supported-and-copy-them-where-the-tool-writes)) |
| `channels.<name>.env` | Environment variables that configure the channel: `token`, `allow`, and optional `roles` ([ADR 0039](../ADR.md#0039-configure-hermes-channels-as-environment-variables)) |

If a stack file gives a tool MCP servers, skills or channels its recipe does not support, `plan` warns and names the tool.

Every `file`, `dir`, `dst` and `writable` path must resolve strictly inside one of the recipe's volume paths. Paths that the init service puts into a shell script must also be shell-safe.

## Runtime options

```yaml
runtime:
  shm_size: 1g
  stop_grace_period: 60s
  no_new_privileges: true
  seccomp: seccomp.json
```

| Key | Meaning |
| --- | --- |
| `shm_size` | Shared-memory size, a positive size in the `resources.min_ram` units. Rendered as Compose `shm_size`. |
| `stop_grace_period` | Positive duration, for example `60s` or `1m30s`. Rendered as Compose `stop_grace_period`. |
| `no_new_privileges` | Boolean, default `false`; `true` adds `no-new-privileges:true` to `security_opt`. Opt-in per recipe ([ADR 0045](../ADR.md#0045-render-recipe-runtime-limits-as-fixed-compose-keys)). |
| `seccomp` | Relative path to a seccomp profile inside the recipe folder. Never a URL, absolute path or traversal. |

`seccomp` is read with the same confinement checks as a template source and must name a regular file no larger than the template limit. The profile becomes a generated tool file at `<first volume>/.olympus/seccomp.json`: it is hashed into the tool and appears in `plan` as a config change, is uploaded under `store/<tool hash>/files/…` (that is, under `/var/lib/olympus` on the server), and bind-mounted read-only. Compose receives `security_opt: [seccomp=/var/lib/olympus/store/<tool hash>/files/<path>]`. The recipe must declare at least one volume; a profile destination that collides with a template or managed file, or that is also listed under `writable`, is an error. Olympus does not parse the profile as JSON; a malformed profile fails when Docker starts the container.

## MCP translation

The stack defines each MCP server once. The core rewrites its `${NAME}` references to the environment variable of the recipe secret that provides them, then encodes it in the tool's own shape; no secret value is written to a file.

- **Hermes:** `command`, `args`, `env` with `${VAR}`, or `url`, `headers`.
- **Codex:** `command`, `args`, `env_vars` (variable names only), or `url`, `bearer_token_env_var` / `env_http_headers`. Codex has no interpolation, so anything that is neither a whole variable reference nor an HTTP header reference fails closed.
- **OpenCode:** `{type: local, command: [...], environment: {VAR: "{env:VAR}"}}`, or `{type: remote, url, headers: {H: "{env:VAR}"}}`.
- **Claude Code:** `command`, `args`, `env` with `${VAR}`, or `type`, `url`, `headers`, where `type` is `http` for the default transport or `sse` when the stack asks for it. Claude Code rejects a remote entry without `type`.
- **Gemini CLI:** `command`, `args`, `env` with `${VAR}`, or `httpUrl` and `headers` for the default transport, or `url` and `headers` for `sse`. Exactly one of the two URL keys is written, because Gemini CLI prefers `httpUrl` and would otherwise ignore the other.

`transport` is meaningless for stdio servers and for tools that only accept one remote form; those recipes ignore it. Only Claude Code and Gemini CLI render it ([ADR 0047](../ADR.md#0047-carry-an-explicit-remote-transport-for-mcp-servers)).

Compose delivers the actual values as environment variables through `/var/lib/olympus/secrets/<tool>/environment`.

## Writable files and copied skills

A tool that rewrites its own config cannot use a read-only bind. The evaluator removes that bind and creates a one-shot `<tool>-init` service. The init service mounts the rendered file read-only under `/olympus-init/<n>`, mounts the tool's named volumes, and copies the file into place before the tool starts; the tool declares `depends_on: service_completed_successfully` ([ADR 0036](../ADR.md#0036-deliver-writable-managed-config-with-a-compose-init-service)). `supports.skills.mode: copy` uses the same path.

The stack file and the recipe set remain the source of truth: edits the tool makes to a managed file are overwritten on the next apply. In `copy` mode, the init service records copied file paths in `.olympus-managed-files` and prunes those files before copying the current selection, including an empty selection. Unrecorded files and directories survive; copies predating the ledger remain untouched. Unsafe ledger paths and symlink ancestors fail init ([ADR 0042](../ADR.md#0042-track-and-prune-only-previously-copied-skill-files)).

## Built images

`image` must still resolve in a registry so the lock can pin a digest. When `build` is present, the lock pins the base image's digest and Compose builds the tool image on the server from the recipe's Dockerfile:

1. the Dockerfile is a generation file at `build.dst`, bind-mounted read-only;
2. the service gets a content-addressed local tag, `olympus/<tool>:sha256-<12 hex>`, derived from the Dockerfile bytes and the pinned base reference;
3. Compose `build.context` is the directory holding the generated Dockerfile, `build.dockerfile` is its base name, and `BASE_IMAGE` carries the pinned base reference ([ADR 0035](../ADR.md#0035-build-tools-without-an-official-image-from-a-recipe-supplied-dockerfile)).

The tool and its init service share the build declaration. Activation pulls with `--ignore-buildable`, builds each missing local image once, then starts with `--no-build --pull never`. Existing built images are reused during rollback. Package installs in recipe Dockerfiles must use exact versions; Codex also fixes build and installed-file timestamps ([ADR 0041](../ADR.md#0041-prepare-content-addressed-built-images-before-starting-services)).

## Templates

The core builds the Compose service and every `supports` config as data and encodes it, so recipes describe structure, not raw config text. A JSON Schema for recipes is published with the core and used by `check`.

Templates receive the agent's entry from the stack file and the resolved MCP servers and skills. They never receive secret values: a secret appears as a reference (an environment variable name), so a tool's hash does not change when a secret does ([ADR 0014](../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)). Go's `text/template` is used with `missingkey=error`.

## Checklist for a new recipe

- Default image tag set and its digest resolvable
- Every port bound to localhost (A4)
- Secrets declared, none hard-coded (A1)
- Data volumes named, so they survive upgrades
- Health check defined, and reachable without authentication
- Managed files the tool rewrites listed under `writable`
- Skills mounted read-only unless the tool writes them
- Tested on a clean server with a stack file using only this tool
