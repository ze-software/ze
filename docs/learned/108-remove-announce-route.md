# 108 — Remove "announce route" References (Cleanup)

## Objective

Clean up all remaining `announce route` string references from comments, test data, and documentation after the command handler was removed in spec 104.

## Decisions

- Mechanical cleanup, no design decisions.

## Patterns

- `.ci` file `:cmd:` lines are documentation only — not executed by the test runner. Updating them to `update text` syntax improves accuracy but does not affect test outcomes.
- Historical summaries in `docs/learned/` are NOT modified — they are institutional memory.
- External RFCs (`rfc/rfc8195.txt`) not modified — "announce route" there is BGP terminology, not Ze API.

## Gotchas

None.

## Files

- ~30 `.ci` files in `test/data/encode/` and `test/data/plugin/` — `:cmd:` lines updated
- `internal/plugin/route.go`, `commit_test.go`, `types.go` — dead comments removed
- ~15 documentation files — active docs updated, historical specs untouched
