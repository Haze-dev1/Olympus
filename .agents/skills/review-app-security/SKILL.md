---
name: review-app-security
description: >-
  Runs a deliberate security review of recent changes to an AI-built web app.
  Threat-models the changes for broken access control and tenant isolation (the
  design flaws scanners miss), checks every changed file against an OWASP/CWE
  line-level checklist, re-verifies the project's security rules for session
  drift, points the user at the right scanning tools for dependencies and
  secrets, and outputs a severity-ranked fix list. Use after finishing a feature,
  after a big batch of AI-generated changes, or before launching. Reviews the
  diff by default; does a whole-system pass in pre-launch mode.
---

# Review app security

## Project setup

Read [Olympus scope and evidence](reference/project-scope.md) before the workflow. It defines the trust boundaries, which generic checklist items apply, the Olympus-specific rules (A1–A12) and execution limits.


A checkpoint review for someone who builds with an AI coding tool and is not a security engineer. It runs after a chunk of work is done — not continuously, and not once at the very end.

**What this catches:** design and trust flaws — one user reaching another user's data, a check that only exists in the UI, a database table anyone can read. Static scanners are blind to these. It also re-checks the security rules in the project's `CLAUDE.md` / `AGENTS.md`, which fade from the model's attention over a long session.

**What this does NOT do:** it is not a scanner and not a guarantee. It tells the user which scanning tools to run (see Step 4) rather than reimplementing them, and the user should keep running those tools independently. Findings still need human judgment. Ship nothing on the strength of a clean review alone.

---

## Workflow

Copy this checklist into your reply and work through it top to bottom. Do not skip steps. Do not fix anything yet — this pass only produces the list.

```
Security review progress:
- [ ] Step 0: Set scope, build the change inventory, load the data-flow note
- [ ] Step 1: Threat-model the changes (4 questions + self-critique)
- [ ] Step 2: Line-level checklist pass over every changed file
- [ ] Step 3: Re-check the project security rules for drift
- [ ] Step 4: Dependency + secrets + exposure — name the tools to run
- [ ] Step 5: Write the ranked fix list
```

---

### Step 0 — Scope

Decide what you are reviewing:

- **Default (diff mode):** everything that changed since the last review. If this is a git repo, diff against the last reviewed commit or tag; otherwise diff the working tree against `HEAD`. If you cannot establish a baseline, ask the user: *"What did you just build or change?"* and review that.
- **Pre-launch mode:** the user says they're launching, or answers yes to *"Is this going live soon?"* Review the whole application, and in Step 1 work through each trust boundary rather than each changed file.

Then load context:

- Look for a data-flow note in the repo (`SECURITY-DATA-FLOW.md`, `docs/data-flow.md`, or similar). If it exists, read it — it tells you what data is sensitive and who the users/tenants are.
- If it does not exist, offer to create one from [reference/data-flow-note.md](reference/data-flow-note.md). Build a rough version as you go through Step 1 either way.
- **Read the auth/authz helpers the change relies on.** If the code calls `requireUser`, `getSession`, `assertTeamAdmin`, an RLS policy file, or route middleware, open the source of each and read *what it actually checks*. A helper named `assertTeamAdmin` can still check the wrong thing. If you cannot find the source, mark every finding or safe verdict that depends on it `needs human eyes`.

Then **build the change inventory** — a table with one row per review target, filled in as you go:

| File | Review target (route / query / mutation / auth check / form / config / migration) | Trust boundary crossed | Verdict |
|---|---|---|---|
| e.g. `app/api/orders/[id]/route.ts` | `GET` handler + `db.order.findUnique` | browser → server → DB | (fill in Steps 1–2) |

The review is not complete until every row has a verdict: a finding ID, or `safe — <file:line that makes it safe>`. If you skip a row, say why. This is what stops the review quietly covering only the two most interesting files.

State the scope and baseline you settled on, then show the inventory before continuing.

---

### Step 1 — Threat-model the changes

This is the design-layer check. Full method, worked examples, and the STRIDE crosswalk: **[reference/threat-model.md](reference/threat-model.md).**

For each new or changed endpoint, route, query, mutation, or form, answer four questions:

1. **What trust boundaries does this touch?** (browser → server, server → database, webhook → handler, app → third-party API)
2. **Who can call this, and as whom?** Walk it for IDOR and tenant isolation: *can user A read or change user B's data by editing an ID, a cookie, or a request body?* Confirm the ownership/role check runs **on the server**, not just in the UI, and is scoped per record — not just "is someone logged in." **Trace each caller-controlled identifier end to end** and confirm the authz check and the data fetch key on the *same* id — a check on `body.orgId` feeding a fetch by `params.projectId` is still broken. Read the guard helper's source, don't trust its name.
3. **What is the worst *malicious* input?** Not "invalid" — think oversized, wrong tenant, replayed, someone else's token, negative or overflowing number, unexpected type.
4. **What happens when it fails?** Does it fail closed? Does the error leak a stack trace, an internal path, a database error, or another user's data?

Then **critique your own four answers** — go back over each answer and check whether its assumptions are supported by the source. **For every target you mark safe, cite the specific line (`file:line`) that makes it safe** — "looks fine" is not a verdict.

Record anything that isn't clearly safe as a candidate finding for Step 5.

---

### Step 2 — Line-level checklist

Go through **[reference/checklist.md](reference/checklist.md)** against every changed file. It's ~20 items ordered by how often they show up in AI-generated code, each with what to look for and what a fix looks like — plus a **cross-file block (X1–X5)** for the bugs the diff hides (a shared helper relaxed, a new caller that drops a guard, a write verb missing the read path's check). The long tail (SSRF, deserialization, SSTI, mass assignment, and more) is in [reference/checklist-extended.md](reference/checklist-extended.md) — scan it too in pre-launch mode, or when the change touches network calls, file handling, or serialization.

Keep genuine low-severity findings — for this audience, "input isn't validated" and "error leaks the stack trace" are worth reporting even without a proven exploit. But **before writing any finding, check it isn't already handled elsewhere in the change** (an auth guard, the ORM's parameterization, framework CSRF, a schema validator). See the "calibration" note at the top of the checklist. A review padded with theoretical findings gets ignored.

---

### Step 3 — Rules drift re-check

If the project has a security rules block in `CLAUDE.md` / `AGENTS.md`, work through it rule by rule against the changed code, assuming the rules faded from context during a long session. Use the drift table in **[reference/checklist.md](reference/checklist.md)** (bottom section). Every rule that the code violates becomes a finding, tagged `rules drift`.

If there is no rules block, recommend adding one (a short plain-language security rules section in `CLAUDE.md` / `AGENTS.md`) and move on.

---

### Step 4 — Dependencies, secrets, exposure

This skill does not scan — dedicated tools do it better. Your job is to say **which tool to run for what**, based on what changed. Details and the current tool list: **[reference/dependencies-secrets.md](reference/dependencies-secrets.md).**

- New or changed dependencies → dependency scanner (`npm audit`, Dependabot, Snyk) for known CVEs; verify any unfamiliar package is real and maintained (or use Socket).
- Any risk of hardcoded secrets — in source, comments, tests, client bundles, or git history → a secrets scanner (gitleaks, git-secrets, or Semgrep's secret rules).
- New public endpoints, changed CORS/headers, or a live URL → Mozilla HTTP Observatory plus the manual checks.

Output a short "run these, paste the reports into a fix session" block. The one thing you *do* reason about here: is a newly public endpoint or bucket *meant* to be public? That judgment belongs in Step 1's output, not a scanner's.

---

### Step 5 — The fix list

Format, severity rubric, and the fix-session instructions: **[reference/output-format.md](reference/output-format.md).**

- Rank findings by severity (Critical → High → Medium → Low). Each gets: what's wrong (one sentence), a concrete failure scenario, the fix, a confidence tag (`confirmed` / `needs human eyes`), and whether any automated tool would also catch it. Critical/High findings also get a paste-ready **Fix session prompt** and a plain-steps **Verify after** check.
- If there is more than one Critical/High, or a fix has a prerequisite, put a **Fix order** block above the findings.
- End with: fix Critical/High first — the **`apply-security-fixes` skill** can drive this, or do it by hand one **fresh session per finding**; **click through every affected feature after fixing** because security changes (access rules, CORS, RLS) often break functionality; and, in pre-launch mode, a **go / no-go** line — any unresolved Critical means do not launch.

---

## Fix-verification mode

Invoke as *"verify the fix for &lt;finding&gt;"* (the `apply-security-fixes` skill calls this after each fix). Given the original finding plus the diff that claims to fix it, check four things and report `fixed` / `partial` / `regressed` with the lines that justify it:

1. **Root cause, not symptom.** Does the fix address the underlying flaw, or just the one line the finding cited?
2. **No new surface.** Did the fix introduce an unscoped query, a broadened role, a loosened CORS/RLS rule, or a new caller that skips a guard?
3. **Every instance.** Grep for siblings of the same pattern (other routes, other verbs, other callers). Are they all fixed, or only the cited one?
4. **Legitimate path intact.** Name the exact click-through that confirms the feature still works for an authorized user.

Anything short of all four clean is `partial` (or `regressed` if #2 failed). Stop there and report — don't wave it through.
