# review-app-security

## Installation in this repo

This copy lives in `.agents/skills/` and is routed from root `AGENTS.md`. Read the project scope reference linked by `SKILL.md` before the upstream workflow below. Security inputs are root `AGENTS.md` (its "Security rules" and "Olympus-specific rules" sections); the generic rules template is `.agents/skills/security-rules.md`. The installation steps further down describe the upstream package.

An AI agent skill that runs a **security review of the code you just built** — the design and trust flaws automated scanners can't see: one user reaching another user's data, a permission check that only exists in the UI, a database table anyone can read. It also re-checks the security rules in your project's `CLAUDE.md` / `AGENTS.md`, which AI coding tools tend to forget over a long session.

It works alongside two things you set up separately:

- **A security rules block in your `CLAUDE.md` / `AGENTS.md`** — plain-language rules the AI follows *while* it writes code, so less is broken to begin with. This skill re-checks those rules against the actual diff for drift, and recommends adding a block if you don't have one.
- **`apply-security-fixes`** — a companion skill that takes this review's fix list and works through it one finding at a time.

**It is not a guarantee.** It won't catch everything. Keep running your usual code scanning tools too — dependency/CVE scanners, secret scanners, SAST — for the classes of issue this review doesn't cover. Adapt the skill over time for your app and for new threats. No warranty.

---

## Install

### 1. Add the security rules block (any tool)

Open `security-rules.md` at the top level of this repo (one directory up from this README). Copy its **`## Security rules`** section — everything from that heading to the end of the file — and paste it at the **bottom** of your own project's `CLAUDE.md` (or `AGENTS.md`, if that's what your tool reads). Keep your own project description and conventions above it.

Then fill in the one placeholder line at the top of that block — `**This app's threat model:**` — with a sentence describing who uses your app, what data it holds, and where it runs.

This is what the review's Step 3 checks the code against. The skill still works without it (it'll just recommend adding one), but you get more out of it with the block in place.

### 2. Install the skill

#### Claude Code

Copy the `review-app-security/` folder to either:

- `.claude/skills/` in your project (just that project), or
- `~/.claude/skills/` (all your projects)

Then in a session:

```
> use the review-app-security skill on my recent changes
```

or, before launch:

```
> run review-app-security in pre-launch mode
```

#### Any other AI coding tool

If your tool doesn't pick up `SKILL.md` on its own, open `METHODOLOGY.md` — the same method in one file — paste it into the chat, and say:

```
Do a security review of my recent changes following this.
```

Check your tool's docs for how it loads skills or reusable instruction files; if it has a native mechanism, point it at `SKILL.md` and the `reference/` folder instead.

---

## When to run it

- After you finish a feature.
- After a big batch of AI-generated changes or fixes.
- Before you launch, or before you give people a link (`pre-launch mode` — reviews the whole app, ends with a go/no-go).

Not continuously (that's what the security rules block in your `CLAUDE.md` / `AGENTS.md` is for) and not only once at the end (you'll drown in findings).

---

## What you get

A change inventory (every route/query/migration the diff touched, each with a verdict), then a list of findings ranked Critical → Low. Each finding has a concrete "here's how someone abuses this" scenario, the fix, and whether an automated tool would also catch it. Critical/High findings also carry a paste-ready fix prompt and a plain-English "verify after" check.

Then: fix the Critical and High ones first, in the stated order — either with the optional **`apply-security-fixes`** companion skill (one fix per session, verified), or by hand **one fresh AI session per finding**. Either way, **click through every feature afterward** — security fixes break functionality more often than you'd think, and you don't have a QA team.

---

## Files

| File | What it is |
|---|---|
| `SKILL.md` | The skill entry point — workflow + links to the reference files |
| `METHODOLOGY.md` | The whole method in one file, to paste into any tool that doesn't load `SKILL.md` directly |
| `reference/threat-model.md` | Step 1 in depth — the four questions, examples, STRIDE |
| `reference/checklist.md` | Step 2 — the ~20-item line-level checklist + rules-drift table |
| `reference/checklist-extended.md` | The long tail (SSRF, deserialization, race conditions, …) |
| `reference/dependencies-secrets.md` | Step 4 — which scanning tool to run for what |
| `reference/output-format.md` | Step 5 — the finding format and severity rubric |
| `reference/data-flow-note.md` | Template for the one-page repo file the review reads as input |
| `reference/project-scope.md` | Olympus trust boundaries, checklist applicability and rules A1–A12 |

Companion skill: **`apply-security-fixes/`** — takes this review's fix list and works through it one finding per session, running fix-verification mode after each.

---

## Also run Anthropic's built-in review

Claude Code ships a `/security-review` command and a GitHub Action. Run those too — they're tuned differently (low false positives, PR-oriented, no threat-modeling step).
