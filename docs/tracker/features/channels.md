# Chat channels

- **Status:** Not started
- **Milestone:** M4
- **Plan:** [plans/v1.md](../../plans/v1.md)

Telegram, then Discord, for harness recipes that declare them.

## Acceptance

- Hermes answers allowed Telegram users only.
- An empty allow-list fails validation.

## Evidence

- 2026-10-19 — Hermes channel facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): 20+ channels; Telegram is `TELEGRAM_BOT_TOKEN` + `TELEGRAM_ALLOWED_USERS`, Discord is `DISCORD_BOT_TOKEN` + `DISCORD_ALLOWED_USERS`/`DISCORD_ALLOWED_ROLES`, both as `.env` variables. The gateway is deny-by-default when the allow-list is empty, which matches the acceptance test.
- Consequence: Hermes channels are environment-based, not a `config.yaml` key, so the recipe's `supports.channels` shape needs an env variant before this feature can be written.
- Codex and OpenCode have no chat channels, so channel support is a harness-recipe concern only.
