# Spec: stack file (`stack.yaml`)

- **Status:** Draft v1 format; offline subset implemented
- **Date:** 2026-10-19
- **Decisions:** [ADR 0004](../ADR.md#0004-use-yaml-for-the-stack-file), [0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input), [0009](../ADR.md#0009-keep-secrets-in-a-git-ignored-env-referenced-by-name), [0012](../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)

The stack file describes everything that should run on one server. Users write it and commit it. Field names here are proposals and will change as the first recipes are written.

## Example

```yaml
inputs:
  recipes: github:Haze-dev1/Olympus/recipes

server:
  host: 1.2.3.4
  user: root
  ssh_key: ~/.ssh/id_ed25519

mcps:
  github:
    command: npx
    args: ["@modelcontextprotocol/server-github"]
    env: { GITHUB_TOKEN: ${GH_TOKEN} }

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
    mcps: [github]

apps:
  open-spaces:
    recipe: open-spaces
```

## Top-level keys

| Key | Required | Type | Meaning |
| --- | --- | --- | --- |
| `inputs` | no | map | Where recipes come from. Defaults to the official recipes repo. |
| `server` | yes | map | The server to set up |
| `mcps` | no | map of name → MCP server | MCP servers available to agents |
| `skills` | no | list of paths | Skill folders (each with a `SKILL.md`), relative to the stack file |
| `agents` | no | map of name → entry | Agents to run |
| `apps` | no | map of name → entry | Non-agent tools to run, such as workspaces |

At least one of `agents` or `apps` must be present.

## `inputs`

| Key | Meaning |
| --- | --- |
| `recipes` | GitHub source of the recipe set: `github:<owner>/<repo>[/<path>]`. Defaults to `github:Haze-dev1/Olympus/recipes`. Other GitHub catalogs are supported ([ADR 0026](../ADR.md#0026-wire-github-catalogs-and-preserve-local-development-overrides)). The revision is pinned in `stack.lock`, not here. |

## `server`

| Key | Required | Default | Meaning |
| --- | --- | --- | --- |
| `host` | yes | | IP address or hostname |
| `user` | no | `root` | SSH user; needs sudo if not root |
| `port` | no | `22` | SSH port |
| `ssh_key` | no | SSH agent | Path to a private key |
| `keep` | no | `5` | Generations to keep on the server; `gc` removes older ones |
| `confirm_timeout` | no | `60s` | How long the activator waits for confirmation before rolling back ([ADR 0017](../ADR.md#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms)) |

## `mcps.<name>`

Either a local command (stdio) or a remote URL (HTTP), not both.

| Key | Meaning |
| --- | --- |
| `command`, `args` | Program to start for a stdio MCP server |
| `url`, `headers` | Endpoint for an HTTP MCP server |
| `env` | Environment variables for the server; values may be `${NAME}` |

## `agents.<name>` and `apps.<name>`

| Key | Required | Meaning |
| --- | --- | --- |
| `recipe` | yes | Recipe name from the `recipes` input |
| `image_tag` | no | Override the recipe's default image tag, for example to hold one tool back; the digest is still locked |
| `mcps` | no | Names from the top-level `mcps` this tool gets |
| `skills` | no | `all`, or a list of paths from the top-level `skills` |
| `secrets` | no | Values for the secrets the recipe declares |
| `channels` | no | Channel settings, for recipes that declare channel support |
| `ports.public` | no | Ports to open publicly. Off by default ([ADR 0010](../ADR.md#0010-expose-nothing-publicly-by-default)). |

## Current offline commands

`olym check -f stack.yaml` validates the stack, recipes and rendering, requiring each referenced secret name in the adjacent `.env` without resolving values. It creates a missing adjacent `stack.lock` from the newest default-branch commit and image index digests. `olym update` refreshes that lock without rendering output or checking runtime secret names. `olym build` requires an existing lock. Both `check` and `build` verify the catalog hash, use locked digests and reject missing tool entries. Change the catalog URL through `update`, rather than silently reusing an unrelated lock.

`GITHUB_TOKEN` from the process environment authenticates only GitHub commit lookups and tarball downloads. It is never stored or printed and is never forwarded to registries or other redirect origins. Public catalogs work without it.

Explicit `-recipes <directory>` on `check` or `build` selects local development recipes and announces that the catalog pin is skipped. An existing lock still supplies every tool's image digest; without a lock, local artifacts may use tags and no lock is created. Local manifests keep a content hash instead of a catalog revision. `update -recipes` is refused. See [TRD section 6.5](../TRD.md#65-current-offline-slice) for the remaining offline artifact boundaries.

## Variables

`${NAME}` is replaced with the value of `NAME` from `.env` next to the stack file, then from the process environment. A missing variable is an error that names the variable and every place it is used. Values are never printed.

## Validation

`check`, `plan` and `apply` validate against the published JSON Schema ([ADR 0018](../ADR.md#0018-build-structured-config-as-data-and-publish-json-schemas)) before connecting to the server, and report each error with file, line and field. Editors that support YAML schemas can use the same schema for autocomplete. See [edgecases/apply.md](../edgecases/apply.md) for the error cases.

## Lock file (`stack.lock`)

Written by Olympus only (`check` when missing, and `update`). JSON with sorted keys, so diffs stay small:

```json
{
  "version": 1,
  "inputs": { "recipes": { "url": "github:Haze-dev1/Olympus/recipes", "rev": "3f2a9c1e…", "hash": "sha256-…" } },
  "tools": { "hermes": { "image": "nousresearch/hermes-agent@sha256:…" } }
}
```

`image` pins the multi-architecture index digest. Every later run uses the locked revision and digests; `update` moves them forward ([ADR 0008](../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)). `hash` is the canonical hash of the extracted catalog tree, defined in [ADR 0023](../ADR.md#0023-hash-the-extracted-catalog-tree-in-a-canonical-form).

## Generations and server layout

Defined in [TRD.md](../TRD.md) sections 5.4 (manifest), 7.1 (server layout) and 8 (activation). In short: rendered tool output lives in `/var/lib/olympus/store/<hash>/`, each generation is a numbered link to a content-addressed `store/gen-<hash>/` holding `compose.yaml` and `manifest.json`, `current` points at the active one, and secrets live in `secrets/<tool>/` outside every generation so hashes never depend on them.

Local builds mirror rendered config entries at `store/<tool hash>/files/<container path without leading slash>`. Compose mounts their absolute `/var/lib/olympus/store/...` paths. Tool hashes cover files and service configuration before adding those mounts. Generation uploads contain only Compose and manifest files; each tool entry is separately verified and reused. See [ADR 0030](../ADR.md#0030-address-rendered-files-by-tool-hash-across-generations).

## Saved plans and image overrides

`olym plan -out plan.json` writes version-1 JSON with `version`, `generation` (desired manifest id), `current` (server manifest id or empty), `server` (SSH user and host:port), and `fingerprints` (tool to domain-separated HMAC map). It never contains secret values or the server key. Before the first activation the fingerprint entries are null because no server key exists; the first apply reads current .env values rather than persisting an unkeyed hash. `olym apply plan.json --yes` re-evaluates and refuses any mismatch before preparation; the expected current id is checked again under the server lock. Without `--yes`, apply asks y/N. Confirmation over fresh SSH is always required.

A version-1 `stack.lock` tool now accepts optional `image_tag`, the exact stack override used during update. Omission means no override. Any mismatch between stack and lock fails with `run olym update`, including an override paired with an older lock lacking the field. See [ADR 0033](../ADR.md#0033-record-image-tag-overrides-in-the-image-lock).
