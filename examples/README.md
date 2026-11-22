# Examples

Small, copy-pasteable Olympus stacks. Each example shows one idea and can be run on its own.

## Setup

1. Install `olym`. See [docs/install.md](../docs/install.md).
2. Have a fresh Ubuntu or Debian server you can reach over SSH. You need the server's IP address or hostname and an SSH key.
3. Pick an example and copy its `.env.example` to `.env` next to the `stack.yaml`.
4. Replace every `replace-me` value with your real value.

```sh
cp examples/getting-started/01-first-agent/.env.example examples/getting-started/01-first-agent/.env
```

`.env` files hold secrets. Do not commit them.

## Run an example

From the repository root, check the stack file and recipes:

```sh
olym check -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Preview the changes on the server:

```sh
olym plan -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Apply the stack to the server:

```sh
olym apply -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes
```

Check what is running:

```sh
olym status -f examples/getting-started/01-first-agent/stack.yaml
```

`olym plan` and `olym apply` connect to the server over SSH. `olym check` runs offline.

`-recipes ./recipes` uses the recipes in your clone of this repository. Leave it out to use the published catalog named in `inputs.recipes` (the default is `github:Haze-dev1/Olympus/recipes`), which Olympus pins in `stack.lock`.

## Examples

### Getting started

| Example | What it shows |
| --- | --- |
| [01-first-agent](getting-started/01-first-agent/) | One Hermes agent, the smallest working stack |
| [02-coding-agent](getting-started/02-coding-agent/) | One Codex coding agent with its API key |
| [03-two-agents](getting-started/03-two-agents/) | Hermes and OpenCode running side by side |
| [04-every-tool](getting-started/04-every-tool/) | One entry for every recipe in the catalog |
| [05-apps](getting-started/05-apps/) | An app in `apps:` and common `server:` options |

### Features

| Example | What it shows |
| --- | --- |
| [01-mcp-servers](features/01-mcp-servers/) | Shared MCP servers connected to agents |
| [02-shared-skills](features/02-shared-skills/) | Skills shared across agents |
| [03-chat-channels](features/03-chat-channels/) | Chat channels with an allow-list |
| [04-update-and-rollback](features/04-update-and-rollback/) | Updating versions and rolling back |
| [05-multi-agent-team](features/05-multi-agent-team/) | A small team of agents with different roles |
| [06-catalog-pin](features/06-catalog-pin/) | Pinning a recipe catalog with `inputs` and `stack.lock` |

### Other

| Example | What it shows |
| --- | --- |
| [full-stack](full-stack/) | A fuller stack with Hermes, Codex, OpenCode, MCP, and a Telegram channel |
