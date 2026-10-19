# Glossary

Terms as this project uses them.

| Term | Meaning |
| --- | --- |
| **Stack** | The whole set of tools defined for one server. |
| **Stack file** | `stack.yaml`: the user's description of the stack. Committed to git. |
| **Lock file** | `stack.lock`: the recipe input's git revision and hash, and each tool's image digest. Committed next to the stack file. |
| **Input** | A pinned source the stack depends on, like a flake input. In v1 the only input is `recipes`. |
| **Evaluation** | The local, pure step that turns the stack file, lock and recipes into a generation. Reads no secret values. |
| **Generation** | One complete rendered system: a Compose file, each tool's config files and a manifest. Numbered on the server; `current` points at the active one. |
| **Store** | `/var/lib/olympus/store/` on the server: rendered tool outputs, each in a folder named by its content hash. |
| **Activation** | Making a generation current on the server: write secrets, `docker compose up`, health checks, then confirmation. Run on the server by the activator. |
| **Activator** | The Linux build of the Olympus binary, uploaded to `/var/lib/olympus/bin/`, that runs activation and rollback on the server. |
| **Confirmation** | The CLI reconnecting over a new SSH connection to tell the activator the new generation is reachable. Without it, the server rolls back ("magic rollback", from deploy-rs). |
| **Rollback** | Re-activating the previous generation. |
| **Recipe** | `recipes/<tool>/recipe.yaml` plus templates: how to install, run and configure one tool. |
| **Core** | The Olympus program: reads the stack file and recipes, plans, applies over SSH, writes the lock file. |
| **Agent** | An entry under `agents:` in the stack file: a tool that acts on its own, such as Hermes or Codex. |
| **App** | An entry under `apps:`: a supporting tool that is not an agent, such as a workspace. |
| **Harness** | An agent runtime that talks to users through chat channels, such as Hermes or OpenClaw. |
| **Coding agent** | An agent that edits code in a workspace, such as Claude Code, Codex, OpenCode or pi. |
| **Channel** | A chat integration for a harness, such as Telegram or Discord. |
| **MCP server** | A Model Context Protocol server that gives agents tools. Defined once under `mcps:`. |
| **Skill** | A folder with a `SKILL.md` file that teaches an agent a task. Listed under `skills:`. |
| **Supports block** | The part of a recipe that says where its tool expects MCP servers, skills and channels, and in what format. |
| **Plan** | The diff between a newly built generation and the server's current one, shown without changing the server. |
| **Drift** | A file on the server that no longer matches its store hash because someone changed it by hand. |
