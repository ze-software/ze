# 314 — Chaos Dashboard UX Overhaul (Umbrella)

## Objective

Coordinate 10 prioritized UX improvements to the ze-chaos web dashboard, capturing shared architectural constraints and delegating each improvement to a child spec.

## Decisions

- Umbrella-only: no code in this spec. All research and constraints captured here; implementation in child specs.
- Phase order (A: independent high-impact → B: layout changes → C: grid-dependent → D: new state) chosen to minimize rework.
- All rendering server-side Go HTML — no JS framework; HTMX + SSE only.
- `viz.go` already over 1000 lines: any new viz feature must go in a separate file.

## Patterns

- Umbrella spec captures shared constraints (SSE event types, CSS custom properties, active set behavior, file size limits) that each child would otherwise have to re-research.
- Child specs reference the umbrella's constraint annotations (`→ Constraint:`) directly in their Required Reading.

## Gotchas

- No `.ci` functional test infrastructure exists for the chaos simulator — all child specs skipped functional tests and relied on unit tests only.
- The umbrella's Implementation Summary section was left as "(pending)" — completion was tracked per child spec, not at umbrella level.

## Files

- `docs/plan/spec-chaos-ux-{1..10}-*.md` — 10 child specs (all completed)
- `docs/architecture/chaos-web-dashboard.md` — to be updated after all children complete
