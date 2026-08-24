# Deferrals: bgp-netns

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-bgp-netns / docs(vpp) netns correction | `ze doctor`'s `doctor-vpp-lcp-netns` remediation RECOMMENDS A CONFIG THAT BREAKS LCP: `doctor.go` emits "Set vpp.lcp.netns to host or root, or run BGP in that namespace". Per VPP's C source, `host`/`root` are namespace NAMES (`lcp.c:67` formats `/var/run/netns/%s`), so `netns host` makes VPP open `/var/run/netns/host`; absent a namespace of that literal name, LCP pair creation fails. The DETECTION is correct (BGP genuinely cannot bind in a non-root netns); only the remediation text is wrong. `lcpNetnsIsRootReachable`'s doc comment (`doctor.go`) asserts the same false premise | Found while correcting `docs/guide/vpp.md`'s matching false claim; the doc fix was docs-only by instruction, and changing an operator-facing doctor message plus the premise its helper documents is a code change wanting its own tests. The guide is corrected meanwhile, so the wrong advice now survives only in the tool | `spec-fixit-vpp-lcp-reachability` (the doctor-check half after the split; it owned `doctor-vpp-lcp-*`) | done |

Resolved 2026-08-24, at the closure of `spec-fixit-vpp-lcp-reachability`. No doctor
surface names a `vpp.lcp.netns` value as the remedy now. `lcpNetnsMarkerDiagnostic` and
`lcpNetnsConfigDiagnostic` (`internal/plugins/iface/vpp/doctor.go`) both print "Leave
vpp.lcp.netns empty to keep the TAPs in VPP's own network namespace", which is the one
value `lcp_set_default_ns` (`third_party/vpp-linux-cp/src/lcp.c`) treats as no namespace
at all. The false premise in the helper's doc comment went with the rename to
`lcpNetnsIsRootMarker`, whose comment now states that a marker is a namespace NAME.
Tests: `TestDoctorLCPNetnsRemediation` and `TestDoctorLCPNetnsCodeDescription`
(`internal/plugins/iface/vpp/register_test.go`) assert the doctor message and the
registered code description that `ze explain` prints.

