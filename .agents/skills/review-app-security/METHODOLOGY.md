# Security review methodology (single-file version)

For Olympus, read [project scope](reference/project-scope.md) first. Its trust boundaries, rules A1–A12 and execution limits apply to this fallback too.

Use this in any AI coding tool that doesn't load `SKILL.md` on its own. Paste the whole file into the chat and say: *"Do a security review of my recent changes following this."*

If you use Claude Code, use the `review-app-security` skill instead — it's the same method with less to paste. Companion: after this produces the fix list, the `apply-security-fixes` skill can drive the fixes one per session.

This is a checkpoint review — run it after finishing a feature or a big batch of AI changes, and again before launching. It is **not** a scanner and **not** a guarantee. It finds design and trust flaws that scanners miss, and re-checks your own security rules, which the AI forgets over a long session.

---

## Step 0 — Scope

- **Diff mode (default):** review everything changed since the last review. Git repo → `git diff` against the last reviewed commit/tag. No baseline → ask me "what did you just build?" and review that.
- **Pre-launch mode:** I'm launching soon → review the whole app, and in Step 1 go boundary by boundary instead of file by file.
- Look for a data-flow note (`SECURITY-DATA-FLOW.md` or similar). Read it if present; offer to create one if not.
- **Read the auth/authz helpers the change relies on** — `requireUser`, `getSession`, `assertTeamAdmin`, RLS policy files, route middleware. Open each and read what it *actually* checks; don't trust the name. Can't find the source → mark dependent findings `needs human eyes`.

**Build the change inventory** — one row per review target, verdict filled in as you go:

| File | Target (route / query / mutation / auth check / form / config / migration) | Trust boundary | Verdict |
|---|---|---|---|

The review isn't done until every row has a verdict: a finding, or `safe — <file:line that makes it safe>`. Skipped a row? Say why. This stops the review quietly covering only the interesting files.

State the scope you settled on and show the inventory, then continue.

---

## Step 1 — Threat-model the changes (the design-layer check)

Work the inventory one row at a time. For every new/changed endpoint, route, query, mutation, job, or form, answer four questions:

1. **What trust boundaries does this touch?** browser→server, server→database, webhook→handler, app→third-party API.
2. **Who can call this, and as whom?** Walk every caller-controlled id (`:id`, `?userId=`, id in body). If I change it to someone else's, what stops me? "The UI wouldn't show that link" is **not** a control. Confirm the ownership/role check is **server-side** and **per-record** — not just "is someone logged in." Multi-tenant: every query scoped to the caller's tenant. Can a normal user hit an admin action? Is "admin" from the server session or from the request?
   - **Trace each identifier end to end:** entry line → every hop → the line it hits a query or authz check. Confirm the authz check and the data fetch key on the **same** id with the **same** scoping. Breaks even when a check is present: *wrong key* (`assertOrgAdmin(user.id, body.orgId)` passes, query is `where: { id: params.projectId }`); *nested reference* (parent checked, `include: { members: true }` returns unscoped children — see Step 2 E12).
   - **Read the guard's source**, don't trust its name. "Is a member of the org" ≠ "is an admin of this team."
3. **What's the worst *malicious* input?** Not invalid — malicious: oversized, wrong tenant, replayed, someone else's token, negative/overflow number, wrong type, `../../`, `<script>`, a URL pointing at `169.254.169.254` or localhost, extra fields like `isAdmin: true`, the same request 1000× in parallel.
4. **What happens when it fails?** Fails closed? Error leaks a stack trace, path, DB error, or another user's data?

Then **critique your own answers** — a first pass can miss flaws when it accepts its own assumptions. For each "this is fine because X," check X is actually enforced in the code, not assumed; check you read the *server* code, not a client-side check; check it holds against direct `curl`, not just the UI; check you read the guard helper's source. **For every target you mark `safe`, cite the line that makes it safe (`file:line`)** — "looks fine" is not a verdict.

---

## Step 2 — Line-level checklist

Check every changed file against these (ordered by how often they appear in AI code):

**Access control & tenancy**
1. Every record access filtered by owner (`where: { id, userId: session.user.id }`), not just "logged in." *(#1 AI-code bug.)* When a check *is* present, confirm it and the data fetch key on the same identifier — a check on `body.orgId` feeding a fetch by `params.projectId` is still broken.
2. Every query in a multi-tenant app scoped to the caller's org/team/company.
3. Authorization re-checked on the server, not only in client code.
4. Role / plan / price / `isAdmin` taken from the server session, never trusted from the request.
5. New Postgres/Supabase tables: `enable row level security` + a policy per operation, in the same migration. Deny-all until policies exist.

**Injection & output**
6. Parameterized queries only — no SQL/NoSQL built by string concatenation or template literals.
7. No shell commands built from input — `execFile`/`spawn` with an args array, not `exec` with a string.
8. No file paths built from input — allowlist, and confirm the resolved path stays in the intended directory.
9. Output encoded for context — no `dangerouslySetInnerHTML` / `.innerHTML` / `v-html` with user content unless sanitized (DOMPurify).
10. No `eval` / `new Function` / `setTimeout("string")` touching request data.

**Auth, sessions, crypto**
11. Framework/library auth, sessions, CSRF, password-reset — not hand-rolled.
12. Passwords hashed with argon2id / bcrypt / scrypt — never md5/sha*.
13. Tokens, session ids, invite codes, API keys generated with a CSPRNG (`crypto.randomBytes`/`randomUUID`), never `Math.random()` or counters.

**Secrets & config**
14. No secrets in source, comments, tests, or any client-bundled file. If one was ever committed, **rotate it** — it's burned.
15. Server-only secrets (service-role keys, admin keys) never imported into browser code or passed as props.
16. Secure defaults: `secure + httpOnly + sameSite` cookies; explicit CORS allowlist (not `*`); debug/verbose errors off in production.

**Data exposure & errors**
17. Responses return only the fields the page needs — no `select("*")`, no whole ORM objects, no password hashes / tokens / other users' rows.
18. Errors fail closed and return a generic message + status code — no stack trace / path / DB error to the client. Auth code denies on error, never falls through.
19. No PII, tokens, passwords, or full request/response bodies in logs.

**Input validation**
20. Every request input validated at the boundary with a schema (zod, valibot, pydantic) — type, range, length, format, allowed values. TypeScript types are erased at runtime and don't count. Report this even with no proven exploit.

**Cross-file checks (run in diff mode too — the diff shows half the story):**
- X1. A shared helper / middleware changed → re-open every call site; a guard relaxed "to simplify" weakens all callers. Name the blast radius.
- X2. A new caller of an existing privileged function → does it replicate *every* guard the other callers have (auth, tenant scope, rate limit)?
- X3. New `POST`/`PATCH`/`DELETE` on a resource whose `GET` is guarded — check each verb's ownership check independently.
- X4. Route-prefix auth middleware (`matcher`, `startsWith`) vs. a newly added nested route that falls outside the pattern.
- X5. Admin / service-role / secret-keyed client at module scope → every handler in the file can bypass RLS with it; confirm each re-authorizes.
- Also in diff mode: E2 (webhook signature verified before the handler acts) and E12 (IDOR via nested/indirect reference).

**Calibration — before writing any finding:** check it isn't already handled elsewhere in the change (an auth guard, the ORM's parameterization, framework CSRF, a schema validator, output sanitization). If something already closes the gap, it's `informational` at most, not a finding. "Input isn't validated" is Medium/Low only when raw input actually reaches logic or the DB unchecked — not when a preceding auth/ownership check constrains it. Rate limiting: report only on login / reset / OTP / signup / expensive endpoints, max Medium. **CSRF on a framework route handler:** don't itemize when the app relies on `SameSite` cookies consistently — note it once, make it a finding only if you can see `SameSite=None`. **Parameterization prevents value interpolation attacks; separately check values that reach a shell command, a file path or a template.** **One finding per root cause** — shared fix, one finding. Don't pad the list — buried real findings get ignored.

**Extended (check in pre-launch mode, or if the change touches network calls / file uploads / templates / deserialization / webhooks):**
SSRF (server fetches a user URL → block private IPs + metadata endpoints); webhook signature verification; open redirect; file-upload validation by magic bytes + isolated storage; insecure deserialization (`pickle`, `yaml.load`, `unserialize`); SSTI; prototype pollution; mass assignment (`update({ data: req.body })`); race conditions on balances/seats/coupons; missing rate limits on login/reset/OTP/export; session rotation on login; reset-token expiry + single-use; JWT algorithm pinning + expiry check.

---

## Step 3 — Rules drift re-check

If the project has a security rules block in `CLAUDE.md` / `AGENTS.md`, go through it rule by rule against the changed code, assuming the rules faded from your context over a long session. Every rule the code violates is a finding tagged `rules drift`. No rules block → recommend adding one, move on.

---

## Step 4 — Dependencies, secrets, exposure (advisory — don't scan, point me at tools)

Based on what changed, tell me which to run and what to do with the output:

- **New/changed dependencies** → `npm audit`, or enable **Dependabot** (repo Settings → Security), or **Snyk** (free tier). For any unfamiliar package: check its npmjs.com/pypi.org page has real download counts *and* a linked active repo; read the name character-by-character for lookalikes. Or use **Socket**.
- **Any chance of hardcoded secrets** (source, comments, tests, client bundle, git history) → **gitleaks** or **git-secrets** (they scan history too), or GitHub secret scanning. Found one → rotate first, then move to env var.
- **New public endpoints / changed CORS or headers / a live URL** → **Mozilla HTTP Observatory** (paste URL, no signup) and **OWASP ZAP** Quick Start against your own URL.

Give me a short "run these, paste the reports back" block. The one thing you *do* judge here: if this change made an endpoint/bucket/preview public, is that intended? If not → finding.

---

## Step 5 — The fix list

Rank findings Critical → High → Medium → Low.

- **Critical:** anyone can reach other users' data or privileged actions right now (IDOR, no RLS, hardcoded secret, SQLi, RCE).
- **High:** exploitable with modest effort or leaks sensitive data.
- **Medium:** weakens a defense, needs chaining.
- **Low:** hygiene.

Severity anchoring: over-fetching a non-critical field is **Medium** not High; an unauthenticated PII endpoint is **Critical** not High; parameterized values are not SQL injection; values reaching a shell, a path or a template still need review.

Each finding:

```
### [SEVERITY] Short title
Where: file:line or feature
What's wrong: one plain sentence
Failure scenario: concrete — who does what, what they get
Fix: the actual change
Confidence: confirmed | needs human eyes
Also caught by: <tool/rule, or "nothing automated — design-layer">
Fix session prompt: (Critical/High only) paste-ready block — "New session. Fix only
  this: <one line>. File: <path>. Do not refactor, do not touch other files. Show
  me the diff and the one thing to test by hand."
Verify after: (Critical/High only) who signs in, what they open, what they should
  see — including the step that confirms the legitimate path still works.
```

Start with: the Step 0 inventory table (verdicts filled in), a summary line
(`Targets reviewed: N of N` + `Critical: N  High: N  Medium: N  Low: N`), whether a
data-flow note was used, and — if >1 Critical/High or any fix has a prerequisite —
a **Fix order** list (e.g. "1. Rotate the key (you, in the dashboard). 2. RLS
migration + route-auth change together — RLS alone breaks the current path.").

End with:
1. Fix Critical/High first, in the Fix order. Use the `apply-security-fixes` skill,
   or by hand: **one fresh session per finding** (paste its Fix session prompt),
   then run "verify the fix for <finding>" before moving on.
2. Run each finding's **Verify after** check, including the legitimate-path step —
   security fixes break functionality and you have no QA team.
3. Run the Step 4 scans; paste reports into a fix session.
4. Re-run this whole review after fixes land.

Pre-launch mode: add a **GO / NO-GO** line — any unresolved Critical = NO-GO.

If there are no findings, say so plainly. Do not invent findings to look thorough.

---

## Fix-verification mode

Invoked as *"verify the fix for &lt;finding&gt;"*. Given the original finding + the diff that claims to fix it, report `fixed` / `partial` / `regressed` with lines, checking:
1. Root cause addressed, not just the cited symptom.
2. No new unscoped query / broadened role / loosened CORS-RLS introduced by the fix.
3. Every instance of the pattern fixed — grep for siblings, not just the cited line.
4. Legitimate path still works — name the click-through.

Anything short of all four clean = `partial` (or `regressed` if #2 failed). Stop and report.
