# Chat channels

- **Status:** In progress: Telegram and Discord render for Hermes with a required allow-list; live interaction pending real bot tokens.
- **Milestone:** M4
- **Plan:** [plans/v1.md](../../plans/v1.md)

Telegram, then Discord, for harness recipes that declare them.

## Acceptance

- Hermes answers allowed Telegram users only.
- Hermes answers allowed Discord users only (or members with allowed roles).
- An empty allow-list fails validation.

## Evidence

- 2026-11-06 — OLY-60 / F1 in progress: recipe validation and evaluation reject a missing allow mapping. Focused recipe and evaluator tests pass, including the unchanged real-stack golden and Hermes mappings. The red run reproduced acceptance of a missing mapping in both layers. Live allowed/unlisted sender verification was not run: no disposable server or QEMU is available. Build, vet, and gofmt passed; full tests failed in the known macOS deploy tests `TestUploadGeneration`, `TestUploadActivatorVersionAndPublication`, and `TestToolEntryReuseAndVerification`. Fix verification remains partial pending the live check; no ADR is warranted because this enforces ADR 0039 and A10 rather than changing their contract.

- 2026-10-19 — Hermes channel facts verified in [Sources/recipe-research.md](../../Sources/recipe-research.md): 20+ channels; Telegram is `TELEGRAM_BOT_TOKEN` + `TELEGRAM_ALLOWED_USERS`, Discord is `DISCORD_BOT_TOKEN` + `DISCORD_ALLOWED_USERS`/`DISCORD_ALLOWED_ROLES`, both as `.env` variables. The gateway is deny-by-default when the allow-list is empty, which matches the acceptance test.
- 2026-10-20 — `supports.channels.<name>.env` maps `token` and `allow` to environment variable names ([ADR 0039](../../ADR.md#0039-configure-hermes-channels-as-environment-variables)). The evaluator requires a non-empty `allow` list, maps the token variable to a declared recipe secret that the stack must provide with the same source variable, and joins the allow-list into `TELEGRAM_ALLOWED_USERS`. The old `channel rendering is outside the offline slice` error is gone for supported channels.
- 2026-10-20 — `testdata/golden-real/compose.yaml` has `TELEGRAM_ALLOWED_USERS: "@alice,@bob"` on the Hermes service and `env_file: /var/lib/olympus/secrets/hermes/environment` for the token. `TestRealStackEmptyTelegramAllowListFails` removes the list and asserts the build fails with `allow-list`. `TestRenderingFailuresAndWarnings` keeps the unsupported-channel and empty-list cases.
- 2026-11-05 — OLY-17: Discord channel added to Hermes recipe (`supports.channels.discord.env` with `token: DISCORD_BOT_TOKEN`, `allow: DISCORD_ALLOWED_USERS`, `roles: DISCORD_ALLOWED_ROLES`) and declared secret `discord-token: {env: DISCORD_BOT_TOKEN}`. Stack schema and evaluator updated to support optional `roles` on channels; evaluator maps `allow` to `DISCORD_ALLOWED_USERS` and `roles` to `DISCORD_ALLOWED_ROLES`. Empty allow-list fails validation (`TestRealStackEmptyDiscordAllowListFails`, Rule A10). Token delivers strictly via `env_file` (`/var/lib/olympus/secrets/hermes/environment`, Rules A1, A5) and is verified absent from `compose.yaml` and `manifest.json`.
- Limits: live acceptance ("Hermes answers allowed Telegram users only" and "Hermes answers allowed Discord users only") needs real bot tokens and deploy. Compose delivers `TELEGRAM_BOT_TOKEN` and `DISCORD_BOT_TOKEN` as process environment variables via secret `env_file`; live platform testing remains pending.
