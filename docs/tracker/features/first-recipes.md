# First recipes: Hermes, Codex, OpenCode

- **Status:** Not started
- **Milestone:** M2
- **Plan:** [plans/v1.md](../../plans/v1.md)

Three different kinds of tool, to test the recipe format before writing more.

## Acceptance

- All three run from recipes on one server.
- Every path, port and image in them is checked against upstream docs.

## Evidence

- 2026-10-19 — upstream facts for all three tools gathered into [Sources/recipe-research.md](../../Sources/recipe-research.md) (one section per tool, plus the TRD section 16 answers). Covers image and tags, ports and health, data and config paths and formats, MCP, skills, channels, authentication, runtime config writes, resources and licence, with a link and checked date per source. Nothing was installed or executed; unverified items are marked UNVERIFIED.
- Open questions the research surfaced, still to decide before or while writing the recipes:
  - **Codex has no official container image**, so its recipe cannot pin an upstream digest the way Hermes and OpenCode can. The recipe format may need to express a build or install step; recorded here rather than decided.
  - **All three tools rewrite their own config** (`config.yaml`/`.env`, `config.toml`/`auth.json`, `auth.json`), so their managed config needs the TRD 7.3 `writable: true` copy-into-volume path.
  - **Hermes chat channels are environment-based** (token plus allow-list in `.env`), which the current `supports.channels` shape does not describe.
  - **No minimum resources are documented for OpenCode**; Hermes documents 2–4 GB recommended and Codex documents 4 GB minimum / 8 GB recommended.
- Not done yet: no recipe files written, nothing tested on a server.
