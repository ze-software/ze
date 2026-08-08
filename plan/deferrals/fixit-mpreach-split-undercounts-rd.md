# Deferrals: fixit-mpreach-split-undercounts-rd

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** SplitMPReachNLRIWithAddPath under-counts the RD under SAFI 128

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (closing review of the MP_REACH next-hop length fix) | `SplitMPReachNLRIWithAddPath` (`internal/component/bgp/message/update_split.go`) derives its next-hop overhead from `nh.Is4()` and omits the RD, so under SAFI 128 it under-counts 8 octets per next hop against `(*MPReachNLRI).nextHopOctets`, which is now the single source of that count | Same class as the three sibling attributes already homed above, and pre-existing: this diff neither introduced nor touched it. The MP_REACH goal holds without it. It wants the same treatment, which is to derive the count from `nextHopOctets` rather than re-deriving from the family, and that is one change across every site that still switches on `Is4()` | needs a destination spec | deferred |
