# Feature: Multi-Agent Team

This example demonstrates deploying a full multi-agent AI team on a single server using Olympus.

The team coordinates across chat and coding roles while sharing common tool definitions (MCP servers) and shared development guidelines (skills).

## Team Architecture

```
                    +-----------------------+
                    |  User (Telegram Chat) |
                    +-----------+-----------+
                                |
                                v
                    +-----------------------+
                    |  Hermes (Coordinator) |
                    +-----------+-----------+
                                |
        +-----------------------+-----------------------+
        |                       |                       |
        v                       v                       v
+---------------+       +---------------+       +---------------+
| Codex (OpenAI)|       |  Claude Code  |       | OpenCode (Web)|
+---------------+       +---------------+       +---------------+
        |                       |                       |
        +-----------------------+-----------------------+
                                |
                +---------------+---------------+
                |                               |
                v                               v
    +-----------------------+       +-----------------------+
    |  Shared MCP Servers   |       |     Shared Skills     |
    |  (GitHub stdio + HTTP)|       |   (Team Guidelines)   |
    +-----------------------+       +-----------------------+
```

### Agent Roles

1. **Hermes Agent (Harness & Chat Coordinator):**
   - Interacts with human operators via Telegram.
   - Coordinates high-level tasks and queries.
2. **Codex (Coding Agent):**
   - Runs OpenAI Codex headless in the background.
   - Handles automated coding tasks and code generation.
3. **Claude Code (Coding Agent):**
   - Runs Anthropic's Claude Code.
   - Excels at complex refactoring and multi-file analysis.
4. **OpenCode (Web / Headless Agent):**
   - Serves an agent interface on port 4096.
   - Can be accessed via web browser over an SSH tunnel.

## Shared Infrastructure

- **Shared MCP Servers:**
  - A stdio GitHub MCP server (`npx @modelcontextprotocol/server-github`) gives every agent access to repositories and issue tracking.
  - An HTTP documentation MCP server (`https://mcp.example.test/mcp`) provides team documentation access.
  - Olympus automatically translates these definitions into each agent's native format (YAML for Hermes, TOML for Codex, JSON for Claude Code and OpenCode).
- **Shared Skills:**
  - The `./skills/team-guidelines` skill is mounted or copied into all four agents.
  - When you update the guidelines, every agent receives the update on the next deploy.

## Security and Networking

- **Loopback Binding (Rule A4):** No container ports are exposed publicly. OpenCode's port 4096 binds exclusively to `127.0.0.1` on the server.
- **SSH Tunneling:** To access OpenCode's web interface, create an SSH tunnel:
  ```sh
  ssh -L 4096:localhost:4096 root@203.0.113.10
  ```
  Then visit `http://localhost:4096` in your browser.
- **Allow-Lists (Rule A10):** Hermes requires an authorized numeric Telegram user ID to prevent unauthorized access.
- **Secret Isolation (Rule A5):** Each agent container receives only its declared secrets in a private 0600 file under `/var/lib/olympus/secrets/<agent>/environment`.

## What You Need

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Set the required environment variables:
- `ANTHROPIC_API_KEY`: Model key for Hermes, Claude Code, and OpenCode.
- `CODEX_API_KEY`: Model key for Codex.
- `TELEGRAM_BOT_TOKEN`: Telegram bot token from `@BotFather`.
- `GH_TOKEN`: GitHub personal access token with repo scope.

## Commands

Validate the stack offline:

```sh
olym check -f stack.yaml
```

Build the local generation:

```sh
olym build -f stack.yaml -o /tmp/build-05
```

Preview server changes:

```sh
olym plan -f stack.yaml
```

Deploy the entire team to your server:

```sh
olym apply -f stack.yaml
```

Check runtime status:

```sh
olym status -f stack.yaml
```

## What You Should See

- `olym check` confirms:
  ```
  Stack is valid.
  ```
- `olym apply` starts all four agent containers and verifies health checks.
- `olym status` displays:
  - `hermes`: running, healthy.
  - `codex`: running, healthy.
  - `claude`: running, healthy.
  - `opencode`: running, healthy.
- Telegram messages from the allowed numeric ID interact directly with Hermes.
