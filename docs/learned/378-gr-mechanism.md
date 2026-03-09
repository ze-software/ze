# 378 — Per-Route Stale Tracking for Graceful Restart

## Objective

Add per-route stale tracking to the RIB storage layer so RFC 4724 Graceful Restart correctly retains stale routes during peer restart and selectively purges only stale routes — not fresh ones received during the GR window.

## Decisions

- `Stale bool` on RouteEntry is per-route metadata, NOT pooled — each route has independent stale state, unlike shared attribute handles
- Safety-net expiry timer in RIB (`restart-time + 5s` margin) auto-purges if bgp-gr never sends purge-stale — avoids stale routes persisting forever on plugin failure
- bgp-gr uses 3-step session-down sequence: `purge-stale` → `retain-routes` → `mark-stale` — handles both first disconnect (purge is no-op) and consecutive restart (purges old stale before re-marking)
- Nuclear `release-routes` kept for timer expiry only — peer never came back, delete everything
- `rib show in` shows `"stale": true` per-route; `stale-at`/`expires-at` times are per-peer in `rib status` (not duplicated per-route)

## Patterns

- Timer callback ownership guard: `autoExpireStale` captures `*peerGRState` pointer in closure, checks `r.grState[peer] == owner` before acting — prevents stale callbacks from interfering with consecutive restart cycles
- Implicit unstale via Insert: `FamilyRIB.Insert()` clears stale on both same-attrs no-op path (`oldEntry.Stale = false`) and different-attrs replacement (new entry defaults to `Stale=false`)
- `time.AfterFunc` creates a lifecycle-scoped goroutine (one per GR cycle) — acceptable per goroutine rules, not a per-event goroutine

## Gotchas

- Timer race on consecutive restart: old `time.AfterFunc` callback can fire after `Stop()` if the goroutine already started — `Stop()` returning false means the callback is in-flight, not that it won't run. Required the ownership guard pattern.
- `Clone()` on RouteEntry does NOT copy `Stale` — no current callers in GR paths, but would silently lose stale state if used for GR route copying in the future
- `entriesEqual` compares pool handles (not wire bytes) — same attributes after dedup produce identical handles, so the same-attrs no-op path works correctly for implicit unstale

## Files

- `internal/component/bgp/plugins/rib/storage/routeentry.go` — `Stale bool` field
- `internal/component/bgp/plugins/rib/storage/familyrib.go` — `MarkStale`, `PurgeStale`, `StaleCount`, Insert stale clearing
- `internal/component/bgp/plugins/rib/storage/peerrib.go` — aggregate stale methods
- `internal/component/bgp/plugins/rib/rib.go` — `peerGRState`, `grState` map
- `internal/component/bgp/plugins/rib/rib_commands.go` — `mark-stale`, `purge-stale`, `autoExpireStale`, status enrichment
- `internal/component/bgp/plugins/rib/rib_attr_format.go` — stale flag in show output
- `internal/component/bgp/plugins/gr/gr.go` — 3-step session-down, EOR purge-stale
- `test/plugin/gr-mark-stale.ci`, `test/plugin/gr-purge-stale-eor.ci` — functional tests
- `docs/architecture/plugin/rib-storage-design.md` — stale tracking section
