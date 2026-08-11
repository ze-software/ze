# Deferrals: fixit-commit-rail-nexthop-unvalidated

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** the rib CommitService rail lost the accidental next-hop guard

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (closing review of the MP_REACH next-hop length fix) | `(*CommitService).packAttributesWithASPath` (`internal/component/bgp/rib/commit.go`) is a THIRD rail that lost the same accidental guard. It sizes with `attrSize` (reaching `MPReachNLRI.Len`) and writes with `WriteAttrTo` (reaching `WriteTo`), then refuses on `off != totalLen`. Before the length fix a zero `netip.Addr` made those 16 apart, so `Commit` returned `BUG: attribute size mismatch`; both now agree at 0, the check passes, and `peer.SendUpdate` would carry NH_Len `0x00` (RFC 7606 Section 7.11, session reset). `useTraditionalNLRI` gates on `nextHop.Is4()`, so an IPv4-unicast route with an unset next hop lands in MP_REACH here too | The rail is DEAD: `(*Transaction).QueueAnnounce` (`internal/component/bgp/transaction/commit_manager.go`) has no non-test caller, so `tx.Routes()` is empty and `SendRoutes` never reaches `cs.Commit` with routes. The goal this work exists to achieve does not depend on it (`ai/rules/rule-precedence.md`: a defect the goal does not need is fixed AFTER closing, never on the way). Fix is one call to `ValidateNextHops` in `(*CommitService).buildMPReachNLRI`. Whoever takes it should also establish whether the set of size-versus-write mismatch checks is CLOSED, since three were found one at a time: `(*announceAttrs).add`, `buildRIBRouteUpdate`, and this one | `plan/spec-bgp-rib-deferred-commit-nexthop-validation.md` | deferred |
