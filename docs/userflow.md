# User flows

How a user works with Olympus, from first setup to teardown.

> **Draft.** No code exists yet. The command is `olym`. File formats follow [specs/stack-file.md](specs/stack-file.md) and [specs/recipe.md](specs/recipe.md).

## 1. First setup on a new server

Starting state: an Ubuntu or Debian VPS you can reach with an SSH key, and the tokens for the tools you want.

1. Create a repo with a `stack.yaml` listing the server, MCP servers, skills, agents and apps.
2. Create a `.env` next to it with every secret the file references as `${NAME}`. `.env` is git-ignored.
3. Run `olym check` to validate the stack file and recipes offline. On first run this pins the recipe input and image digests in a new `stack.lock`.
4. Optionally run `olym build` to render the generation locally and read the Compose file and configs it will deploy.
5. Run `olym plan`. It connects over SSH and lists what it will create. Nothing changes on the server.
6. Run `olym apply`. It installs Docker and Compose if missing and uploads the generation. The activator on the server writes secret files, runs `docker compose up` and health checks, then waits for the CLI to reconnect and confirm. If anything fails, or the confirmation never arrives, the server returns to the previous generation by itself.
7. Run `olym status` to see each tool's container state and health check.
8. Reach web UIs through an SSH tunnel, for example `ssh -L 9119:localhost:9119 user@host`. No tool is reachable publicly ([ADR 0010](ADR.md#0010-expose-nothing-publicly-by-default)).
9. Message the Telegram bot from an allowed account; other accounts get no answer.
10. Commit `stack.yaml` and `stack.lock`.

## 2. Add a tool or an MCP server

1. Add the agent under `agents:`, or the MCP server under `mcps:` and its name to each agent that should get it.
2. Add any new secrets to `.env`.
3. Run `plan`: only the new container and the changed config files are listed.
4. Run `apply`. It creates a new generation; only tools whose hash changed are restarted.
5. Commit the updated `stack.yaml` and `stack.lock`.

## 3. Upgrade versions

1. Run `olym update`. It moves the recipe input to the latest revision and re-resolves image digests. This rewrites `stack.lock` only.
2. Review the `stack.lock` diff in git.
3. Run `plan`, then `apply`. Only tools whose hash changed are restarted.
4. If something breaks, run `olym rollback` to return to the previous generation, then revert `stack.lock` in git so the next `apply` does not re-upgrade.
5. Run `olym gc` now and then to delete old generations' files from the server.

## 4. Rebuild on another server

1. Change `server.host` (or keep a second stack file per server).
2. Copy `.env` to the machine running the command.
3. Run `apply`. The locked digests give the same versions as the original server.
4. Agent memory, chat history and coding-agent logins are **not** carried over; they live on the old server. Backups are a later feature.

## 5. Remove a tool or tear everything down

- **Remove one tool:** delete it from `stack.yaml`, run `plan` to confirm only that tool goes, then `apply`. Its data volume is kept.
- **Tear down:** run `olym destroy` and confirm. All containers it created are removed. Data volumes are kept unless `--delete-data` is passed.

## 6. Add support for a new tool (recipe author)

1. Create `recipes/<tool>/recipe.yaml` following [specs/recipe.md](specs/recipe.md), plus any config templates.
2. Declare in `supports` where the tool reads MCP servers, skills and channels, and in which format.
3. Test it on a throwaway server with a minimal `stack.yaml` that uses only this tool.
4. Open a pull request. No core change should be needed; if one is, the recipe format is missing something, and that needs an ADR.
