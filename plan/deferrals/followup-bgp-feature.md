# Deferrals: followup-bgp-feature

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-10 | spec-followup-bgp-feature item 3 | AS-Confederation OTC (RFC 9234 §5 confederation rules: OTC value = confed identifier, member-AS semantics) | User decision 2026-07-08: re-defer unchanged — ze is a single-AS speaker (`role.getLocalASN`, role.go) so §5 is vacuously satisfied; true support needs confederation-member config + AS_CONFED origination first (large feature) | `plan/spec-bgp-deferred-confederation-otc.md` | deferred |
| 2026-07-10 | spec-followup-bgp-feature item 4 | community-name web decorator leaf wiring (decorator registered in service_web.go and functional; no community leaf exists in the BGP YANG to attach it to) | Blocked on a BGP YANG community leaf existing; recorded 2026-07-08 at item 4 completion | `plan/spec-bgp-deferred-community-name-leaf.md` | deferred |

