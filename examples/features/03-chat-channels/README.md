# Feature: Chat Channels (Telegram and Discord with Allow-Lists)

This example shows how to configure chat channels for Hermes Agent to interact with your agents via Telegram and Discord.

## Why an Empty Allow-List is Rejected

In Olympus, Security Rule A10 mandates:
> **A10.** Chat channels require a non-empty allow-list.

If you omit the `allow` list or leave it empty (`allow: []`), Olympus rejects the stack during evaluation:

```
hermes: channel requires a non-empty allow-list
```

This fail-closed design prevents accidentally deploying an open bot gateway. Without an allow-list, any user on Telegram or Discord could message your bot, consume model API credits, and trigger container actions.

## ID Format Requirements

Chat providers use specific ID formats. Olympus requires exact identifier types:

- **Telegram User IDs (Numeric only):**
  - Must be numeric user IDs, such as `"123456789"`.
  - Do **not** use profile handles like `"@alice"`. Upstream Hermes matches `TELEGRAM_ALLOWED_USERS` directly against the numeric `user_id` in Telegram incoming message payloads. A handle like `@alice` never matches and will fail closed (denying access).
  - To find your numeric Telegram ID, message a bot like `@userinfobot` or check `https://api.telegram.org/bot<TOKEN>/getUpdates`.
- **Discord User and Role IDs (Numeric only):**
  - `allow`: Non-empty list of numeric Discord user IDs (e.g. `"112233445566778899"`).
  - `roles`: Optional list of numeric Discord role IDs (e.g. `"998877665544332211"`).
  - To copy IDs in Discord, enable Developer Mode (Settings > Advanced > Developer Mode), then right-click a user or role and select "Copy User ID" or "Copy Role ID".

## Secret and Allow-List Delivery

Olympus separates secret tokens from operational configuration:
- Bot tokens (`TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`) are delivered securely as 0600 files under `/var/lib/olympus/secrets/hermes/environment` (Rule A1, Rule A5). They are never exposed in Compose manifests or CLI logs.
- Allow-lists and role lists are joined with commas and passed to container environment variables (`TELEGRAM_ALLOWED_USERS`, `DISCORD_ALLOWED_USERS`, `DISCORD_ALLOWED_ROLES`).

## What You Need

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Set the required environment variables:
- `ANTHROPIC_API_KEY`: Anthropic API key for Hermes.
- `TELEGRAM_BOT_TOKEN`: Bot token from Telegram's `@BotFather`.
- `DISCORD_BOT_TOKEN`: Bot token from the Discord Developer Portal.

## Commands

Validate the stack offline:

```sh
olym check -f stack.yaml
```

Build the local generation:

```sh
olym build -f stack.yaml -o /tmp/build-03
```

Preview server changes:

```sh
olym plan -f stack.yaml
```

Deploy to your server:

```sh
olym apply -f stack.yaml
```

Check runtime status:

```sh
olym status -f stack.yaml
```

## What You Should See

- `olym check` confirms the stack is valid and passes channel validation:
  ```
  Stack is valid.
  ```
- `olym status` shows `hermes` running and healthy.
- Sending a message to the Telegram bot from an allowed numeric user ID receives a response.
- Messages from unlisted accounts receive no response.
