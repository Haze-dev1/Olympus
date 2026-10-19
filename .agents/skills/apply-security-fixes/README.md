# apply-security-fixes

## Installation in this repo

This copy lives in `.agents/skills/` and is routed from root `AGENTS.md`. Read the project scope reference linked by `SKILL.md` before the upstream workflow below. Security inputs are root `AGENTS.md` (its "Security rules" and "Olympus-specific rules" sections); the generic rules template is `.agents/skills/security-rules.md`. The installation steps further down describe the upstream package.

A companion skill to `review-app-security`. It takes that review's ranked fix list and works through it **one finding at a time**: applies the fix in isolation, fixes the other places the same bug appears, runs the finding's verify-after check, then double-checks the diff with fix-verification mode — and stops if a fix only partly landed.

**Run `review-app-security` first.** This skill acts on that skill's output; it does not look for new issues.

---

## Why it's separate

The review deliberately ends at the list and tells you to "fix each finding in a fresh session." That's the right process — a fresh context per fix keeps the model from wandering — but it's a lot of manual copy-paste, and it's easy to mark a finding done when the fix only covered one of the three places the bug lives. This skill runs that loop for you and holds the line on verification.

---

## Install

### Claude Code

Copy the `apply-security-fixes/` folder to either:

- `.claude/skills/` in your project, or
- `~/.claude/skills/` (all your projects)

Then, after a review:

```
> use apply-security-fixes on the review above
```

or point it at a file:

```
> use apply-security-fixes on ./security-review.md
```

### Other tools

If your tool doesn't load `SKILL.md` on its own, the workflow in it is short — paste it into the chat and say "work through my fix list following this."

---

## What it will and won't do

**Will:** apply each fix minimally, grep for and fix sibling instances, run automatable checks, run fix-verification mode, stop on a partial fix, one finding per commit.

**Won't:** find new vulnerabilities, change severities, rotate secrets, rewrite git history, or change hosting/dashboard settings — those come back to you with instructions. It also asks before migrations, auth-code changes, package installs, deletes, and pushes.

Not a guarantee. Re-run `review-app-security` after it finishes. No warranty.
