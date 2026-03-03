# 133 — Internal Migration (pkg/ → internal/)

## Objective

Move all library code from `pkg/` to `internal/` to make clear Ze BGP is a binary, not a library. Consolidate test infrastructure under `internal/test/`.

## Decisions

- `pkg/pool/` (double-buffer design) moved to `internal/pool/` and the old `internal/pool/` (single-buffer with metrics) renamed to `internal/delete-pool/` to preserve scheduler/metrics ideas for potential future merge.
- `pkg/editor` grouped under `internal/config/editor` (not `internal/editor/`) since the editor is a config concern.
- `pkg/testpeer` → `internal/test/peer`, `pkg/testsyslog` → `internal/test/syslog`, `test/functional` → `internal/test/runner`, `test/ciformat` → `internal/test/ci` — all test infrastructure consolidated under one path.

## Patterns

None beyond the import path updates.

## Gotchas

- Package renames (`testpeer→peer`, `functional→runner`) caused variable shadowing in callers: local variable `peer` shadowed the package import `peer`, causing compile errors. Watch for this when renaming packages to short common nouns.
- Bulk sed replacements on `.md` files can corrupt spec files if they contain "before/after" state showing the old format — the old format in the spec gets replaced too.

## Files

- ~200+ Go files with updated imports
- `internal/test/peer/`, `internal/test/syslog/`, `internal/test/runner/`, `internal/test/ci/` — new locations
- `pkg/` — deleted
