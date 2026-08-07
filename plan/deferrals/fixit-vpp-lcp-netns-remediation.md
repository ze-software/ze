# Deferrals: fixit-vpp-lcp-netns-remediation

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-vpp-lcp-netns-remediation (R-5) | `doctor-vpp-lcp-netns` stays SILENT for `vpp.lcp.netns host` (or `root`): `lcpNetnsIsRootReachable` (`internal/plugins/iface/vpp/doctor.go`) returns true for those names, so no warning fires, yet VPP resolves the leaf as a namespace NAME under `/var/run/netns/` and LCP pair creation then fails at apply. An operator who sets `host` gets no diagnostic and a broken dataplane. This is a DETECTION gap, distinct from the false-remediation bug fixed in this commit | The remediation fix was scoped by the user to the message string and the doc comment; changing what the check DETECTS alters `lcpNetnsIsRootReachable`'s contract, which `lcpPairNetns` (`lcp.go`) also depends on, and wants its own tests plus a ruling on whether ze should reject the config outright rather than warn | `plan/spec-fixit-vpp-lcp-reachability.md` (the doctor-check half after the split; it owns `doctor-vpp-lcp-*`) | done |

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
| 2026-08-07 | review round 3 of the doctor-listener work | `validateNetns` (`internal/component/vpp/config.go`) rejects a path separator but admits WHITESPACE, so `vpp.lcp.netns "my ns"` validates and `GenerateStartupConf` writes `default netns my ns`. `lcp_itf_pair_config` (`third_party/vpp-linux-cp/src/lcp_interface.c`) matches `default netns %v`; the trailing `ns` then reaches no arm and the stanza returns `clib_error_return (0, "interfaces not found")`, so VPP fails startup on a config ze accepted | Pre-existing, and not what this round set out to change. The round made the EMPTY leaf valid, which is a different value class: it removes a directive rather than writing an unparseable one. Widening the same guard in the same commit would cost that commit its single focus and restart the gates already green (`ai/rules/rule-precedence.md`, closing comes first) | `plan/spec-fixit-vpp-lcp-reachability.md` (in-progress; it owns `vpp.lcp.netns` end to end) | open |

The fix is two lines in the same function ze already validates in, and the test
belongs beside `TestValidate`'s "netns with path separator" row. It is separable
because the goal that round served -- an operator can express the value ze doctor
recommends -- holds with the whitespace case left as it is. Anyone picking it up
should decide at the same time whether every character VPP's `unformat` cannot
carry belongs in one check, rather than adding a second special case.

