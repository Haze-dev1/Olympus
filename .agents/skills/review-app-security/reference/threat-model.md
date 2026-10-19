# Step 1 — Threat-modeling the changes

## Contents
- Why this step exists
- The four questions, in detail
- The self-critique pass
- STRIDE crosswalk
- Worked examples
- Pre-launch mode

## Why this step exists

Static analysis and pattern scanners read code for known-bad shapes: a string-built SQL query, a hardcoded key, `eval` on user input. They cannot tell whether the *design* is sound — whether an endpoint that looks fine is missing an ownership check, whether one tenant can reach another's data, whether a role is trusted because the client sent it. Those are the flaws that most often take down AI-built apps, and the only way to find them is to reason about the change deliberately. That's what this step is.

Keep it anchored to **what changed**, not "the architecture" — the architecture of an AI-built app is out of date within a day. Ten to fifteen minutes.

## Build the inventory first

Before you threat-model anything, list every review target the change introduces or touches — each endpoint, route, query, mutation, background job, form, config block, and migration — in the Step 0 inventory table. Threat-model them one row at a time and fill in the verdict column. This is deliberate: it stops you from reasoning hard about the one file that looks interesting and declaring the review done. If a target is genuinely out of scope, write that in its verdict cell rather than leaving it off the table.

## The four questions, in detail

Ask them for every new or changed **endpoint, route, query, mutation, background job, or form**.

### 1. What trust boundaries does this touch?

A trust boundary is any point where data or a request crosses from less-trusted to more-trusted. Common ones:

- **Browser → server** — anything from the client is attacker-controlled: body, query string, headers, cookies, uploaded files.
- **Server → database** — is the query built safely, and scoped to the right rows?
- **Webhook / third-party callback → your handler** — is it authenticated (signature check), or will you act on anything posted to the URL?
- **Your app → third-party API** — are you sending more than you should? Trusting the response too much?

List which ones this change crosses. Each is a place to check the other three questions.

### 2. Who can call this, and as whom?

The highest-value question. Broken access control is the #1 vulnerability class in AI-generated apps, and the specific failure is code that checks *"is the user logged in?"* but not *"is this user allowed to touch **this** record?"*

- **Walk every identifier the caller controls** — `:id` in the path, `?userId=` in the query, an `orgId` in the body, a record id in a form. For each: if I change it to point at someone else's data, what stops me? If the answer is "the UI wouldn't show me that link," that is **not** a control — the finding is real.
- **Trace each caller-controlled identifier end to end.** Write it out: (a) the line where it enters, (b) every function it's passed through, (c) the line where it reaches a query or an authz decision. Then confirm the **authz check and the data fetch key on the *same* identifier with the *same* scoping.** Two ways this breaks even when a check is clearly present:
  - *Wrong key:* `assertOrgAdmin(user.id, body.orgId)` passes, but the query is `where: { id: params.projectId }` — the caller is admin of *an* org, and reads a project in a *different* one.
  - *Nested reference:* the parent is ownership-checked, but `include: { members: true }` / a join returns child rows that were never scoped. See checklist E12.
- **Confirm the check is server-side.** A disabled button, a hidden menu item, a client-side role check — none of these are enforcement. The server must re-check on every request.
- **Confirm it's per-resource, not just per-session.** `getSession()` returning a user is authentication. `where: { id, userId: session.user.id }` is authorization. You need both.
- **Read the guard's source, don't trust its name.** If authorization runs through a helper (`assertTeamAdmin`, `requireOrgMember`) or an RLS policy, open it and confirm it checks the exact relationship you need. A helper that checks "is a member of the org" is not "is an admin of this team."
- **Multi-tenant apps:** every query must be scoped to the caller's tenant/org/team. One missing `WHERE company_id = ?` is a cross-tenant breach.
- **Privilege escalation:** can a normal user call an admin-only action? Is "admin" determined from the server-side session, or from something the client sent?

### 3. What is the worst malicious input?

Not malformed — *malicious*. Assume the caller is hostile and knows how the code works.

- Oversized (a 2 GB upload, a 10 MB JSON body, a 100,000-item array)
- Wrong tenant / someone else's id
- A valid token that belongs to a different user, or an expired one, replayed
- Numbers: negative, zero, `MAX_INT + 1`, a float where an int is expected
- Strings where structure is assumed: `../../etc/passwd`, `'; DROP`, `<script>`, `{{7*7}}`, a URL pointing at `169.254.169.254` or `localhost`
- Missing fields, extra fields (mass assignment — can the caller set `isAdmin: true`?)
- The same request sent 1,000 times in parallel (race conditions, double-spend)

For each plausible one: does the code reject it at the boundary, or does it flow through?

### 4. What happens when it fails?

- **Fails closed?** On an auth error, a validation error, a dependency being down — does it deny, or does it fall through to allowing the action?
- **What leaks in the error?** A stack trace, a file path, a SQL error string, an internal hostname, another user's data in a "helpful" message. The client should get a generic message and a status code, nothing else.
- **Partial failure:** if step 3 of 5 fails, is the system left in a safe state?

## The self-critique pass

After answering all four, go back over your own answers. A first pass can miss flaws when it accepts its own assumptions. Recheck the evidence for each conclusion. For each answer, ask:

- "I said this is fine because X — is X actually enforced in the code I'm looking at, or am I assuming it?"
- "Did I check the server code, or did I see a check in the client and stop?"
- "Would this hold if the caller never touches the UI and calls the endpoint directly with curl?"
- "Did I read the guard helper's source, or trust its name?"

Upgrade anything that doesn't survive that pass to a finding.

**For every target you mark `safe`, cite the specific line that makes it safe** (`file:line`) in the inventory verdict. "Looks fine" or "the framework handles it" without a line is not a verdict — go find the line, or downgrade it to a finding.

## STRIDE crosswalk

STRIDE is the standard checklist for what can go wrong at a boundary. The four questions cover it; use this if you want to be systematic in pre-launch mode.

| STRIDE category | Question it maps to | What to look for |
|---|---|---|
| **S**poofing | Q2 | Weak/absent authentication; trusting a client-supplied identity |
| **T**ampering | Q3 | Unvalidated input changing data or logic; client-supplied price/role/id trusted |
| **R**epudiation | Q4 | No audit trail for sensitive actions; can't tell who did what |
| **I**nformation disclosure | Q2, Q4 | IDOR, over-fetching, verbose errors, secrets in responses/logs |
| **D**enial of service | Q3 | No size/rate limits on expensive operations (report as Low–Medium for this audience) |
| **E**levation of privilege | Q2 | Normal user reaching admin actions; server trusting a client role claim |

Optional tool: **STRIDE-GPT** (open source, github.com/mrwadams/stride-gpt) generates a starting STRIDE checklist from a plain-English app description — useful to seed a pre-launch review. Not required.

## Worked examples

### Example A — the missing ownership check

```ts
export async function GET(req, { params }) {
  const session = await getSession();
  if (!session) return json({ error: "unauthorized" }, 401);
  const order = await db.order.findUnique({ where: { id: params.id } });
  return json(order);
}
```

- **Q1:** browser → server, server → database.
- **Q2:** anyone logged in. As whom? *Anyone.* `params.id` is caller-controlled and the query doesn't filter by owner. Change the id → read someone else's order. **IDOR, Critical.**
- **Q3:** worst input is simply *a valid order id that isn't mine* — no exotic payload needed.
- **Q4:** returns the row directly; if the id is bogus it returns `null` (minor info-leak on 200 vs 404, secondary).
- **Fix:** `where: { id: params.id, userId: session.user.id }`, return 404 on no match.

### Example B — the table with no RLS

```sql
create table public.profiles (
  id uuid primary key default gen_random_uuid(),
  user_id uuid references auth.users not null,
  email text not null, phone text, stripe_customer_id text
);
```

- **Q1:** browser → database. On Supabase, `public` tables are exposed through an auto-generated REST API reachable with the public `anon` key.
- **Q2:** with no `enable row level security` and no policies, *anyone with the anon key* (it's in the client bundle) can `select *` the whole table — every user, every tenant.
- **Q3 / Q4:** not even needed; the default state is "wide open."
- **Fix:** `alter table public.profiles enable row level security;` in the same migration + one policy per operation scoped to `auth.uid()` / the user's `company_id`. New table stays deny-all until policies exist.

### Example C — the safe change (what "no finding" looks like)

```ts
const user = await requireUser();                       // 401 if not signed in
await assertTeamAdmin(user.id, params.teamId);          // 403 unless admin of THIS team
const token = randomBytes(32).toString("base64url");    // CSPRNG, opaque
const supabase = await getUserClient();                 // user-scoped, RLS still applies
```

- **Q2:** authenticated *and* authorized per-resource, on the server. Changing `teamId` just yields a 403.
- **Q3:** token is opaque and generated with a CSPRNG — not hand-rolled.
- **Q4:** DB error returns a generic 500, no leak.
- **Verdict:** no finding. Resist inventing one.

## Pre-launch mode

Instead of walking each changed file, walk each trust boundary in the whole system:

1. From the data-flow note, list every boundary (each external entry point, each data store).
2. For each, run the four questions across *all* endpoints/queries that cross it, not just recent ones.
3. Pay special attention to: every table's RLS/policy state; every admin action's authz; every third-party webhook's signature check; every place a secret is read; every publicly reachable URL, bucket, or preview deployment.
