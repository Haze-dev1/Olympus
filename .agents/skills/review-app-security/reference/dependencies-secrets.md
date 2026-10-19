# Step 4 — Dependencies, secrets, exposure

This skill does **not** run scans. Dedicated tools do it faster, more completely, and with fewer false positives. This step decides *which* tool the user should run, based on what changed, and tells them what to do with the report.

Produce a short block the user can act on. Template at the bottom.

## What to check for, and which tool

### New or changed dependencies

Triggered by: any change to `package.json`, `requirements.txt`, `pyproject.toml`, `Gemfile`, `go.mod`, a lockfile, or an `import` of a package not used before.

- **Known vulnerabilities (CVEs):**
  - `npm audit` — one command, JS/Node, built in. Recommend fixes separately; never run `npm audit fix` during a read-only review. Python dependencies need a Python-aware scanner such as pip-audit, if already available.
  - **GitHub Dependabot** — turn it on in the repo's Security settings; it opens PRs with fixes automatically. Works for most ecosystems.
  - **Snyk** (free tier, no card) — broader than `npm audit`, covers more languages, gives fix guidance.
- **Is the package real and maintained (slopsquatting check)** — AI tools sometimes suggest packages that don't exist, which attackers then register with malware:
  - **Socket** (GitHub app, free for small projects) — automates package-reputation and supply-chain checks on every PR.
  - Manual: open the package's npmjs.com / pypi.org page. Real download numbers *and* a linked, active source repo. Read the name character by character for lookalikes (`dateutils` vs `dateutil`, hyphen vs underscore). If download counts or the repo link are missing, don't install it.

### Hardcoded secrets

Triggered by: always (secrets get committed by accident constantly), especially if the change adds config, API clients, or test fixtures.

- **gitleaks** or **git-secrets** — scan the working tree *and* git history for key patterns. History matters: a secret removed in a later commit is still in the repo.
- **Semgrep** has secret-detection rules if the user already runs it.
- GitHub **secret scanning** (Security settings) — free for public repos, alerts on known provider key formats.
- If anything is found: **rotate it first** (assume it's compromised), then move it to an env var, then scrub history if needed (`git filter-repo`) — rotation is what actually protects you.

### Live-site / exposure

Triggered by: new public endpoints, changed CORS or security headers, a new deployment, or pre-launch mode.

- **Mozilla HTTP Observatory** (observatory.mozilla.org) — paste the URL, no signup. Grades headers and config F–A+ with specific fixes.
- **OWASP ZAP** — "Quick Start → Automated Scan" against your *own* URL only. Closest thing to an automated pentest that a non-specialist can run.
- Manual checks: view-source / devtools Network tab on the deployed site — any `sk_`, `key`, `token`, `password` strings in the JS bundles? Is a staging/preview URL publicly reachable and indexed? Is an admin route protected?

### The one judgment call that stays here

A scanner can tell you an endpoint or storage bucket is publicly reachable. It can't tell you whether that's *wrong*. If this change made something public — an API route, an S3/Supabase bucket, a preview deploy — decide whether it's supposed to be, and if not, that's a **finding for Step 5**, not something to defer to a tool.

## Output template

```
## Run these scans and paste the reports back

Because this change touched <deps / config / a live URL>:

- [ ] `npm audit`  (or: enable Dependabot in repo Settings → Security)
- [ ] gitleaks  — `docker run -v $(pwd):/repo zricethezav/gitleaks detect -s /repo`
- [ ] Mozilla HTTP Observatory — https://observatory.mozilla.org, enter <your URL>

New packages this change added: <list>. Verified real + maintained: <yes/no per package>.

Once you have the reports, start a fresh session, paste them in, and ask for fixes
one report at a time.
```

Omit lines that don't apply. If the change touched none of deps / secrets-prone files / config / URLs, say so and skip the step.
