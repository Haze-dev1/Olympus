# Olympus scope and evidence

Read this before the generic checklist. Paths are relative to the repo root.

## What Olympus is, for a reviewer

A Go CLI that a developer runs on their own machine. It reads `stack.yaml`, `stack.lock` and the pinned recipes, renders a generation locally (Compose file, config files, manifest), then connects to the developer's server over SSH as root or a sudo user, installs Docker, uploads the generation to `/var/lib/olympus/`, writes secret files and runs one Compose service per AI tool (Hermes, Codex, OpenCode and others). Those containers hold model API keys, chat bot tokens and code-access tokens. There is no web app, no database and no multi-user login in v1.

Read `AGENTS.md`, `docs/PRD.md`, `docs/ADR.md`, `docs/architecture/design.md`, `docs/specs/` and `docs/edgecases/` before choosing expectations. As of 2026-10-19 there is no code; a review before code exists can only review docs and recipes.

## Trust boundaries

| Boundary | What crosses it | Main risk |
| --- | --- | --- |
| Stack file and recipes → CLI | YAML, templates, hook commands | Values reaching a shell, a path or a template engine unescaped |
| `.env` → CLI → server | Secret values | Secrets in output, logs, the lock file, process arguments or world-readable files |
| CLI → server (SSH) | Commands run as root; the activator binary | Command injection; unverified host keys; a swapped activator binary running as root |
| Registry → server | Container images | Unpinned or swapped images |
| Git → CLI | The recipes input at a locked revision | A moved or tampered revision; check the locked hash |
| Server → internet | Tool ports, chat bots | Gateways exposed without auth; bots answering anyone |
| Container → container / host | Volumes, Docker socket | One tool reading another's secrets; container escape via the Docker socket |

## Applying the generic checklist here

- Items about per-record ownership, tenants, RLS, CSRF, cookies, CORS, XSS and uploads (checklist 1–5, 9 and the web items) are **N/A** while there is no web app or database. Record N/A with this reason; keep the rule numbers.
- Item 7 (shell construction) and item 8 (path construction) are the **main** risks: the CLI runs commands as root built partly from stack-file and recipe values.
- Item 10 (`eval`) covers template engines: a template must not be able to run code beyond the variables it is given.
- Treat recipes as code, even though they are YAML: `hooks.post_install` is a command, and templates write files.

## Olympus-specific rules (A1–A12)

These are in `AGENTS.md` under "Olympus-specific rules". Check each against the change, next to the 15-rule drift table in [checklist.md](checklist.md).

| # | Rule | Status | Note |
|---|---|---|---|
| A1 | Secret values never in stack file, lock file, plan output, logs, errors or process arguments | | |
| A2 | Remote commands built from argument lists with every argument quoted; no unquoted interpolation | | |
| A3 | Template destinations and volume paths resolve inside the tool's own directories | | |
| A4 | Ports bound to 127.0.0.1 unless `ports.public` says otherwise | | |
| A5 | Each container gets only its own secrets; secret files mode 0600 | | |
| A6 | SSH host keys verified; host key checking never disabled silently | | |
| A7 | Images pinned by digest once a lock exists | | |
| A8 | Recipe hooks run inside the tool's container, never on the host | | |
| A9 | YAML parsed with a safe loader | | |
| A10 | Chat channels require a non-empty allow-list | | |
| A11 | Only the `olympus` Compose project and `/var/lib/olympus/` are changed or removed | | |
| A12 | The activator binary's checksum and version are verified before it runs as root | | |

## Bound execution and reporting

Review produces findings only. Fixes need an authorized request through `apply-security-fixes`. Never run `apply` or `destroy` against a real server as part of a review; live checks belong to `olympus-test` on a disposable server. Never read `.env` or print secret values; refer to secrets by variable name. Cite `file:line` for findings and safe verdicts. A clean review is not release approval.
