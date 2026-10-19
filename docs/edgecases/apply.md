# Edge cases: plan and apply

Cases the core must handle in v1, and the intended behaviour. Each becomes a test once the core exists. Error wording is a proposal.

## Stack file and secrets

| Case | Intended behaviour |
| --- | --- |
| `${NAME}` has no value in `.env` or the environment | Fail before connecting; name the variable and every line that uses it |
| `.env` is missing | Fail if any `${...}` is used; say where `.env` was looked for |
| YAML type trap (`no`, `on`, `08` read as boolean or number) | Validation error naming the field and the expected type |
| Agent lists an MCP server not defined under `mcps:` | Fail; name the agent and the missing server |
| Recipe name not found in `recipes/` | Fail; suggest the closest recipe name |
| Recipe lacks support for MCP servers, skills or a channel the agent asks for | Warn in `plan`, continue without that feature |
| `stack.yaml` pins a version different from `stack.lock` | The stack file wins; `plan` shows the change and `apply` rewrites the lock |

## Server

| Case | Intended behaviour |
| --- | --- |
| SSH fails (wrong key, host down, key has a passphrase and no agent) | Fail with the SSH error and the host and user tried |
| OS is not Ubuntu or Debian | Fail before changing anything; list supported systems |
| SSH user is not root and has no sudo | Fail with the exact command that needed sudo |
| Docker already installed, with containers Olympus did not create | Leave them alone; manage only the `olympus` Compose project ([ADR 0015](../ADR.md#0015-use-one-docker-compose-project-as-the-runtime)) |
| Port already used by something else | Fail for that tool in `plan`; name the port and the process using it |
| Server has less RAM than a recipe's `min_ram` | Warn in `plan`; continue only if the user confirms |
| ARM server and an image with no ARM build | Fail for that tool; name the image and architecture |
| Disk full during image pull | Fail for that tool; report free space |

## During apply

| Case | Intended behaviour |
| --- | --- |
| One tool fails to start, or its health check never passes | Re-activate the previous generation; report the failed tool and its last log lines; exit non-zero |
| Connection drops during upload | Nothing activated; `current` unchanged; re-running `apply` re-uploads only missing store entries |
| Connection drops after activation starts | The activator keeps running; with no confirmation by `confirm_timeout` it re-activates the previous generation and releases the lock ([ADR 0017](../ADR.md#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms)) |
| The new generation breaks SSH access (for example a port or firewall change) | The CLI cannot reconnect to confirm, so the server rolls back by itself |
| Confirmation arrives after the timeout | `confirm` fails and says the server already rolled back; `apply` exits non-zero |
| `apply --no-confirm` | Activates without waiting; no automatic rollback for lost access; warn once |
| Activator binary on the server is a different version | Upload the matching build before activating; never run a mismatched activator |
| Activator binary fails its checksum | Refuse to run it; re-upload |
| Another `apply`, `rollback` or `gc` is running | Fail with who holds the lock and since when |
| Someone edited a tool's config file on the server | `plan` reports drift (store file no longer matches its hash); `apply` writes a fresh store entry |
| First `apply` on a server (no generations yet) | A failure leaves no current generation; containers that started are removed |
| Tool needs an interactive login (Claude Code, Codex) | Recipe takes a token secret; if none is given, `plan` warns that the tool will start logged out |
| Telegram token valid but `allow` is empty | Fail; an open bot lets anyone run the agent |

## Generations

| Case | Intended behaviour |
| --- | --- |
| `rollback` with no previous generation | Fail; say there is nothing to roll back to |
| `rollback` to a generation whose store entries `gc` deleted | Fail; name the missing entries; suggest re-applying from that generation's `stack.lock` in git |
| `gc` | Never deletes entries used by `current` or the previous kept generations; reports space freed |
| Two runs render the same stack | Byte-identical generation and hashes (test for purity) |
| Only a secret value changed in `.env` | Hash unchanged; `plan` reports a secret change for that tool and `apply` restarts only it |

## Destroy

| Case | Intended behaviour |
| --- | --- |
| Containers removed by hand already | Skip them; remove what is left |
| Data volumes exist | Keep them unless `--delete-data` is passed; list what was kept |
