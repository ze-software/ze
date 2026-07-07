# 1074 -- unify-route-events

## Context

DESIGN-REVIEW finding 2 flagged two route-change event types for the same notion joined by a lossy adapter. `ribevents.BestChangeBatch` (rich, 15 per-entry FIB fields, feeds sysrib + flow enrichment) and `redistevents.RouteChangeBatch` (lean, pooled, value-type-only, feeds cross-protocol redistribution) are genuinely different layers. The bridge `EmitBestChange`/`convertBestChange` mapped only Action/Prefix/NextHop and silently dropped everything else, including the lean type's own `Metric`, the per-entry origin AS, and any entry whose action was not add/update/withdraw. The goal was to make the bridge lossless and drop-free without merging the two structs, closing latent correctness gaps (origin-AS loss, silent action drop) while preserving all externally observable redistribution output.

## Decisions

- Kept BOTH event types and made the bridge lossless, over (a) migrating sysrib/flowexport onto an enriched redistevents type or (b) emitting redistevents directly from the RIB. The FIB consumers need 12 fields (ECMP/Backup/Labels/SRv6/PathID/...) that have no redistribution consumer and would bloat the lean value-type leaf shared by 8 producers.
- Enriched the payload with only per-entry `OriginAS uint32` (plus populated the existing `Metric`), over adding all 12 dropped fields. Audit of the sole consumer (`redistribute_egress`) showed it reads only Action/Prefix/NextHop per entry and OriginASN/Community per batch; per-entry origin AS is the one thing a single batch `OriginASN` cannot express (BGP best-paths each have their own).
- Log-and-count unknown actions over silent `return false`. A future `RouteAction` enumerant reaching the bridge now surfaces via a warn (with the raw `action_code`, because `RouteAction.String()` renders an unknown value as "unspecified") and an `UnknownActionSkips()` atomic counter.
- Consumer prefers per-entry `OriginAS` when nonzero, else batch `OriginASN`. Fix lives in `dispatchEntryToConsumer` so both the incremental and replay callers (both pass `b.OriginASN`) get it.

## Consequences

- `redistevents` stays a value-type leaf: new fields must be fixed-size value types reset by the pool's `clear(b.Entries)` in `ReleaseBatch` (no per-field reset). `RouteChangeEntry` has no json tags, so it is an in-process EventBus payload with no external/forked-plugin contract to version.
- The as112 single-ASN virtual-router behavior is preserved by the zero-means-fall-back semantics: producers that leave per-entry `OriginAS` zero keep using the batch `OriginASN`.
- `Metric` is now carried but still not consumed: `configredist.RouteEntry` has no Metric field, so wiring it through to the OSPF/ISIS external-route metric is a scoped follow-up (spec R-3). The kernel producer already set `RouteChangeEntry.Metric`, so BGP is now merely consistent, not newly-dead-code.
- The bridge has no injected metrics registry, so the unknown-action counter is a package-level atomic exposed via `UnknownActionSkips()` rather than a Prometheus counter.

## Gotchas

- Adding the learned summary makes `ai/LEARNED-FULL-INDEX.md` stale, so `make ze-doc-test` (which runs as part of `ze-verify-wiring-docs`) fails until `make ze-discovery-index` regenerates it. Editing any doc/adding any learned file forces this discovery-index gate; run `make ze-discovery-index` and include the regenerated index in the commit. (The gate re-verifies `ai/DOCS-TO-CODE.md` too, but this change does not alter it.)
- `make ze-verify`'s full-suite run flaked on `TestPoolPreservesCapacityWithoutString` in `internal/core/textbuf` (sync.Pool capacity dropped to 128 under GC pressure); passes 5/5 in isolation. sync.Pool capacity assertions are inherently flaky in a full parallel run; scope verification to changed packages.
- The replay path (`replayBestPaths`) emits `ribevents.BestChange` only and never calls the bridge, so the bridge only ever sees incremental add/update/withdraw with Metric/OriginAS already populated by `checkBestPathChange`. Do not assume the bridge sees replay batches.

## Files

- `internal/core/redistevents/events.go` -- added `OriginAS uint32` to `RouteChangeEntry`
- `internal/core/redistevents/pool.go` -- comment: `clear()` covers value-type additions (no logic change)
- `internal/core/redistevents/pool_test.go` -- new: `TestRouteChangeBatchPoolResetsOriginAS` (AC-7)
- `internal/component/bgp/redistribute/producer.go` -- lossless `convertBestChange` (Metric+OriginAS, log-and-count unknown action) + `UnknownActionSkips()`
- `internal/component/bgp/redistribute/producer_test.go` -- extended + `MapsAllActions`, `UnknownActionLogged`
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` -- `dispatchEntryToConsumer` prefers per-entry OriginAS
- `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` -- `TestHandleBatchPrefersEntryOriginAS`
- `docs/architecture/core-design.md` -- per-entry OriginAS + lossless-bridge paragraph and anchors
- `ai/LEARNED-INDEX.md` + `ai/LEARNED-FULL-INDEX.md` -- index entry for this summary (discovery-index regen)
