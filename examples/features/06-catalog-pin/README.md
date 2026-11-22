# Feature: Recipe Catalog Pinning and Overrides

This example shows how to pin recipe catalogs in `stack.yaml` using git references, how `stack.lock` locks exact revisions, and how to use the local `-recipes` override during development.

## Catalog Specification with `?ref=`

You can specify where recipes are fetched by configuring `inputs.recipes`:

- **Default (repository default branch):**
  ```yaml
  inputs:
    recipes: github:Haze-dev1/Olympus/recipes
  ```
- **Pinned to a release tag:**
  ```yaml
  inputs:
    recipes: github:Haze-dev1/Olympus/recipes?ref=v1.0.0
  ```
- **Pinned to a branch:**
  ```yaml
  inputs:
    recipes: github:Haze-dev1/Olympus/recipes?ref=development
  ```

Only one `ref` parameter is permitted (ADR 0040).

## What `stack.lock` Pins

When you run `olym check` or `olym update` against a remote catalog, Olympus creates or updates `stack.lock`. Diffs stay small and deterministic:

```json
{
  "version": 1,
  "inputs": {
    "recipes": {
      "url": "github:Haze-dev1/Olympus/recipes?ref=v1.0.0",
      "rev": "9d41b659c03b134d40237c87c88b49e29a8a7e02",
      "hash": "sha256-abc123..."
    }
  },
  "tools": {
    "hermes": {
      "image": "nousresearch/hermes-agent@sha256:e6d81ae52ebf6a43efa529701405509f20054ba818f179ddd748bed4f8af584c",
      "platforms": [
        "linux/amd64",
        "linux/arm64"
      ]
    }
  }
}
```

Key fields:
- `rev`: The exact 40-character commit SHA resolved from the git ref.
- `hash`: The canonical SHA-256 tree hash of the extracted recipe files (ADR 0023).
- `image`: The exact multi-architecture index digest (`@sha256:...`). Once locked, Olympus pulls strictly by digest, never by a moving tag (Rule A7).
- `platforms`: The CPU architectures supported by the image index (`linux/amd64`, `linux/arm64`). Server preflight checks this list against the target host before pulling.

Commit both `stack.yaml` and `stack.lock` to git to guarantee reproducible deployments across machines and over time.

## Updating Pins (`olym update`)

To update recipe revisions and container digests:

```sh
olym update -f stack.yaml
```

`update`:
1. Queries GitHub to resolve `?ref=` to the latest commit SHA.
2. Queries container registries for updated multi-architecture index digests.
3. Updates `stack.lock` atomically.

`update` does not touch `stack.yaml` and never restarts containers on the server until you subsequently run `olym apply`.

## Local Recipe Override (`-recipes`)

During recipe development, or when testing offline without network access, use the `-recipes` flag to point to a local directory:

```sh
olym check -f stack.yaml -recipes ../../../recipes
olym build -f stack.yaml -recipes ../../../recipes -o /tmp/build-06
```

When `-recipes` is provided:
- Olympus prints: `Local -recipes override: skipping the lock's catalog pin.`
- GitHub network requests are skipped.
- Catalog extraction limits (10,000 tar entries, 272 MiB expanded data) are skipped.
- The build manifest records a local content hash instead of a remote git revision.
- Note: `olym update -recipes` is refused because `update` is strictly intended for remote sources.

## What You Need

Copy `.env.example` to `.env`:

```sh
cp .env.example .env
```

Set the required environment variables:
- `ANTHROPIC_API_KEY`: Anthropic API key for Hermes.

## Commands

Validate using the local recipe override:

```sh
olym check -f stack.yaml -recipes ../../../recipes
```

Build the local generation:

```sh
olym build -f stack.yaml -recipes ../../../recipes -o /tmp/build-06
```

Preview changes against your server:

```sh
olym plan -f stack.yaml -recipes ../../../recipes
```

Deploy:

```sh
olym apply -f stack.yaml -recipes ../../../recipes
```

Check status:

```sh
olym status -f stack.yaml
```

## What You Should See

- `olym check` reports:
  ```
  Local -recipes override: skipping the lock's catalog pin.
  Stack is valid.
  ```
- `olym build` outputs:
  ```
  Local -recipes override: skipping the lock's catalog pin.
  Built sha256-...
  ```
- The generated manifest in `/tmp/build-06/store/manifest.json` uses the locked tool configuration and pinned recipe settings.
