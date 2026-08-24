# Deferrals: fixit-vpp-lcp-netns-remediation

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-vpp-lcp-netns-remediation (R-5) | `doctor-vpp-lcp-netns` stays SILENT for `vpp.lcp.netns host` (or `root`): `lcpNetnsIsRootReachable` (`internal/plugins/iface/vpp/doctor.go`) returns true for those names, so no warning fires, yet VPP resolves the leaf as a namespace NAME under `/var/run/netns/` and LCP pair creation then fails at apply. An operator who sets `host` gets no diagnostic and a broken dataplane. This is a DETECTION gap, distinct from the false-remediation bug fixed in this commit | The remediation fix was scoped by the user to the message string and the doc comment; changing what the check DETECTS alters `lcpNetnsIsRootReachable`'s contract, which `lcpPairNetns` (`lcp.go`) also depends on, and wants its own tests plus a ruling on whether ze should reject the config outright rather than warn | `spec-fixit-vpp-lcp-reachability` (the doctor-check half after the split; it owned `doctor-vpp-lcp-*`) | done |

R-5 landed 2026-08-07 in `internal/plugins/iface/vpp/doctor.go`. `checkVPPLCPNetns` now
speaks for every non-empty `vpp.lcp.netns`, and the empty leaf is the only silent value:
`lcpNetnsConfigDiagnostic` warns that a host marker is not the host namespace, and
`lcpNetnsHostDiagnostic` warns when the named namespace is absent from
`/var/run/netns/`, which is what fails LCP pair creation at apply. The contract concern
in the Reason column is answered by separating the two questions rather than widening
one: `lcpNetnsIsRootMarker` (renamed from `lcpNetnsIsRootReachable`, the name that made
the markers look safe) still answers "is this one of ze's markers" for `lcpPairNetns`
(`lcp.go`), whose behavior is unchanged. Tests: `TestDoctorLCPNetnsRootMarkerWarns`,
`TestDoctorLCPNetnsEmptyLeafSilent`, `TestDoctorLCPNetnsAbsentFromHost`,
`TestDoctorLCPNetnsPresentOnHost`, `TestDoctorLCPNetnsProbeError`
(`internal/plugins/iface/vpp/register_test.go`); each was proved to go RED against a
mutation of the leg it covers. The reject-versus-warn question in the Reason column is
NOT decided here: the check warns, and the ruling is open for Thomas.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-07 | review round 3 of the doctor-listener work | `validateNetns` (`internal/component/vpp/config.go`) rejects a path separator but admits WHITESPACE, so `vpp.lcp.netns "my ns"` validates and `GenerateStartupConf` writes `default netns my ns`. `lcp_itf_pair_config` (`third_party/vpp-linux-cp/src/lcp_interface.c`) matches `default netns %v`; the trailing `ns` then reaches no arm and the stanza returns `clib_error_return (0, "interfaces not found")`, so VPP fails startup on a config ze accepted | Pre-existing, and not what this round set out to change. The round made the EMPTY leaf valid, which is a different value class: it removes a directive rather than writing an unparseable one. Widening the same guard in the same commit would cost that commit its single focus and restart the gates already green (`ai/rules/rule-precedence.md`, closing comes first) | `spec-fixit-vpp-lcp-reachability` (closed 2026-08-24; it owned `vpp.lcp.netns` end to end) | done |

Resolved 2026-08-14 by `5503a81c5` ("fix(vpp): refuse an lcp netns VPP would silently
truncate"), and recorded here at the closure of `spec-fixit-vpp-lcp-reachability` on
2026-08-24. `validateNetns` (`internal/component/vpp/config.go`) now walks the name and
refuses any rune for which `unicode.IsSpace` is true or `unicode.IsPrint` is false, so
`vpp.lcp.netns "my ns"` is rejected at parse rather than written into startup.conf. The
open question in the paragraph this replaces was answered the way it asked: the guard
screens the whole character class VPP's `unformat` cannot carry, rather than adding a
second special case beside the path separator. Tests live beside `TestValidate` in
`internal/component/vpp/config_test.go`, which drives a space, a tab, a newline and a NUL,
and the commit records that disabling the character check accepts all of them.

