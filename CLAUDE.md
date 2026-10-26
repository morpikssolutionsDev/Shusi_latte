# Sushi Latte

Prototype and showcase of a single-outlet HORECA platform: one Go API with SQLite, plus three React apps (Client Menu, Staff PWA, Product Display), running on one local host. Repository layout: [TRD §3](docs/TRD.md#3-repository-structure).

## Language

[GLOSSARY.md](GLOSSARY.md) is the vocabulary for every type, table, endpoint, event and UI string. Use the bold term; its _Avoid_ line lists retired synonyms. A new domain concept goes into the glossary in the same change that introduces it.

## Spec

| Read when you need… | Document |
|---|---|
| business rules, permissions, requirements (`BR-*`) | [docs/BRD.md](docs/BRD.md) |
| data model, API, realtime, auth, jobs, printing (`TR-*`) | [docs/TRD.md](docs/TRD.md) |
| every flow branch and state machine | [docs/OVERVIEW-AND-FLOWS.md](docs/OVERVIEW-AND-FLOWS.md) |
| the reason behind a hard-to-reverse choice | [docs/adr/](docs/adr/) |
| demo mode, seed data, which Table shows which scenario | [docs/DEMO-AND-SEED.md](docs/DEMO-AND-SEED.md) |
| any UI work: tokens, colour, type, motion, components, copy | [docs/design/DESIGN-PHILOSOPHY.md](docs/design/DESIGN-PHILOSOPHY.md), then the guide it points to |
| logo or photos | [docs/brand/](docs/brand/), [docs/images/README.md](docs/images/README.md) |

The spec is the source of truth. When code and spec disagree, or a behaviour is not covered, ask the user, record the answer in the BRD/TRD (plus an ADR when hard to reverse), then build it.

## Rules

- Tests and PR descriptions cite the `BR-*` / `TR-*` IDs they implement.
- Totals, status transitions, permissions and PIN Overrides are decided in Go inside one transaction; apps render what the API returns.
- Realtime events are published on the in-process event bus after `COMMIT`.
- UI code uses semantic, surface or component tokens. Plum is reserved for Dietary Warnings; logo Salmon `#F2885B` is decoration, Salmon Deep is the text and action colour.
- `docs/design/tokens/*.json`, the tables in `docs/design/COLOR.md` and `docs/design/styleguide.html` are generated: edit `tools/design/*.py`, then run `python tools/design/build.py`.
- Demo helpers live behind `DEMO_MODE`; seed data goes through the domain packages so every seeded record obeys the same rules as live data.
- `project goals.md` is the owner's original brief and stays as written.
