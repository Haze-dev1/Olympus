# Chat channels

- **Status:** In progress: Telegram renders for Hermes with a required allow-list; not exercised against Telegram.
- **Milestone:** M4
- **Plan:** [plans/v1.md](../../plans/v1.md)

Telegram, then Discord, for harness recipes that declare them.

## Acceptance

- Hermes answers allowed Telegram users only.
- An empty allow-list fails validation.

## Evidence

- 2026-10-19 — Hermes channel facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): 20+ channels; Telegram is `TELEGRAM_BOT_TOKEN` + `TELEGRAM_ALLOWED_USERS`, Discord is `DISCORD_BOT_TOKEN` + `DISCORD_ALLOWED_USERS`/`DISCORD_ALLOWED_ROLES`, both as `.env` variables. The gateway is deny-by-default when the allow-list is empty, which matches the acceptance test.
- 2026-10-20 — `supports.channels.<name>.env` maps `token` and `allow` to environment variable names ([ADR 0039](../../ADR.md#0039-configure-hermes-channels-as-environment-variables)). The evaluator requires a non-empty `allow` list, maps the token variable to a declared recipe secret that the stack must provide with the same source variable, and joins the allow-list into `TELEGRAM_ALLOWED_USERS`. The old `channel rendering is outside the offline slice` error is gone for supported channels.
- 2026-10-20 — `testdata/golden-real/compose.yaml` has `TELEGRAM_ALLOWED_USERS: "@alice,@bob"` on the Hermes service and `env_file: /var/lib/olympus/secrets/hermes/environment` for the token. `TestRealStackEmptyTelegramAllowListFails` removes the list and asserts the build fails with `allow-list`. `TestRenderingFailuresAndWarnings` keeps the unsupported-channel and empty-list cases.
- Limits: the live acceptance ("Hermes answers allowed users only") needs a real bot token and a deploy. Compose delivers `TELEGRAM_BOT_TOKEN` as a process environment variable; whether Hermes reads it there or only from `/opt/data/.env` is unverified (recorded in `recipes/hermes/README.md`).
