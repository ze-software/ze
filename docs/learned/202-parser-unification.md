# 202 — Parser Unification

## Objective

Extract shared value parsers into `internal/parse/` to eliminate duplication across config, API, and encode paths. Phase 1 covers Origin only.

## Decisions

- Two accepted forms for Origin: empty string → IGP (config compatibility) and `"?"` → INCOMPLETE (API compatibility) — both preserved in the unified parser rather than normalizing to one input form.
- Phases 2-6 (AS_PATH, NEXT_HOP, etc.) deferred — Phase 1 proves the pattern with the simplest case.

## Patterns

- `internal/parse/` as the shared parser package; all three call sites (config, API, encode) import from there.

## Gotchas

- None.

## Files

- `internal/parse/origin.go` — unified Origin parser
- `internal/component/config/` — updated to use parse.Origin
- `internal/plugins/bgp/encode/` — updated to use parse.Origin
