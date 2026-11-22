# User documentation

- **Status:** Done
- **Updated:** 2026-11-22
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

## Merge review, 2026-11-06

- `docs/usage/commands.md` listed only the flags in the CLI's invalid-flag message. Added the flags `internal/cli/cli.go` also accepts (`-activator`, `-out`, `-yes`, `-keep`, `-json`, `-delete-data`) to the overview and per-command sections, and described the saved-plan staleness check.
- `examples/full-stack/stack.yaml` passed `olym check -recipes recipes` with fake sentinel values in a temporary `.env` (deleted). Whether Hermes accepts `@username` or needs numeric Telegram IDs in `allow` was not verified then; it matched the stack spec's own example and was left to the OLY-16 live test. OLY-16 source review on 2026-11-07 resolved it: Telegram matches numeric user IDs only, so the example and the stack spec now use numeric IDs.

## Evidence, 2026-11-22

Added `examples/` (getting-started by opencode, features by agy, reviewed by Claude) and an offline validation test for every example stack.

1. **Examples added:**
   - `examples/README.md`: setup, run commands, and an index table covering getting-started, planned `examples/features/*` examples, and `examples/full-stack/`.
   - `examples/getting-started/01-first-agent/`: smallest Hermes stack with `.env.example`.
   - `examples/getting-started/02-coding-agent/`: Codex stack with `.env.example`.
   - `examples/getting-started/03-two-agents/`: Hermes and OpenCode side by side with `.env.example`.
   - `examples/getting-started/04-every-tool/`: one entry per recipe in `recipes/`, no secrets required.
   - `examples/getting-started/05-apps/`: OpenClaw in `apps:` with server options and `.env.example`.
   - `examples/full-stack/.env.example`: fake secret values for the existing full-stack example.

2. **Validation test:**
   - Added `internal/cli/examples_test.go` (`TestExamples`). It walks `examples/**/stack.yaml`, copies each example to a temp dir, writes `.env` from `.env.example` or synthesizes fake values from `${NAME}` refs, then runs `olym check` and `olym build` with `-recipes ./recipes`.
   - The test picks up `examples/features/*` automatically: 12 subtests pass (full-stack, 5 getting-started, 6 features).

   **Review fixes:** restored 2-space indent in `progress.json` (the worker had reindented the whole file); matched expected `plan` output to the real CLI (`+ <tool>  new tool`); `06-catalog-pin` used a nonexistent `?ref=v1.0.0` and now uses `?ref=development`; the rollback walkthrough wrongly said `current` moves before confirmation and cited a `--no-confirm` flag the CLI does not have, both corrected. Found live: `check -recipes` writes no `stack.lock` and `apply` then fails with `apply requires digest-pinned images`, so run commands no longer pass `-recipes`; the READMEs say `-recipes ./recipes` is for offline validation only.

   **Catalog gap:** the default branch (`main`) ships only Hermes, Codex and OpenCode, so examples using other recipes need `?ref=development` until `development` is promoted. `examples/README.md` says so.

3. **Verification:**
   - `go run ./cmd/olym check -f examples/getting-started/01-first-agent/stack.yaml -recipes ./recipes` with `.env.example` copied to `.env`: `Stack is valid.`; build produced `sha256-1982ce76ab69664110ac589f5093412570071a64dd4074f785f7d89c96c4bb58`. `.env` removed.
   - `go run ./cmd/olym check -f examples/getting-started/02-coding-agent/stack.yaml -recipes ./recipes`: `Stack is valid.`; build `sha256-8a34a08a2d24d02a42c3d212fb107a67b010dc74afa89e8e7cf8174b162c2923`. `.env` removed.
   - `go run ./cmd/olym check -f examples/getting-started/03-two-agents/stack.yaml -recipes ./recipes`: `Stack is valid.`; build `sha256-609eaf53ced032ef89111826b25b04fca50e6adc03fe2d0a0de929ba7af6cfa5`. `.env` removed.
   - `go run ./cmd/olym check -f examples/getting-started/04-every-tool/stack.yaml -recipes ./recipes`: `Stack is valid.`; build `sha256-3a94538cd91b00987c3a594b308f98df5644255e1f639170f0266a2fe03b8b7b`.
   - `go run ./cmd/olym check -f examples/getting-started/05-apps/stack.yaml -recipes ./recipes`: `Stack is valid.`; build `sha256-ff44aa9a96c0bbf612ea016d9b89a80ce16cf72a528cacd25e9a0daa2da43514`. `.env` removed.
   - `go run ./cmd/olym check -f examples/full-stack/stack.yaml -recipes ./recipes`: `Stack is valid.`; build `sha256-1f4dc78f1cfbcc5a411031afcc3c0d71d08cb4cfe04cc4c1775ec20f684ddfb7`. `.env` removed.
   - `go test ./internal/cli/... -run TestExamples -v`: PASS (6 subtests: full-stack and 5 getting-started).
   - `go test ./...`: PASS.
   - `gofmt -l .`: empty.
   - `golangci-lint run ./...`: 0 issues.

   **Live (lab VM, Ubuntu 24.04, sentinel `.env.example` values, catalog `?ref=development` resolved to `7c27562`):**
   - `getting-started/01-first-agent`: check wrote `stack.lock` (tools `hermes`); plan, apply, status (`hermes running healthy`), re-apply `No changes.`, destroy all exit 0.
   - `features/05-multi-agent-team`: check wrote `stack.lock` (claude, codex, hermes, opencode); plan, apply exit 0; status shows all four `running healthy`; re-apply `No changes.`; destroy exit 0, volumes kept. No sentinel value in any output.
   - Not run live: the other ten examples (offline check and build only), real chat or model calls.

4. **ADR:** No ADR change warranted. This adds documentation, examples, and a validation test; it does not change architecture, file formats, durable behavior, or an important tradeoff.
