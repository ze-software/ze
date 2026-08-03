# Deferrals: fixit-bgp-per-family-prefix-enforcement

Deferral rows for this source. `/ze-status` folds the aggregate live backlog on
read from `plan/deferrals/`. Nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-30 | spec-fixit-bgp-per-family-prefix-enforcement | Expose per-family prefix staleness dates on an operator surface. The spec stores `updated` per family (`ze-bgp-conf.yang`). It then aggregates to the oldest date for the existing `prefix-updated` JSON key (`internal/component/bgp/plugins/cmd/peer/peer.go`) and for the staleness report bus (`internal/component/bgp/reactor/reactor_peers.go`). An operator cannot see WHICH family is stale | A per-family output field changes an external JSON surface and `plugin.PeerInfo` (`internal/component/plugin/types_bgp.go`). That is a new output capability. It is not part of correcting the parse defect. The enforcement goal holds without it, so this is separable per `ai/rules/completion.md` | `plan/spec-fixit-bgp-per-family-prefix-enforcement.md` (fold into the same spec when the reviewer judges the field cheap, otherwise name a successor spec here before closure) | deferred |
