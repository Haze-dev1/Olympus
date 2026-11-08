# stack.yaml Reference

This document provides a field-by-field reference for the Olympus stack file (`stack.yaml`).

The schema definition is maintained in [schemas/stack.schema.json](../../schemas/stack.schema.json). For architectural details, lock file behaviour, and generation layout, see the [stack file specification](../specs/stack-file.md).

---

## Top-Level Structure

A `stack.yaml` file defines the infrastructure, MCP servers, skills, agents, and applications to run on a target host.

```yaml
inputs:
  recipes: github:Haze-dev1/Olympus/recipes

server:
  host: 1.2.3.4
  user: root

mcps:
  github:
    command: npx
    args: ["@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: ${GH_TOKEN}

skills:
  - ./skills/deploy-checklist

agents:
  hermes:
    recipe: hermes
    mcps: [github]
    skills: all
    secrets:
      model-key: ${ANTHROPIC_KEY}
    channels:
      telegram:
        token: ${TG_TOKEN}
        allow: ["123456789"]

apps:
  openbot:
    recipe: openbot
```

| Key | Required | Type | Description |
| --- | --- | --- | --- |
| [`inputs`](#inputs) | No | Object | Recipe catalog source definitions. |
| [`server`](#server) | **Yes** | Object | Target server connection parameters. |
| [`mcps`](#mcps) | No | Map of String &rarr; Object | Shared MCP server configurations. |
| [`skills`](#skills) | No | Array of Strings | Paths to skill folders relative to `stack.yaml`. |
| [`agents`](#agents-and-apps) | No* | Map of String &rarr; Object | AI agents and harnesses to deploy. |
| [`apps`](#agents-and-apps) | No* | Map of String &rarr; Object | Non-agent tools and workspaces to deploy. |

*\*At least one of `agents` or `apps` must be defined.*

---

## `inputs`

Configures the source repository for tool recipes.

| Field | Required | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `recipes` | No | String (`minLength: 1`) | `github:Haze-dev1/Olympus/recipes` | Recipe catalog source URI in format `github:<owner>/<repo>[/<path>][?ref=<ref>]`. See [inputs specification](../specs/stack-file.md#inputs). |

---

## `server`

Defines SSH connection and deployment parameters for the target machine.

| Field | Required | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `host` | **Yes** | String (`minLength: 1`) | *(none)* | IP address or hostname of the target server. |
| `user` | No | String (`minLength: 1`) | `root` | SSH user account (requires sudo access if non-root). |
| `port` | No | Integer (`1`&ndash;`65535`) | `22` | Target SSH port. |
| `ssh_key` | No | String (`minLength: 1`) | SSH agent | Path to private SSH key file. |
| `keep` | No | Integer (`minimum: 1`) | `5` | Number of previous generations retained during garbage collection. |
| `confirm_timeout` | No | String (`minLength: 1`) | `60s` | Rollback deadline duration string (e.g. `60s`, `5m`) waiting for CLI confirmation. See [server specification](../specs/stack-file.md#server). |

---

## `mcps`

A dictionary mapping MCP server names to their execution specifications. An MCP server must be either a local process (stdio) or a remote service (URL), never both.

### Stdio MCP Server

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `command` | **Yes** (stdio) | String (`minLength: 1`) | Executable program command to start. |
| `args` | No | Array of unique Strings (`minLength: 1`) | CLI arguments passed to `command`. |
| `env` | No | Map of String &rarr; String | Environment variables for the MCP process. Values may reference secrets as `${NAME}`. |

### Remote MCP Server

| Field | Required | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `url` | **Yes** (remote) | String (`minLength: 1`) | *(none)* | HTTP/HTTPS endpoint URL of the remote MCP server. |
| `transport` | No | String (`enum: ["http", "sse"]`) | `http` | Transport protocol (`http` for streamable HTTP, `sse` for Server-Sent Events). Only valid with `url`. |
| `headers` | No | Map of String &rarr; String | *(none)* | HTTP headers sent with requests. Values may reference secrets as `${NAME}`. |

See [mcps specification](../specs/stack-file.md#mcpsname) for per-tool translation details.

---

## `skills`

A list of relative filesystem paths to local skill folders.

- **Type:** Array of unique Strings (`minLength: 1`).
- **Required:** No.
- **Constraints:** Each specified directory must exist and contain a valid `SKILL.md`.

---

## `agents` and `apps`

`agents` and `apps` share the identical configuration schema. `agents` is used for conversational agents and coding assistants; `apps` is used for non-agent services and workspaces.

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `recipe` | **Yes** | String (pattern: `^[a-z][a-z0-9-]{0,30}$`) | Recipe name resolving to a recipe in the recipe catalog. |
| `image_tag` | No | String (`minLength: 1`) | Overrides the default container image tag from the recipe; image digest remains pinned in `stack.lock`. |
| `mcps` | No | Array of unique Strings (`minLength: 1`) | Names of MCP servers from top-level `mcps` exposed to this tool. |
| `skills` | No | String (`"all"`) or Array of unique Strings (`minLength: 1`) | Skills exposed to this tool: `"all"` to include all top-level skills, or a list of specific paths matching top-level `skills`. |
| `secrets` | No | Map of String &rarr; String | Secret values mapping recipe-declared secret names to single `${NAME}` references. |
| `channels` | No | Map of String &rarr; Channel Object | Channel configuration for tools supporting chat integrations. |
| `ports.public` | No | Array of unique Integers (`1`&ndash;`65535`) | Ports to bind publicly to `0.0.0.0`. By default, all container ports bind to `127.0.0.1`. |

### Channel Object

For tools with channel support (e.g. `telegram`, `discord`):

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `token` | **Yes** | String (`minLength: 1`) | Bot authentication token reference as `${NAME}`. |
| `allow` | **Yes** | Array of unique Strings (`minItems: 1`) | Non-empty allow-list of authorized user identifiers, in the format the bot expects. Telegram matches numeric user IDs only; `@username` entries match nobody and the bot answers no one. Discord accepts numeric user IDs (usernames are resolved at connect time). |
| `roles` | No | Array of unique Strings (`minLength: 1`) | Optional list of authorized role IDs (e.g. Discord roles). |

---

## Secret References

All secret values in `stack.yaml` must be single environment variable references using `${VARIABLE_NAME}` syntax. Real secret values must never be committed to `stack.yaml`; Olympus loads variable values from a local, git-ignored `.env` file adjacent to `stack.yaml` or from the process environment. See [variables specification](../specs/stack-file.md#variables).
