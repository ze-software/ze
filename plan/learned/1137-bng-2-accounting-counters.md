# 1137: bng-2 -- Accounting Traffic Counters

**Objective:** Wire real per-subscriber byte/packet counters from pppN kernel interfaces into RADIUS accounting Interim-Update and Stop packets, replacing hardcoded zeros. Add RFC 2869 Gigaword attributes for sessions exceeding 4GB.

**Changes:**
| File | What changed | Why |
|------|-------------|-----|
| internal/component/l2tp/events/events.go | Added PppInterface field to SessionIPAssignedPayload | Accounting plugin needs pppN name to read counters |
| internal/component/l2tp/reactor.go | Capture sess.pppInterface under lock, populate in SessionIPAssigned emission | Field available at emission time (set during kernel setup) |
| internal/component/radius/dict.go | Added AttrAcctInputGigawords (52), AttrAcctOutputGigawords (53) | RFC 2869 Gigaword encoding |
| internal/component/l2tp/plugins/auth_radius/acct.go | Added pppInterface to acctSession, acctGetStats var, splitGigawords func, counter read + encoding in buildAcctPacket | Core feature: real counters in Interim/Stop packets |
| internal/component/l2tp/plugins/auth_radius/acct_test.go | 6 new tests + assertAttrUint32 helper | TDD coverage for all 10 ACs |
| rfc/short/rfc2869.md | Created protocol-only RFC summary | Required reading for Gigaword encoding |
| docs/features.md | Added accounting counter mention to L2TP BNG entry | Feature documentation |
| docs/comparison.md | Added RADIUS accounting counter attribute table | Comparison documentation |

**Design decisions:**
- Reused `iface.GetStats()` instead of creating separate sysfs reader (metrics.go already uses it; avoids duplication)
- Added PppInterface to SessionIPAssignedPayload rather than subscribing to a second event (simplest path, field already available)
- Gigaword attributes emitted only when >0 (saves 12 bytes/packet for <4GB sessions, RFC does not mandate zeros)
- Package-level `var acctGetStats = iface.GetStats` for test injection (follows metrics.go pattern)

**Deviations:**
- Eliminated planned counters.go/counters_linux.go/counters_other.go files (iface.GetStats handles platform abstraction)
- No functional test (.ci) created (requires live kernel PPPoL2TP for pppN interface)

**Not done:**
- Functional test with live traffic (needs kernel PPPoL2TP, covered by Docker lab in future)
- Acct-Terminate-Cause attribute (separate concern, not in spec scope)

**Risks & observations:**
- Packet counts truncate at uint32 (no RFC-defined Gigapackets attribute); sessions with >4 billion packets will wrap silently. Unlikely in practice.
- iface.GetStats returns zeros on non-Linux platforms; accounting will report zeros which is correct (no kernel pppN on non-Linux).

**Verification:** `go test -race ./internal/component/l2tp/plugins/auth_radius/... -v` -- all 62 tests pass. `make ze-verify` passes (only pre-existing scripts/evidence build failure).

## Files

None recorded.
