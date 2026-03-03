# 041 — Format-Based Migration

## Objective

Replace version-based config migration (v2→v3 constants) with named, semantic transformations that support dry-run, visibility into what was applied, and atomic operation.

## Decisions

- Transformations are named semantically (`neighbor->peer`, `static->announce`) rather than numbered (`v2→v3`) — names survive future restructuring.
- Detect-then-apply: each `Transformation` has independent `Detect(tree) bool` and `Apply(tree) (tree, error)` functions. Detect is idempotent — skips already-migrated configs.
- Atomicity: `Migrate()` works on a clone; the original tree is unchanged unless ALL transformations succeed. Partial success returns error with nil result.
- `migrateAPIBlocks()` was deliberately kept as one transformation (not split) because the API block migration is tightly coupled internally.
- Removed: `ConfigVersion` type, `DetectVersion()`, `MigrateV2ToV3()`, `hasV2Patterns()` — all replaced by the transformation registry.

## Patterns

- Transformation order matters: structural renames (Phase 1: `neighbor→peer`, glob patterns, template renames) must run before content transforms (Phase 2: `static→announce`, `api→new-format`).
- `DryRun()` runs transformations on a clone to validate they would succeed, returns analysis even for would-fail cases (returns analysis, not error).

## Gotchas

None.

## Files

- `internal/config/migration/migrate.go` — `Transformation`, `Migrate()`, `DryRun()`, `NeedsMigration()`
- `internal/config/migration/detect.go` — extracted detection functions
- `cmd/ze/bgp/config_migrate.go` — added `--dry-run`, `--list` flags
