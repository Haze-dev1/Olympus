# User documentation

- **Status:** Done
- **Updated:** 2026-11-06
- **Milestone:** M1

Adds user-facing documentation for Olympus (OLY-35): a Quickstart in `README.md`, command-by-command CLI reference in `docs/usage/commands.md`, a field-by-field `stack.yaml` configuration reference in `docs/usage/stack-reference.md`, and a complete reference example stack in `examples/full-stack/stack.yaml`.

## Evidence, 2026-11-06

1. **Quickstart in README.md:**
   - Copied binary installation and checksum verification steps from `docs/install.md` verbatim.
   - Provided minimal `stack.yaml` and `.env` setup instructions.
   - Documented the primary deployment workflow: `olym check` &rarr; `olym plan` &rarr; `olym apply`.
   - Updated documentation navigation links in `README.md` and `docs/README.md`.

2. **Command Reference (`docs/usage/commands.md`):**
   - Built CLI binary locally: `/opt/homebrew/bin/go build -o /tmp/olym ./cmd/olym/`.
   - Ran `/tmp/olym help` to capture root usage:
     `usage: olym {check|build|update|plan|apply|status|rollback|gc|destroy} [-f stack.yaml] [-recipes recipes] [-o .olympus/build]`
   - Ran `/tmp/olym <command> -h` across all commands to capture error flag text:
     `invalid flags; use -f, -recipes, -o, -known-hosts and --detailed-exitcode`
   - Documented all 9 CLI commands with exact flags displayed by the CLI (`-f`, `-recipes`, `-o`, `-known-hosts`, `--detailed-exitcode`).
   - Documented `olym version`.

3. **Stack File Reference (`docs/usage/stack-reference.md`):**
   - Documented every top-level key and nested field strictly from `schemas/stack.schema.json` and `docs/specs/stack-file.md`.
   - Referenced types, requiredness, allowed values, regex patterns, and default values.
   - Linked to specification sections instead of duplicating lengthy specifications.

4. **Full-Stack Example (`examples/full-stack/stack.yaml`):**
   - Configured Hermes (with `model-key`, `telegram-token`, `github-token`), Codex (with `api-key`, `github-token`), OpenCode (with `model-key`, `github-token`).
   - Configured shared GitHub MCP server (`npx @modelcontextprotocol/server-github`) with `${GH_TOKEN}`.
   - Configured Telegram channel on Hermes with non-empty allow-list `["@admin_user"]` and `${TELEGRAM_BOT_TOKEN}`.
   - Verified that all secrets use `${VARIABLE}` syntax and no secret values are hardcoded.
   - Verified offline validation using a temporary `.env` with sentinel values:
     ```sh
     cat << 'EOF' > examples/full-stack/.env
     GH_TOKEN=OLYM_SENTINEL_1
     HERMES_MODEL_KEY=OLYM_SENTINEL_2
     TELEGRAM_BOT_TOKEN=OLYM_SENTINEL_3
     CODEX_API_KEY=OLYM_SENTINEL_4
     OPENCODE_MODEL_KEY=OLYM_SENTINEL_5
     EOF
     /tmp/olym check -f examples/full-stack/stack.yaml -recipes recipes
     # Output:
     # Local -recipes override: skipping the lock's catalog pin.
     # Stack is valid.
     rm -f examples/full-stack/.env
     ```
   - Confirmed `.env` was deleted and untracked.

5. **Disagreements Noted:**
   - The CLI parser in `internal/cli/cli.go` accepts additional flags (`-activator`, `-out`, `-yes`, `-keep`, `-delete-data`, `-json`) and accepts the `version` command, but its help string (`olym help`) and flag validation string (`olym <command> -h`) exclusively print `-f`, `-recipes`, `-o`, `-known-hosts`, and `--detailed-exitcode`. Per task instructions, the CLI printed output wins in `docs/usage/commands.md` and Go code is left untouched.
