# Deferrals: bgp-netns

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-bgp-netns / docs(vpp) netns correction | `ze doctor`'s `doctor-vpp-lcp-netns` remediation RECOMMENDS A CONFIG THAT BREAKS LCP: `doctor.go:124-125` emits "Set vpp.lcp.netns to host or root, or run BGP in that namespace". Per VPP's C source, `host`/`root` are namespace NAMES (`lcp.c:67` formats `/var/run/netns/%s`), so `netns host` makes VPP open `/var/run/netns/host`; absent a namespace of that literal name, LCP pair creation fails. The DETECTION is correct (BGP genuinely cannot bind in a non-root netns); only the remediation text is wrong. `lcpNetnsIsRootReachable`'s doc comment (`doctor.go:133-135`) asserts the same false premise | Found while correcting `docs/guide/vpp.md`'s matching false claim; the doc fix was docs-only by instruction, and changing an operator-facing doctor message plus the premise its helper documents is a code change wanting its own tests. The guide is corrected meanwhile, so the wrong advice now survives only in the tool | `plan/spec-fixit-vpp-lcp-reachability.md` (the doctor-check half after the split; it owns `doctor-vpp-lcp-*`) | deferred |

