---
name: apply-security-fixes
description: >-
  Works through a security review's fix list one finding at a time — applies the
  fix in isolation, fixes sibling instances of the same pattern, runs the
  finding's verify-after check, then runs fix-verification mode on the diff and
  stops if the fix is only partial. Use after review-app-security (or its
  METHODOLOGY.md) has produced a ranked fix list. It does not find new issues;
  run review-app-security for that.
---

# Apply security fixes

## Project setup

Read [Olympus scope and evidence](../review-app-security/reference/project-scope.md) before the workflow. It defines the trust boundaries and the Olympus-specific rules (A1–A12) that a fix must not break.


Takes the output of `review-app-security` and drives the remediation loop the review hands off: one finding, one fix, one verification, then the next. It exists because "start a fresh session per finding" is the right process but a lossy hand-off for someone who isn't a security engineer — this skill keeps each fix scoped, checks it landed, and refuses to march past a fix that didn't hold.

**What this does NOT do:** it does not hunt for new vulnerabilities, re-threat-model, or second-guess the review's severities. If you want that, run `review-app-security` again after this finishes. It also never rotates a secret, rewrites git history, or changes a hosting/dashboard setting for you — those it hands back to you with instructions.

---

## Workflow

Copy this checklist into your reply and work top to bottom.

```
Apply-fixes progress:
- [ ] Step 0: Load and parse the fix list
- [ ] Step 1: Confirm fix order + surface manual prerequisites
- [ ] Step 2: For each Critical/High — fix in isolation, incl. sibling instances
- [ ] Step 3: Run the finding's "Verify after" check
- [ ] Step 4: Run fix-verification mode on the diff — stop on partial/regressed
- [ ] Step 5: Summary — done / manual actions / re-run the review
```

---

### Step 0 — Load and parse the fix list

Take the review output — pasted into the chat, or a file path the user gives you. Parse it into an ordered list of findings, each with its severity, `Where`, `Fix`, and (for Critical/High) `Fix session prompt` and `Verify after`.

If the report lacks severity, location, or a concrete fix, ask for those missing fields. Accept equivalent evidence-backed reports; do not require a new full review solely for formatting.

State how many findings you have at each severity, and which ones you're going to act on (Critical + High by default; ask before doing Medium/Low).

---

### Step 1 — Confirm fix order + manual prerequisites

- If the review has a **Fix order** block, follow it. If it doesn't and there's more than one finding, propose an order (prerequisites first, then Critical, then High) and confirm with the user.
- Surface every step that is **not yours to do in the repo**: rotating a leaked key, changing a Supabase/Vercel/Stripe dashboard setting, scrubbing git history, revoking a token. List them, and get the user's acknowledgement that they'll handle those, before dependent code changes. Independent authorized fixes may proceed while a manual prerequisite remains open.
- Confirm you have a way to tell fixes apart — ideally each fix is its own commit on a branch.

---

### Step 2 — Fix one finding, in isolation

For each finding you're acting on, one at a time:

- Apply **only** that fix. No refactoring, no reformatting, no "while I'm here" changes, no touching files the finding doesn't name — unless the next bullet applies.
- **Grep for sibling instances of the same pattern** and fix those too: other routes with the same missing check, the same `select("*")`, the other HTTP verbs on the same resource, the other callers of a helper you changed. The review may have cited one line; the bug is often in three places. List every location you changed.
- Keep the change minimal and readable. If the real fix is architectural (needs a new middleware, a schema change, an RLS migration), say so and get confirmation before doing it — don't force a shallow patch.
- Produce the diff.

**Get the user's confirmation before:** running a database migration, changing authentication or access-control code, installing a package, deleting a file, or pushing to a remote.

---

### Step 3 — Run the "Verify after" check

Run the finding's `Verify after` steps:

- If it's automatable (a `curl`, a test you can write and run), do it and show the result.
- If it needs a browser or a second real user account, use an available browser against isolated synthetic accounts when authorized; otherwise give exact manual steps and mark verification blocked until evidence arrives.
- Always include the "confirm the legitimate path still works" step — an access-control fix that also breaks the owner's own access is not done.

---

### Step 4 — Fix-verification mode on the diff

Read [review-app-security](../review-app-security/SKILL.md) and use its **fix-verification mode** against this finding + the diff you just produced ("verify the fix for &lt;finding&gt;"). It checks four things and returns `fixed` / `partial` / `regressed`:

1. Root cause addressed, not just the cited symptom.
2. No new unscoped query / broadened role / loosened CORS or RLS introduced by the fix.
3. Every instance of the pattern fixed, not only the cited line.
4. The legitimate path still works.

- `fixed` → mark the finding done, move to the next.
- `partial` or `regressed` → **stop.** Report what's still open (or newly broken), and either take one more focused pass at it or hand it back to the user. Do not move on to the next finding with an unresolved one behind you.

---

### Step 5 — Summary

- **Fixed:** list each finding, the files/lines changed, and the verify result.
- **Needs your action:** the manual prerequisites from Step 1, plus anything a fix surfaced (a key to rotate, a setting to change, a test to run in the browser).
- **Still open:** any finding that came back `partial`/`regressed` and why.
- **Next:** run the full `review-app-security` pass again — fixes introduce regressions, and a clean re-review is the only way to know the list is actually closed. In pre-launch mode, don't launch until that re-review is clean.
