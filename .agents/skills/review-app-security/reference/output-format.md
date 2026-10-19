# Step 5 — The fix list

## Severity rubric

Plain-language, so a non-specialist can act on the ranking:

| Severity | Meaning | Examples |
|---|---|---|
| **Critical** | Anyone can reach other users' data or run privileged actions **right now**, no special skill needed. | IDOR on a data endpoint; table with no RLS; hardcoded live secret; SQL injection; RCE. |
| **High** | Exploitable with modest effort, or leaks clearly sensitive data. | Missing auth on an admin action; weak/missing signature check on a webhook; SSRF; secrets in the client bundle. |
| **Medium** | Weakens a defense; usually needs to be chained with something else. | No input validation at a boundary; verbose error messages; missing rate limit on login; over-fetching non-critical fields. |
| **Low** | Hygiene; unlikely to be the way in but should be fixed. | Enumerable ids where ownership *is* checked; missing security header; PII in a debug log. |

When unsure between two levels, pick the higher one and tag it `needs human eyes`.

### Severity anchoring — worked calls

- Over-fetching a non-critical field (e.g. `stripe_customer_id` to the client) is **Medium**, not High — it needs another bug to cause real harm.
- An unauthenticated endpoint that returns PII is **Critical**, not High — no skill, no chaining, works right now.
- A correctly parameterized query is not SQL injection solely because validation is absent. Separately trace caller-controlled values that reach a shell command, a file path or a template.
- Missing schema validation where a preceding ownership/auth check already constrains the input is **Low**, or `informational` — not Medium.

## Finding format

One block per finding, ordered Critical → Low. **Critical and High** findings carry two extra fields (`Fix session prompt`, `Verify after`); Medium and Low stop at `Also caught by`.

```
### [CRITICAL] Any logged-in user can read any order
Where: app/api/orders/[id]/route.ts:14
What's wrong: The route checks that the caller is signed in but never checks that
  the order belongs to them.
Failure scenario: A signed-in customer opens /api/orders/8fd2... with another
  customer's order id (guessable, or copied from a shared link) and reads that
  order's total and line items.
Fix: Scope the query to the owner — `where: { id: params.id, userId: session.user.id }`
  — and return 404 when there's no match.
Confidence: confirmed
Also caught by: nothing automated — this is a design-layer flaw. SAST and
  `npm audit` will not find it.
Fix session prompt: "New session. Fix only this: the GET /api/orders/[id] route
  returns any order without checking it belongs to the signed-in user. File:
  app/api/orders/[id]/route.ts. Scope the query by session user id, 404 on no
  match. Do not refactor, do not touch other files. Show me the diff and the one
  thing to test by hand."
Verify after: Sign in as user B, open /api/orders/<one of user A's order ids> —
  expect 404. Then open one of your own orders — expect it still loads.
```

Fields:
- **Where** — file:line, or the feature name if it's a design/config issue with no single line.
- **What's wrong** — one sentence, no jargon the reader would have to look up.
- **Failure scenario** — concrete and specific: who does what, and what they get. Not "an attacker could exploit this."
- **Fix** — the actual change, with the code shape where it's short. If the fix is architectural, say what and roughly where.
- **Confidence** — `confirmed` (you can trace the exploit in the code you were shown) or `needs human eyes` (depends on code you didn't see, or on how something is deployed).
- **Also caught by** — name the tool/rule, or say "nothing automated." This tells the reader what their scanning tools will and won't back them up on.
- **Fix session prompt** *(Critical/High only)* — a block the reader can paste verbatim into a fresh session or the `apply-security-fixes` skill. Scoped to this one finding; explicitly forbids refactoring and unrelated edits.
- **Verify after** *(Critical/High only)* — the plain-English regression check: who signs in, what they open, what they should see. Include the "confirm the legitimate path still works" step — access-control fixes break normal use constantly.

## Fix order

When there is more than one Critical/High finding, or any fix has a prerequisite, put an ordered list above the findings:

```
## Fix order
1. Rotate the Stripe key — Stripe dashboard, you not the agent. It's already burned.
2. Land the RLS migration and the /api/profile auth change together — enabling RLS
   alone breaks the current service-role code path.
3. Then the remaining High/Medium findings, any order.
```

## Summary header

Above the findings (and above the Fix order block), include the Step 0 inventory table with every verdict filled in, then:

```
## Security review — <diff since abc1234 | pre-launch, whole system>
Data-flow note: <found and used | not present, built a rough one | not present>
Targets reviewed: N of N   (every inventory row has a verdict)

Critical: N   High: N   Medium: N   Low: N
```

Safe rows in the inventory carry the line that clears them, e.g. `safe — assertTeamAdmin at route.ts:12`. If `Targets reviewed` is less than the total, name the rows you skipped and why.

## Closing block (always include)

```
## Next steps

1. Fix Critical and High findings first, in the Fix order above. Two ways:
   - Run the `apply-security-fixes` skill on this list — it does them one at a
     time, checks each with fix-verification mode, and stops on a partial fix.
   - Or by hand: a FRESH session per finding, paste its "Fix session prompt",
     let it work, then run "verify the fix for <finding>" before moving on.
2. After each fix, run that finding's "Verify after" check — including the step
   that confirms the legitimate path still works. Security fixes (access rules,
   RLS, CORS, auth guards) break normal functionality constantly, and you don't
   have a QA team.
3. Run the scans listed in Step 4 and paste the reports into a fix session.
4. Re-run this whole review after the fixes land — fixes introduce regressions.
```

Add for **pre-launch mode**:

```
## Go / no-go

<No unresolved Critical findings in the reviewed scope; list remaining findings and unverified deployment/provider checks. This is not deployment approval.>
  — or —
<NO-GO — do not launch. Unresolved Critical: [list]. Fix and re-review first.>
```

## If there are no findings

Say so plainly. Don't manufacture findings to look thorough — a clean review of a small, careful change is a real and correct outcome. Still show the inventory table with every row marked `safe — <line>` — that's how the reader sees you checked everything, not just eyeballed it. Don't reach outside the diff for a pre-existing smell to have "something" to report.

```
## Security review — diff since abc1234
Targets reviewed: 3 of 3
No security issues found in this change.

Notes (not vulnerabilities):
- Rotating the invite token invalidates previously shared links — expected; let
  teammates know.

Still worth doing: run `npm audit` if you haven't lately (unrelated to this change).
```
