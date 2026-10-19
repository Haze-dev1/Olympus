# Docs

Start with [PRD.md](PRD.md) for what Olympus is for, then [TRD.md](TRD.md) for how it is built. Both are standalone.

## Layout

| Path | Holds |
| --- | --- |
| [PRD.md](PRD.md) | Product requirements: problem, users, goals, non-goals, requirements, release criteria, risks |
| [TRD.md](TRD.md) | Technical requirements: architecture, formats, build and activation, server layout, security, testing, release |
| [userflow.md](userflow.md) | How a user works with the tool, from first setup to teardown |
| [ADR.md](ADR.md) | Every architecture decision in one file, with rejected alternatives, plus the template |
| [architecture/](architecture/) | [design.md](architecture/design.md), the [diagram](architecture/architecture.png) and the [glossary](architecture/glossary.md) |
| [specs/](specs/) | Exact formats: [stack file](specs/stack-file.md) (and lock file), [recipe](specs/recipe.md) |
| [edgecases/](edgecases/) | Cases the code must handle and the intended behaviour, one file per area ([apply](edgecases/apply.md)) |
| [plans/](plans/) | Implementation plans, one per release ([v1](plans/v1.md)) |
| [prototypes/](prototypes/) | Draft example files: a stack file, `.env.example`, a Hermes recipe |
| [tracker/](tracker/) | [progress.json](tracker/progress.json) for milestone and feature status, and one file per feature in [features/](tracker/features/) |
| [Sources/](Sources/) | External research: [landscape.md](Sources/landscape.md) on existing tools and what was checked |

The design doc also has a live version at https://claude.ai/code/artifact/ceba9b3f-01ae-4774-bcf0-740b16516722. The copy in this repo is current and the one to edit; the live version predates ADRs 0013–0016.

## Keeping it up to date

- **PRD and TRD stay standalone and current.** A change to requirements updates `PRD.md`; a change to design, formats or behaviour updates `TRD.md`, alongside any ADR.
- **A new architecture decision** gets a section at the end of [ADR.md](ADR.md), using its [template](ADR.md#template), before or with the change. Reversed decisions get a new ADR; old ones are marked "Superseded by", never deleted.
- **A new feature idea** goes in the Future table in [architecture/design.md](architecture/design.md#future-everything-we-can-add), not into v1 scope.
- **A feature starting or finishing** updates its file in `tracker/features/` and its status in `tracker/progress.json` (`not_started`, `in_progress`, `done`), plus the `updated` date.
- **A format change** updates the matching file in `specs/` and, if it affects them, `prototypes/`.
- **A new failure case** found while building goes in the matching `edgecases/` file.
- **New external research** goes in `Sources/`, with an as-of date and links.
- Dates are absolute (YYYY-MM-DD).
