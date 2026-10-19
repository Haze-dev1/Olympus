# Spec: stack file (`stack.yaml`)

- **Status:** Draft v1 format; offline subset implemented
- **Date:** 2026-10-19
- **Decisions:** [ADR 0004](../ADR.md#0004-use-yaml-for-the-stack-file), [0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input), [0009](../ADR.md#0009-keep-secrets-in-a-git-ignored-env-referenced-by-name), [0012](../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)

The stack file describes everything that should run on one server. Users write it and commit it. Field names here are proposals and will change as the first recipes are written.

## Example

```yaml
inputs:
  recipes: github:haze/olympus/recipes

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
| `recipes` | Git source of the recipe set, such as `github:haze/olympus/recipes`. v1 accepts only the official repo ([ADR 0006](../ADR.md#0006-keep-official-recipes-in-one-folder-in-this-repo)). The revision is pinned in `stack.lock`, not here. |

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

`olym check -f stack.yaml` validates this file and requires each referenced name to exist in `.env` beside it. It does not resolve values or use process environment fallback. `olym build` additionally loads recipes from `-recipes recipes`, with no fetching, digest resolution or lock file. See [TRD section 6.5](../TRD.md#65-current-offline-slice) for the exact implemented boundary.

## Variables

`${NAME}` is replaced with the value of `NAME` from `.env` next to the stack file, then from the process environment. A missing variable is an error that names the variable and every place it is used. Values are never printed.

## Validation

`check`, `plan` and `apply` validate against the published JSON Schema ([ADR 0018](../ADR.md#0018-build-structured-config-as-data-and-publish-json-schemas)) before connecting to the server, and report each error with file, line and field. Editors that support YAML schemas can use the same schema for autocomplete. See [edgecases/apply.md](../edgecases/apply.md) for the error cases.

## Lock file (`stack.lock`)

Written by Olympus only (`check` when missing, and `update`). JSON with sorted keys, so diffs stay small:

```json
{
  "version": 1,
  "inputs": { "recipes": { "url": "github:haze/olympus/recipes", "rev": "3f2a9c1e…", "hash": "sha256-…" } },
  "tools": { "hermes": { "image": "nousresearch/hermes-agent@sha256:…" } }
}
```

`image` pins the multi-architecture index digest. Every later run uses the locked revision and digests; `update` moves them forward ([ADR 0008](../ADR.md#0008-pin-exact-versions-in-stacklock), [0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)).

## Generations and server layout

Defined in [TRD.md](../TRD.md) sections 5.4 (manifest), 7.1 (server layout) and 8 (activation). In short: rendered tool output lives in `/var/lib/olympus/store/<hash>/`, each generation is a numbered link to a content-addressed `store/gen-<hash>/` holding `compose.yaml` and `manifest.json`, `current` points at the active one, and secrets live in `secrets/<tool>/` outside every generation so hashes never depend on them.
