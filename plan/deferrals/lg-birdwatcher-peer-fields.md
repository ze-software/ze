# Deferrals: lg-birdwatcher-peer-fields

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-15 | spec-lg-birdwatcher-peer-fields (Phase 4) | Populate the birdwatcher `routes_received` / `routes_imported` / `routes_exported` / `routes_filtered` fields. `state_changed` and `last_error` are IMPLEMENTED (see the spec Progress section); only the route counts remain | A-4 resolved to PARTIALLY BROKEN and Phase 4 stopped per the spec's own Failure Routing. Only `routes_received` has a correct source (`AdjRIBInManager.status()`, `adj_rib_in/rib_commands.go:220-236`, per-peer counts). `routes_imported`/`routes_accepted` have NONE: the Adj-RIB-In is PRE-policy (RFC 4271 3.2), so mapping its count to "accepted" would report a number that is wrong rather than absent, which `ai/rules/no-workarounds-for-missing-behavior.md` forbids. `routes_exported` has no inbound-only source; `routes_filtered` is untracked repo-wide (`ai/rules/project-knowledge.md`). Also `bgp-adj-rib-in` is an OPTIONAL plugin, so the one available count is config-conditional. Needs a design decision, not more plumbing | `plan/spec-bgp-filtered-route-storage.md` (routes_filtered; design, rewritten against BIRD v2.19.0 source) + `plan/spec-bgp-per-peer-received-counter.md` (routes_received, pre-policy). Re-homed 2026-07-16: the original destination `spec-lg-birdwatcher-peer-fields.md` was deleted at closure, orphaning this row | deferred |

