# Step 2 — Extended checklist (the long tail)

Scan this in **pre-launch mode**, or in diff mode when the change touches: outbound network requests, file uploads/downloads, template rendering, deserialization, background jobs, webhooks, or authentication flows.

Same format: **what to look for** → *why* → fix.

### Network & server-side requests

**E1. SSRF — server fetches a user-supplied URL.** `A10 / CWE-918`
Look for: `fetch(userUrl)`, image/webhook/import features that take a URL, PDF/screenshot generators. → *Attacker points it at `http://169.254.169.254/` (cloud metadata → credentials) or internal services.* → Allowlist hosts/schemes; block private IP ranges and redirects to them; resolve DNS and re-check.

**E2. Webhook / callback authenticity.** `A08 / CWE-345`
Look for: a webhook handler (Stripe, GitHub, Twilio, etc.) that acts on the payload without verifying the signature/secret. → *Anyone who finds the URL can forge events (fake "payment succeeded").* → Verify the provider's signature against the raw body before doing anything.

**E3. Open redirect.** `CWE-601`
Look for: `redirect(req.query.next)` / `returnTo` / `callbackUrl` without validation. → *Phishing: your domain redirects to the attacker's; also used to steal OAuth codes.* → Allowlist paths; only permit relative URLs.

### Files & serialization

**E4. File upload handling.** `A04 / CWE-434`
Look for: trusting the file extension or `Content-Type`; storing uploads in a web-served directory; no size limit; original filename used on disk. → *Uploaded `.php`/`.svg`/HTML → stored XSS or code execution; zip bombs.* → Validate by magic bytes; store in a dedicated bucket/service with random keys; cap size; serve with `Content-Disposition: attachment` and a correct type.

**E5. Insecure deserialization.** `A08 / CWE-502`
Look for: `pickle.loads`, `yaml.load` (non-safe), PHP `unserialize`, Java/`.NET` binary deserialization, `node-serialize` on untrusted data. → *Remote code execution.* → `yaml.safe_load`, JSON only, or a schema-validated format.

**E6. XXE — XML parsing.** `A05 / CWE-611`
Look for: an XML parser handling user-supplied XML with external entities / DOCTYPE enabled. → *File read, SSRF.* → Disable DTDs and external entity resolution.

### Templating & dynamic code

**E7. Server-side template injection.** `CWE-1336`
Look for: user input concatenated into a template string that's then rendered (Jinja, Handlebars, EJS, Thymeleaf). → *RCE or data disclosure.* → Pass user data as template *variables*, never build the template from it.

**E8. Prototype pollution.** `CWE-1321`
Look for: deep-merge / `Object.assign` recursion / `lodash.merge` on untrusted JSON; setting keys from user input without filtering `__proto__` / `constructor` / `prototype`. → *Corrupts app-wide object behavior; can escalate to RCE.* → Use `Map`, `Object.create(null)`, or a library that guards these keys; validate keys.

### Application logic

**E9. Mass assignment / over-posting.** `A08 / CWE-915`
Look for: `db.user.update({ data: req.body })` or spreading the whole body into a create/update. → *Caller adds `role: "admin"`, `credits: 999999`, `emailVerified: true`.* → Pick allowed fields explicitly.

**E10. Race conditions on limited resources.** `CWE-362`
Look for: check-then-act on balances, stock, invite seats, one-time tokens, without a transaction or lock. → *Parallel requests double-spend / redeem a coupon twice.* → Atomic DB operations, unique constraints, `SELECT ... FOR UPDATE`, or idempotency keys.

**E11. Missing rate / size limits on expensive or sensitive endpoints.** `A04 / CWE-770`
Look for: login, password reset, OTP, search, export, file upload, LLM/API-proxy endpoints with no throttling or body-size cap. → *Credential stuffing, brute force, cost blowout, DoS.* → Rate-limit per IP/user; cap body size; add exponential backoff on auth. *(Report as Medium for this audience — Low if the endpoint isn't auth/cost sensitive.)*

**E12. IDOR via indirect reference in nested data.** `A01`
Look for: an id deep in a request (line-item id, attachment id, comment id) that's used to load a child record without re-checking the parent's owner. Also: an ORM read where the top-level `where` is owner-scoped but an `include: { comments: true }` / `.populate()` / a join pulls child rows that were never scoped — the ownership filter only applied to the parent. → *Same as checklist #1 but easier to miss.* → Verify the whole chain belongs to the caller; scope the nested query too, or filter the children after loading.

### Auth flows

**E13. Session fixation / missing rotation.** `A07 / CWE-384`
Look for: session id not regenerated on login / privilege change. → *Attacker fixes a victim's session then rides it.* → Regenerate the session on every auth state change.

**E14. Password reset & email verification token hygiene.** `A07`
Look for: reset tokens that don't expire, aren't single-use, aren't tied to the user, or are returned in the response; account enumeration via different responses for "email exists" vs not. → *Account takeover.* → Short-lived, single-use, random, server-checked tokens; identical responses regardless of whether the account exists.

**E15. JWT validation.** `A07 / CWE-347`
Look for: `algorithms` not pinned (accepts `none` or HS/RS confusion); no expiry check; secret weak or shared with the client; sensitive data in the (readable) payload. → *Forged tokens.* → Pin the algorithm, verify signature + `exp` + `iss`/`aud`, keep the secret server-side.
