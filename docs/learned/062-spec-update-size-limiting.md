# 062 — UPDATE Size Limiting

## Objective

Enforce consistent UPDATE message size limits (4096 standard RFC 4271, 65535 extended RFC 8654) across build and forward paths, fixing a data loss bug where `sendInitialRoutes` silently skipped oversized UPDATEs.

## Decisions

- Two complementary approaches: Option A (size-aware builder `BuildGroupedUnicastWithLimit`) for proactive limiting; Option B (`SplitUpdate`) for reactive post-build splitting in forwarding/replay paths
- Both are needed: builders for efficient fresh construction; split utility for already-built UPDATEs received from peers
- Existing `ChunkNLRI` is IPv4-only (no ADD-PATH, no labeled unicast, no VPN, no MP_REACH_NLRI splitting) — documented as limitation, new `SplitUpdate` handles MP families
- Attributes-only size check: if attributes alone exceed maxSize, return `ErrAttributesTooLarge` (cannot be fixed by splitting NLRI); if single NLRI exceeds available space, return `ErrNLRITooLarge`
- Size limiting enforced at the RIB→send boundary, not deep in builders

## Patterns

- Wire cache preservation: `u.PathAttributes` (raw wire bytes) reused directly in all split chunks — zero-copy for attributes
- For MP_REACH_NLRI: the attribute must be rebuilt (NLRIs are inside it), but other attributes can be preserved

## Gotchas

- The prior `sendInitialRoutes` silently skipped routes that exceeded max size — routes were lost with no error or log
- Mixed announce+withdraw in a single UPDATE must be split into separate announce and withdraw UPDATEs when oversized
- UPDATE overhead is always at least 23 bytes (Header 19 + WithdrawnLen 2 + AttrsLen 2)

## Files

- `internal/bgp/message/update_split.go` — `SplitUpdate()`, `SplitMPReachNLRI()`
- `internal/bgp/message/update_build.go` — `BuildGroupedUnicastWithLimit()`
- `internal/reactor/peer.go` — replace skip with split in `sendInitialRoutes()`
