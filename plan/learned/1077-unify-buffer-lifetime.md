# 1077 -- unify-buffer-lifetime

## Context
Ze holds route/attribute bytes past a call boundary through four separate, incompatible
lifetime contracts (WireUpdate.Snapshot eager copy, attrpool refcounted Handle, ribForwardHandle
lazy copy, redistevents borrow-only batch). All four were enforced by comment only and failed
SILENTLY (recycled / another-route's / freed / zeroed-plausible bytes) rather than crashing. The
goal was to decide whether they are true duplication or distinct layers, then close the enforcement
gaps and give them one vocabulary -- without changing any release-build behavior or cost.

## Decisions
- Keep the four contracts SEPARATE, unify enforcement + vocabulary, over merging into one type:
  their copy semantics are mutually exclusive (eager / never / lazy / none); no single type can be
  both never-copy (attrpool dedup) and copy-on-retain (Snapshot). Merging would also couple the
  server-event layer to the RIB-forward layer.
- attrpool ABA guard via debug-build "do not reuse freed slots" (`slotReuseEnabled` const in the
  existing validate_debug/release split), over a 64-bit generation Handle: the Handle is fully
  packed (1+5+26 bits, 26 = 4 shardID + 22 slot) with no spare bit, so an always-on generation
  needs a wider Handle doubling footprint across millions of routes. No-reuse reaches the same
  detection (a stale handle hits a still-dead slot → existing ErrSlotDead) with zero ABI change.
- One `internal/core/memguard` poison primitive (`Poison`/`IsPoisoned`/`const Enabled`, build-tag
  split) over four ad-hoc zero/no-op patterns. Callers gate on `if memguard.Enabled` so release
  builds dead-code-eliminate the guard, the poison, and the slice-header argument.
- redistevents uses a struct SENTINEL, not `memguard.Poison`, over byte-poisoning the batch:
  `RouteChangeEntry` embeds `netip.Addr`/`Prefix` whose internal `z` pointer would become a
  fabricated pointer under byte-poison and crash the GC. Scalar fields get 0xDEADBEEF; netip fields
  stay zero (nil pointer, GC-safe). Still gated on `memguard.Enabled`, still references the vocabulary.

## Consequences
- The enforcement is debug-only (test/chaos), NOT a production guard. Production ABA enforcement for
  attrpool would still require the 64-bit Handle widening -- deliberately deferred on memory cost.
- New shared vocabulary in `docs/architecture/memory/lifetime-contracts.md` (Boundary / Borrow /
  Retain / Own) with a `<!-- source: -->` anchor per contract; the four contracts' doc-comments link it.
- `go test -tags debug` now exercises the enforcement. Because slot reuse is off in debug, the
  attrpool slot table and free list grow unbounded in long debug/chaos runs (R-3, acceptable for
  bounded test runs; a bounded free-slot ring is the fallback if a debug chaos run OOMs).
- The `debug` build tag is now used outside attrpool (memguard + call sites). Adding a new poison
  site is one `if memguard.Enabled { memguard.Poison(...) }` call -- no new type, field, or registry.

## Gotchas
- `go test -tags debug ./...` (bare) FALSELY fails in bgp/config, config/cli, doctor, plugin/all,
  dnsserver -- it omits the project registration tags. The correct gate is
  `go test -tags "debug ze_core $ZE_FEATURES" ./...` (GO_TEST_TAGS = ze_core $ZE_FEATURES). Those
  failures fail identically WITHOUT the debug tag, proving they are a missing-tag artifact, not the
  change. Under proper tags every package passes in both builds.
- Three existing tests assert release-only behavior the debug enforcement intentionally changes;
  made build-aware, NOT the 64-bit fallback: `TestInternReuseDuringCompactionKeepsData` and
  `TestSlotReuseStaleIndexEntry` hard-assert slot reuse (skip in debug via `if !slotReuseEnabled`,
  annotated `// test-relax:`); `TestForwardHandleBytesLazyCopy` tail and
  `TestRouteChangeBatchPoolResetsOriginAS` branch on `memguard.Enabled`.
- `IsPoisoned` on a SUB-slice at a non-4-aligned offset reads the repeating pattern out of phase and
  returns false even though the bytes are poison. Check `IsPoisoned` on the exact poisoned region
  (the whole recycled slot), not an arbitrary sub-slice (contract-A test).
- The `if memguard.Enabled` guard (not a bare `memguard.Poison(slice)` call) is required wherever the
  argument is a slice expression, so the slice header + bounds check are elided in release (AC-5).

## Files
- NEW `internal/core/memguard/{memguard,poison_debug,poison_release,memguard_test}.go`
- NEW `docs/architecture/memory/lifetime-contracts.md`
- `internal/component/bgp/attrpool/{pool,validate_debug,validate_release}.go` (+ debug_test, handle_test, pool_test, compaction_test)
- `internal/component/bgp/plugins/rib/forward_handle.go` (+ rib_bestchange_test.go)
- `internal/component/bgp/reactor/session.go` (+ recent_cache_test.go)
- `internal/core/redistevents/pool.go` (+ pool_test.go)
- `internal/component/bgp/wireu/wire_update.go`, `internal/component/bgp/types/rawmessage.go` (doc-comments)
