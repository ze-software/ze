# Deferrals: fixit-vpp-lcp-netns-remediation

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-vpp-lcp-netns-remediation (R-5) | `doctor-vpp-lcp-netns` stays SILENT for `vpp.lcp.netns host` (or `root`): `lcpNetnsIsRootReachable` (`internal/plugins/iface/vpp/doctor.go:136-143`) returns true for those names, so no warning fires, yet VPP resolves the leaf as a namespace NAME under `/var/run/netns/` and LCP pair creation then fails at apply. An operator who sets `host` gets no diagnostic and a broken dataplane. This is a DETECTION gap, distinct from the false-remediation bug fixed in this commit | The remediation fix was scoped by the user to the message string and the doc comment; changing what the check DETECTS alters `lcpNetnsIsRootReachable`'s contract, which `lcpPairNetns` (`lcp.go:109-114`) also depends on, and wants its own tests plus a ruling on whether ze should reject the config outright rather than warn | `plan/spec-fixit-vpp-lcp-reachability.md` (the doctor-check half after the split; it owns `doctor-vpp-lcp-*`) | deferred |

