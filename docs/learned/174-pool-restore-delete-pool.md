# 174 — Pool Double-Buffer Non-Blocking Compaction

## Objective

Restore the delete-pool implementation, add double-buffer non-blocking compaction, and extend the handle layout to support buffer selection.

## Decisions

- Handle layout changed to hybrid: `bufferBit(1) | poolIdx(5) | flags(2) | slot(24)` — max pools reduced from 63 to 31 (sufficient for BGP) to gain the buffer-selection bit needed for double-buffer compaction.
- NLRI pool initial size is 256KB not 6MB — spec estimated 6MB for full DFZ but 256KB is the initial allocation; pool grows as needed.
- `InvalidHandle = 0xFFFFFFFF` (poolIdx=31, the reserved value) — detecting invalid by pool index already excluded by 5-bit range.
- `Get()` errors logged and skipped; `Release()` errors ignored — corruption is logged for detection, cleanup paths are best-effort.

## Patterns

- Double-buffer compaction: allocate new buffer, migrate slots in batches, both old and new handles valid during migration, free old buffer only when its refCount reaches zero.
- `MigrateBatch(N)` returns bool (done/not done) — scheduler calls it in ticks, pausing when activity is detected.

## Gotchas

- Architecture doc described MSB-only handle design that was never implemented — doc updated to match the actual hybrid layout after implementation.
- Index corruption in `nlriset.go Remove()`: when Get() fails for the last element during swap-remove, the index map has a stale entry pointing to the wrong slot. Fix: iterate the index to find and delete the stale key.

## Files

- `internal/pool/handle.go` — hybrid handle layout: bufferBit + poolIdx(5) + flags + slot
- `internal/pool/pool.go` — double-buffer, AddRef, GetBySlot, ReleaseBySlot, StartCompaction, MigrateBatch
- `internal/pool/attributes.go` — global pools: Attributes (idx=0, 1MB), NLRI (idx=1, 256KB)
- `internal/plugin/rib/storage/nlriset.go` — fixed swap-remove index corruption bug
