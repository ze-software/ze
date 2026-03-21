# 332 — Pool Simplify: Handle Flags Removal and Compaction Wiring

## Objective

Remove unused 2-bit Flags field from the pool Handle (designed for NLRI pooling that never happened) and wire the existing compaction Scheduler into the RIB plugin lifecycle so buffer memory is actually reclaimed.

## Decisions

- Removed Flags bits by expanding Slot from 24 to 26 bits — no information lost, 67M slots is ample.
- `AllPools()` export from `pool/attributes.go` is the minimal coupling: scheduler receives a pool slice, does not import RIB internals.
- `OnStarted(ctx)` is the correct lifecycle hook for the scheduler goroutine: runs after 5-stage startup (Socket A is free), context is tied to plugin lifetime, goroutine exits cleanly on cancel.
- Kept `compaction.go` as a thin wiring file (`runCompaction(ctx, pools)`) — no logic, just constructs and launches the already-tested Scheduler.

## Patterns

- When infrastructure exists but is never started, prefer wiring over rewriting. The Scheduler and MigrateBatch were already correct and tested.
- `OnStarted` → goroutine with plugin context is the standard pattern for background work in RIB-style plugins.

## Gotchas

- Handle flags (`HasPathID`, `WithFlags`) were always passed as 0 in production — both `Intern()` and `MigrateBatch()` hardcoded `flags=0`. The API change had zero runtime impact.
- Pool buffer is append-only for writes; slot reuse (`freeSlots`) recycles slot indices but never reclaims the underlying buffer gaps — compaction is the only mechanism that does so.

## Files

- `internal/component/bgp/attrpool/handle.go` — Flags removed, Slot expanded to 26 bits
- `internal/component/bgp/plugins/rib/compaction.go` — thin wiring (new file)
- `internal/component/bgp/plugins/rib/pool/attributes.go` — added `AllPools()`
- `docs/architecture/pool-architecture.md` — handle layout and scheduler wiring updated
