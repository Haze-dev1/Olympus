# Release review of development against main (2026-11-22)

A code review of everything `development` carries ahead of `main`, done before promoting it. `development` is 42 PRs ahead of `main` (#19 to #60): 212 files and about 16,000 lines. The review read the code only; nothing was run on a server. The four serious findings were each checked against the code. The other nine are minor.

This complements the same day's [security review](review-2026-11-22.md), which covered only the changes since 2026-11-06.

Status values: **Open**, **Fixing** (with the issue), **Fixed** (with the PR).

## Summary

| ID | Severity | Area | Finding | Status |
|---|---|---|---|---|
| [1](#1-openclaw-cannot-read-its-skills) | Serious | deploy, OpenClaw recipe | OpenClaw can't read its skills: store files are root-owned 0600, and OpenClaw runs as uid 1000 | Fixed (OLY-68) |
| [2](#2-pi-accepts-an-sse-mcp-server-it-cannot-use) | Serious | eval, Pi recipe | Pi accepts an SSE MCP server that it can't use | Fixed (OLY-69) |
| [3](#3-a-tool-restarts-before-its--init-step-when-its-secrets-change) | Serious | activate | A tool restarts before its `-init` step has run when its secrets change, including on rollback | Fixed (OLY-67) |
| [4](#4-the-port-check-reports-free-ports-as-in-use) | Serious | deploy preflight | The port check reports free ports as in use | Fixed (OLY-70) |
| [5](#5-post_install-arguments-must-be-unique) | Minor | schema | `post_install` arguments must be unique | Fixed (OLY-69) |
| [6](#6-large-catalog-repositories-cannot-be-fetched) | Minor | catalog | Large catalog repositories can't be fetched | Fixed (OLY-69) |
| [7](#7-a-warning-that-olym-update-cannot-clear) | Minor | lock, CLI | A warning that `olym update` can't clear | Fixed (OLY-69) |
| [8](#8-the-development-build-warning-appears-when-it-should-not) | Minor | CLI, release build | The development-build warning appears when it shouldn't | Fixed (OLY-70) |
| [9](#9-the--init-service-lacks-the-recipes-hardening) | Minor | eval | The `-init` service lacks the recipe's hardening | Fixed (OLY-68) |
| [10](#10-the-restore-failure-path-leaves-old-containers-running) | Minor | activate | The restore-failure path leaves old containers running | Fixed (OLY-67) |
| [11](#11-missing-doc-comments) | Minor | code standard | Missing doc comments | Fixed (OLY-70) |
| [12](#12-package-level-mutable-state-for-version-and-checksums) | Minor | code standard | Package-level mutable state for the version and checksums | Fixed (OLY-70) |
| [13](#13-two-rollback-tests-only-check-that-an-error-happened) | Minor | tests | Two rollback tests only check that an error happened | Fixed (OLY-67) |

## Serious

### 1. OpenClaw cannot read its skills

Store files are written with mode 0600 (`internal/deploy/deploy.go:188`) and unpacked as root, so they end up owned by root and readable only by root. They are then mounted into the OpenClaw container, which runs as `node` (uid 1000). Reading them gives a permission error. OpenClaw is the only affected recipe: every other recipe that mounts files runs as root. OpenClaw's only evidence is offline golden-file tests, so nothing ever read a skill inside a running OpenClaw container.

### 2. Pi accepts an SSE MCP server it cannot use

Pi uses the `hermes` config style, which writes `"transport": "sse"` (`internal/eval/eval.go:654`). But `recipes/pi/README.md:72` says Pi rejects SSE. Codex already gets a build-time error for this case; Pi gets none, so `check` and `apply` pass and the server fails silently at runtime. Small fix.

### 3. A tool restarts before its -init step when its secrets change

`startWithSecrets` recreates the tools whose secrets changed with `--no-deps` before the normal `up` (`internal/activate/activate.go:409-416`), so their `-init` service is skipped. The new rollback path, `startWithSecrets(prev, restoredTools(...))`, goes through this too, so a rolled-back Hermes could keep running the failed generation's `config.yaml`. The order was confirmed in the code. Not confirmed: whether Compose then declines to restart the tool, which depends on how Compose decides a service changed. A lab test that rotates a secret and then forces a failed health check would settle it.

### 4. The port check reports free ports as in use

`conflictsWithLoopback` (`internal/deploy/preflight.go:211`) treats a listener on `[::1]`, or on any 127.x address, as blocking a bind to 127.0.0.1. Linux allows those binds side by side. It only triggers in that setup, but when it does, both `plan` and `apply` fail and there is no way to override it.

## Minor

### 5. post_install arguments must be unique

`schemas/recipe.schema.json:291` requires `post_install` arguments to be unique, so `[cp, -a, /x, /x]` or a repeated flag is rejected. This predates the reviewed range.

### 6. Large catalog repositories cannot be fetched

The tarball limits now count every entry in the repository, not only the recipe folder, so a large monorepo fails at 10,000 entries. This may be intended hardening from the security review (R2); if so, document the limit.

### 7. A warning that olym update cannot clear

The missing-platforms warning points to `olym update`, but `update` never fills in platforms for images already pinned by digest.

### 8. The development-build warning appears when it should not

It appears even when `-unsafe-dev-activator` is passed, and on release-script builds from untagged commits, because those fall back to the same `0.1.0-dev` version.

### 9. The -init service lacks the recipe's hardening

The `-init` service doesn't get `no-new-privileges` or the seccomp profile, yet it runs as root against the tool's volume.

### 10. The restore-failure path leaves old containers running

Its `Down` call doesn't pass `--remove-orphans`, so tools that only the previous generation had stay up.

### 11. Missing doc comments

`recipe.Runtime`, `DevelopmentVersion` and `Version` have no doc comments, which `AGENTS.md` requires.

### 12. Package-level mutable state for version and checksums

`var Version` and the checksum variables are package-level mutable state, which `AGENTS.md` forbids. They are needed for `-ldflags -X`, so record that as an exception in `AGENTS.md` rather than restructuring the code.

### 13. Two rollback tests only check that an error happened

They don't check the error type, or that `current` still points at the previous generation, so a broken rollback would still pass.

## Suggested order

1 and 2 first (clear bugs, small fixes), then 3 with a lab test to confirm it, then 4. Lab checks are needed for 1 and 3.
