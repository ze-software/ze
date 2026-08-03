# Deferrals: fixit-bgp-session-fsm-lifecycle

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-17 | spec-fixit-bgp-session-fsm-lifecycle (Known Limitations) | ROUTE-REFRESH receipt does not restart the hold timer (`handleRouteRefresh` fires no FSM event, `session_handlers.go`), so a refresh-only peer survives on repeated graced re-arms rather than a proper reset. The source spec framed this as work owed: "candidate follow-up spec if Thomas wants RFC-shaped handling" | **Ruled by Thomas 2026-07-16: cancelled, because Ze is ALREADY the RFC-shaped side.** The framing was backwards. RFC 4271's FSM restarts HoldTimer on KeepAliveMsg (26) and UpdateMsg (27) only (`rfc/short/rfc4271.md`), and RFC 2918 states no hold-timer rule at all, so a peer that sends ROUTE-REFRESH but no KEEPALIVE is MEANT to time out. Firing an FSM event on ROUTE-REFRESH would be a divergence FROM the RFC, not toward it. The source spec's Known Limitation is corrected in the same commit; the `recentRead`-true-at-expiry observation it depended on still stands and is kept | cancelled / user-approved-drop | cancelled |
| 2026-07-19 | spec-fixit-bgp-session-fsm-lifecycle functional-proof | session.go/session_handlers.go wiring + counters + functional/interop tests owed | live-server/QEMU constraint, deferred to CI | plan/spec-fixit-bgp-session-fsm-lifecycle.md | **done 2026-08-03**: both Q-5 counters wired and asserted (`ze_bgp_hold_expiry_graced_total`, `ze_bgp_open_in_established_total`; `TestGracedHoldExpiryIncrementsCounter`, `TestRefusedOpenIncrementsCounter`), `test/plugin/deadpeer-holddown.ci` written and mutation-verified, `test/interop/scenarios/50-holdtime-deadpeer-frr/` written and mutation-verified against FRR 10.3.1 |

