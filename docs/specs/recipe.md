# Spec: recipe (`recipes/<tool>/recipe.yaml`)

- **Status:** Draft, not implemented
- **Date:** 2026-10-19
- **Decisions:** [ADR 0005](../ADR.md#0005-describe-each-tool-with-a-declarative-yaml-recipe), [0006](../ADR.md#0006-keep-official-recipes-in-one-folder-in-this-repo), [0012](../ADR.md#0012-define-mcp-servers-and-skills-once-and-translate-them-per-tool)

A recipe tells the core how to install, run and configure one tool. Recipes live in `recipes/` in this repo and are versioned together, by the git revision pinned in `stack.lock` ([ADR 0016](../ADR.md#0016-pin-the-recipe-set-as-a-flake-style-input)). The core turns each recipe into one Docker Compose service ([ADR 0015](../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)). A draft example is in [prototypes/recipes/hermes/recipe.yaml](../prototypes/recipes/hermes/recipe.yaml); its paths and ports are not yet checked against Hermes's docs.

## Fields

| Key | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Recipe name, matching its folder |
| `kind` | yes | `harness`, `coding-agent` or `app` |
| `image` | yes | Container image with a default tag; the lock pins its digest |
| `resources.min_ram` | no | Minimum memory; `plan` warns if the server has less |
| `volumes` | no | Map of name → path in the container. Kept across upgrades and by `destroy` unless asked. |
| `ports` | no | Ports the tool listens on; bound to localhost |
| `health` | no | `port` and `path` for an HTTP check, or `command` |
| `secrets` | no | Secret names the tool needs; the stack file provides values |
| `supports` | no | What shared features the tool accepts (below) |
| `templates` | no | `source -> destination` pairs for files with no structured form. Never used for YAML, TOML or JSON keys the core manages through `supports` ([ADR 0018](../ADR.md#0018-build-structured-config-as-data-and-publish-json-schemas)). |
| `hooks.post_install` | no | Command run once inside the container after first start. Escape hatch; prefer declarative fields. |

## `supports`

| Key | Meaning |
| --- | --- |
| `mcps.file` | Path of the config file that holds MCP servers |
| `mcps.format` | `yaml`, `toml` or `json` |
| `mcps.key` | Key under which servers are listed, such as `mcp_servers` |
| `skills.dir` | Folder the tool reads skills from |
| `channels` | List of supported channels, such as `[telegram, discord]` |

If a stack file gives a tool MCP servers, skills or channels its recipe does not support, `plan` warns and names the tool.

## Templates

The core builds the Compose service and every `supports` config as data and encodes it, so recipes describe structure, not raw config text. A JSON Schema for recipes is published with the core and used by `check`.

Templates receive the agent's entry from the stack file and the resolved MCP servers and skills. They never receive secret values: a secret appears as a reference (the path of its file under `/var/lib/olympus/secrets/<tool>/`, or an environment variable name), so a tool's hash does not change when a secret does ([ADR 0014](../ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)). Go's `text/template` is the likely template language now that the core is Go; see the [design doc](../architecture/design.md#open-questions).

## Checklist for a new recipe

- Default image tag set and its digest resolvable
- Every port bound to localhost
- Secrets declared, none hard-coded
- Data volumes named, so they survive upgrades
- Health check defined
- Tested on a clean server with a stack file using only this tool
