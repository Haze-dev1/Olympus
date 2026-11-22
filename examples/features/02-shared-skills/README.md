# Feature: Shared Skills (`skills: all` vs List)

This example shows how to define skills once in your repository and distribute them across multiple AI agents.

## How Skills Work in Olympus

A skill is a directory containing a `SKILL.md` file (with YAML frontmatter providing `name` and `description`). You declare reusable skill directories under `skills:` at the root of `stack.yaml`:

```yaml
skills:
  - ./skills/deploy-checklist
  - ./skills/code-review
```

Each agent can then choose which skills to consume:
- `skills: all`: Grants the agent access to every skill declared in the stack. In this example, Hermes gets both `deploy-checklist` and `code-review`.
- A specific list: Grants the agent only the selected skills. In this example, Codex gets only `deploy-checklist`.

## Delivery Mechanisms (`copy` vs `mount`)

Different tools store and access skills differently. Recipes declare their supported skill directory and delivery mode in `supports.skills`:

- **Copy Mode (`mode: copy`):**
  - Used by tools like Hermes (`/opt/data/skills`).
  - At startup, an Olympus init service copies the skill files into the tool's volume.
  - Olympus tracks copied skills with an ownership ledger (`.olympus-skills.json`). If you remove a skill from `stack.yaml` on a later deploy, Olympus automatically prunes the removed files while preserving any files the agent created itself.
- **Mount Mode (`mode: mount`):**
  - Used by tools like Codex (`/root/.agents/skills`) and Claude Code (`/root/.claude/skills`).
  - Olympus mounts the skill files read-only directly from the content-addressed store (`/var/lib/olympus/store/<hash>/files/...`) into the container.

## What You Need

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Set the required environment variables:
- `ANTHROPIC_API_KEY`: API key for Hermes.
- `CODEX_API_KEY`: API key for Codex.

## Commands

Validate the stack offline:

```sh
olym check -f stack.yaml
```

Build the local generation to inspect skill delivery:

```sh
olym build -f stack.yaml -o /tmp/build-02
```

Inspect the build artifacts in `/tmp/build-02/store/`:
- For Hermes: Notice the init service script copying skills and recording the ledger.
- For Codex: Notice the read-only volume mounts pointing to the store files.

Preview server changes:

```sh
olym plan -f stack.yaml
```

Deploy to your server:

```sh
olym apply -f stack.yaml
```

Check runtime status:

```sh
olym status -f stack.yaml
```

## What You Should See

- `olym check` confirms:
  ```
  Stack is valid.
  ```
- `olym build` succeeds and renders:
  - Both `deploy-checklist` and `code-review` in the store for Hermes.
  - Only `deploy-checklist` in the store for Codex.
- `olym status` shows both containers running and healthy.
