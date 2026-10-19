---
name: olympus-test
description: Run an Olympus QA pass when asked to test or verify Olympus. Covers static and unit checks, recipe validation, the edge-case matrix, a live plan/apply/destroy cycle on a disposable server, and exposure and secret-leak checks, ending in an evidence report. Honors a targeted or full scope and reports defects without fixing product code.
---

# Olympus test pass

One run proves a change (or the whole tool) on six fronts and ends in one report. Every front ends **green**, **red**, or **not run (reason)**. A pass applies only to what was exercised in this session; old evidence is reported separately.

> As of 2026-10-19 the Go module exists with `internal/catalog` and `internal/lock`; front 1 runs the build, vet, test and format commands in `AGENTS.md` under "Validation before you commit". The `olym` CLI does not exist yet, so front 4 is "not run (no CLI)" until slice 1 lands.

## Guardrails

- **Disposable servers only.** Live fronts run against a server created for this run and destroyed after it. Never point `apply` or `destroy` at a server the user did not name as disposable for this run.
- **Sentinel secrets.** The test `.env` uses made-up values with a fixed prefix, such as `OLYM_SENTINEL_<NAME>_<random>`, never real keys. That makes leak checks safe: search output for the prefix. Real provider calls need a target and call budget the user approves for this run.
- **Never read the user's real `.env`.** Build the test `.env` in a temporary directory.
- **Report, don't fix.** Findings go in the report. Change product code only when the user asks, after the report.
- **Stop what you started, by PID or container name.** `pkill -f` matches your own shell.

## 0. Scope

Pick the mode: **changed** (default; files changed since the last reviewed commit, plus untracked files) or **full** (the user asks for the whole tool, or a release). Map each changed file to a feature in `docs/tracker/features/` and to the flows in `docs/userflow.md`. Read `docs/PRD.md`, `docs/ADR.md`, the relevant `docs/specs/` and `docs/edgecases/` before choosing expectations.

Honor explicitly targeted checks; do not expand them into a full pass. Done when every scoped file maps to a feature or is labelled docs-only.

## 1. Static and unit checks

Run the applicable commands under "Validation before you commit" in `AGENTS.md`. Done when each command has an exit code and its pass, skip and fail counts recorded.

## 2. Recipe validation

For every changed recipe (all recipes in full mode):

- It parses against [docs/specs/recipe.md](../../../docs/specs/recipe.md).
- It passes the spec's "Checklist for a new recipe".
- Every path, port and image in it is checked against the tool's upstream docs, with the source linked in the report.

## 3. Edge-case matrix

For each changed behaviour, take every row in the matching `docs/edgecases/*.md` file and record where it is proven: an existing test, a live check in front 4 or 5, or **GAP**. Add new cases found during the run to the edge-case file and to the report.

Done when no row is blank.

## 4. Live cycle on a disposable server

With a stack file covering the changed recipes and the test `.env`:

1. `plan` lists the expected creations and nothing else.
2. `apply` succeeds; `status` shows every tool running and healthy.
3. `apply` again reports no changes, and two `build` runs give byte-identical generations ([ADR 0014](../../../docs/ADR.md#0014-evaluate-the-stack-locally-into-a-generation-then-activate-it-on-the-server)).
4. A one-line change to the stack file makes `plan` list only that change.
5. After an upgrade (`update`, then `apply`), `rollback` restores the previous images.
6. A recipe with a failing health check leaves the previous generation current.
7. Kill the CLI after activation starts: within `confirm_timeout` the server is back on the previous generation ([ADR 0017](../../../docs/ADR.md#0017-activate-on-the-server-and-roll-back-unless-the-cli-confirms)).
8. `destroy` removes the Compose project and keeps volumes.

Record each command's exit code and the relevant output lines.

## 5. Exposure and secret checks

- **Ports:** on the server, every tool listens on 127.0.0.1 only (`ss -tlnp`). From outside, only SSH answers.
- **Secret leaks:** search the CLI's output, its logs, `stack.lock` and the server's container logs for the sentinel prefix. Any match is a finding; report where, never the value.
- **File modes:** secret files on the server are mode 0600 and readable only by their tool.
- **Chat:** a bot with an empty allow-list is rejected at validation.

## 6. Report and teardown

Destroy the disposable server (or confirm the user will), then report:

```markdown
## Olympus test pass — <mode>, <branch> @ <short sha>, <date>

| Front | Result | Evidence |
|---|---|---|
| Static and unit | green/red/not run | exit codes, counts |
| Recipe validation | green/red | recipes checked, sources |
| Edge cases | X tested, Y live, Z gaps | |
| Live cycle | green/red | step results |
| Exposure and secrets | green/red | ports, leak search result |

### Findings (most severe first)
- `path:line` — defect — how to reproduce — severity

### Gaps (edge cases nothing proves)
### Not run (and why)
```

For a tracked feature, add the evidence to its file in `docs/tracker/features/`. Never mark a feature done only because QA passed.
