# Deferrals: fixit-load-dependent-functional-failures

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-24 | spec-fixit-load-dependent-functional-failures | Forward-path egress-rail divergence fix (372 remove-private-as-replace-peer, 378 rfc7606-relay-one-field, 394/395 role-otc-egress-filter/stamp, 351 redistribute-l2tp-multi-peer-nexthop): route the adj-rib-in peer-up replay through the forward rail via a new `RelayStoredRoute` reactor primitive (+ RPC/SDK), per-family MP_REACH reconstruction, add-path path-id gap, replay-owner dedupe | Owner decision 2026-07-24: the forward-rail fix is a spec-sized new plugin-protocol primitive (not a redirect), touches the routing hot path, and needs its own interop test + independent review. Carved out so the harness (Phase 1) and 345 (router-id opt-in) — both done and stress-validated — land cleanly | `plan/spec-fixit-bgp-egress-rail-divergence.md` | deferred |
