# The data-flow note

A one-page file that lives in the repo (suggested path: `SECURITY-DATA-FLOW.md` at the root). It's the stable reference the per-change security checks need — what data exists, what's sensitive, who the users are. Without it, every review re-derives this from scratch and misses things.

- The **security rules block in `CLAUDE.md` / `AGENTS.md`** can tell the AI to keep this file updated as the app changes.
- **Step 0 of this skill** reads it.
- **Step 1** uses it to know which changes touch sensitive data and which user/tenant boundaries matter.

Re-model (revisit the whole file) on **trigger events**, not a schedule:
- a new external integration or third-party API
- new auth-related code (login method, roles, permissions)
- a new class of data stored (payments, health, location, documents)
- a new user role or tenant type

## Template

```markdown
# Security data-flow note

_Last reviewed: YYYY-MM-DD_

## What this app is
<one or two sentences: what it does, who uses it, where it's deployed>

## Users and roles
- <role>: <what they can do>
- <role>: <what they can do>
Multi-tenant? <yes/no — if yes, what's the tenant: company / team / workspace>

## Data we store
| Data | Sensitivity | Where it lives | Who should see it |
|---|---|---|---|
| e.g. user email | PII | `profiles` table | the user, their org admins |
| e.g. Stripe customer id | payment | `profiles` table | server only |
| e.g. uploaded documents | user content | storage bucket | the owner |
| e.g. session tokens | secret | cookie / `sessions` table | server only |

## Trust boundaries
- browser → API: <where requests come in — list the main entry points>
- API → database: <ORM / direct / Supabase client — is RLS on?>
- webhooks in: <which providers, are signatures verified>
- app → third parties: <what we call, what we send them>

## Auth / authz helpers
- <helper name, e.g. `assertTeamAdmin(userId, teamId)`>: <file path> — <what it actually checks, in one line>
- <RLS policies>: <migration / file> — <scoped to what>
(A reviewer reads these instead of trusting the names.)

## Secrets and how they're stored
- <secret name>: <env var / secrets manager> — server-only? <yes/no>

## Known public surface
- <public endpoints, buckets, pages that are intentionally unauthenticated>
- <preview / staging URLs — are they protected?>
```

Keep it short. A page that's painful to maintain won't be maintained, and a stale note is worse than none.
