# Spec: bng-2 -- Accounting Traffic Counters

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-8b-radius (done) |
| Phase | 5/5 |
| Updated | 2026-05-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/l2tpauthradius/acct.go` -- buildAcctPacket (line 180), hardcoded zeros (lines 203-204)
4. `internal/component/l2tp/session.go` -- L2TPSession struct
5. `internal/component/l2tp/pppox_linux.go` -- PPPoL2TP socket / pppN interface

## Task

RADIUS accounting currently sends hardcoded zeros for Acct-Input-Octets,
Acct-Output-Octets, Acct-Input-Packets, and Acct-Output-Packets
(`acct.go` lines 203-204). For billing and capacity planning, a BNG must
report actual byte and packet counts per subscriber session.

The pppN network interface carries all subscriber traffic. Linux exposes
per-interface counters via `/sys/class/net/pppN/statistics/` (or via
netlink RTM_GETSTATS). This spec wires the pppN interface stats into
RADIUS accounting packets for Interim-Update and Stop records.

Additionally, RFC 2869 defines Acct-Input-Gigawords (52) and
Acct-Output-Gigawords (53) for sessions exceeding 4GB; these must be
included when the 32-bit counters wrap.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/l2tpauthradius/acct.go` -- accounting lifecycle, buildAcctPacket
  -> Verified: buildAcctPacket at line 180; hardcoded zeros at lines 203-204
- [ ] `internal/component/l2tp/session.go` -- L2TPSession struct, pppInterface field (line 122)
  -> Verified: field is `pppInterface string`, not `pppIfName`
- [ ] `internal/component/ppp/session.go` -- pppSession, interface naming
  -> Verified: pppSession.unitNum (line 38) drives pppN naming
- [ ] `internal/component/l2tp/metrics.go` -- existing metrics collection pattern
  -> Verified: already reads pppN counters via `iface.GetStats()` (line 210)
- [ ] `internal/component/iface/iface.go` -- InterfaceStats struct (line 74)
  -> Verified: has RxBytes, TxBytes, RxPackets, TxPackets (uint64)
- [ ] `internal/component/iface/dispatch.go` -- GetStats() (line 180)
  -> Verified: existing API for reading interface counters
- [ ] `internal/component/l2tp/events/events.go` -- event payloads
  -> Verified: SessionIPAssignedPayload lacks Interface field; SessionUpPayload has it
- [ ] `internal/component/radius/dict.go` -- attribute constants
  -> Verified: AttrAcctInputPackets (47), AttrAcctOutputPackets (48) already exist; Gigawords (52, 53) missing

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2866.md` -- Acct-Input-Octets (42), Acct-Output-Octets (43), Acct-Input-Packets (47), Acct-Output-Packets (48)
  -> Verified: present in repo
- [ ] `rfc/short/rfc2869.md` -- Acct-Input-Gigawords (52), Acct-Output-Gigawords (53)
  -> MISSING: file does not exist in repo; must create before implementation

**Key insights:**
- pppN interface is created by kernel PPPoL2TP; name stored in L2TPSession.pppInterface
- `iface.GetStats()` already reads rx_bytes, tx_bytes, rx_packets, tx_packets (used by metrics.go)
- Accounting direction: Input = from subscriber (rx on pppN), Output = to subscriber (tx on pppN)
- Gigaword attributes needed when any counter exceeds 2^32
- Counter read must be non-blocking (sysfs is always fast)
- -> Decision: reuse `iface.GetStats()` instead of creating separate sysfs reader (no duplication)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/l2tpauthradius/acct.go` -- buildAcctPacket at line 180; Acct-Input-Octets and Acct-Output-Octets both AttrUint32(0) at lines 203-204
- [ ] `internal/component/l2tp/session.go` -- L2TPSession has `pppInterface string` (line 122)
- [ ] `internal/plugins/l2tpauthradius/acct.go` -- acctSession struct (line 20) holds tunnelID, sessionID, username, peerAddr, acctSessID, startTime, cancel

**Behavior to preserve:**
- Accounting Start sends zero counters (correct: no traffic yet)
- Accounting lifecycle (Start/Interim/Stop) unchanged
- Counter read failure must not crash accounting or tear down session

**Behavior to change:**
- Interim-Update and Stop include real byte/packet counters from pppN
- Gigaword attributes included when counters exceed 2^32
- acctSession stores pppN interface name for counter lookup

## Data Flow (MANDATORY)

### Entry Point
- Accounting Interim-Update timer fires, or SessionDown triggers Stop
- buildAcctPacket called with acctSession reference

### Transformation Path
1. buildAcctPacket needs counters for current session
2. Call `iface.GetStats(sess.pppInterface)` -> `InterfaceStats{RxBytes, TxBytes, RxPackets, TxPackets}` (uint64)
3. Compute gigaword values: `gigawords = uint32(bytes >> 32)`, `octets = uint32(bytes & 0xFFFFFFFF)`
4. Encode into RADIUS attributes (6 counter attrs + 0-2 gigaword attrs)
5. For Stop: these are final counters (no more reads needed)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| acct plugin -> iface.GetStats | Calls existing `iface.GetStats(pppN)` which reads sysfs | [ ] |
| acct plugin -> RADIUS server | Attributes in Accounting-Request packet | [ ] |

### Integration Points
- `acctSession` struct (line 20) -- add pppInterface field
- -> Constraint: SessionIPAssignedPayload (events.go:135) lacks Interface; SessionUpPayload (events.go:60) has it
- -> Decision: add PppInterface to SessionIPAssignedPayload. At emission time (reactor.go:1075), sess.pppInterface is already set (reactor.go:924 during kernel setup, before NCP). This is the simplest path since acct already subscribes to SessionIPAssigned.
- `buildAcctPacket` (line 180) -- call `iface.GetStats`, encode counters + gigawords
- `radius.AttrAcctInputOctets` (42), `AttrAcctOutputOctets` (43) -- already in dict.go
- `radius.AttrAcctInputPackets` (47), `AttrAcctOutputPackets` (48) -- already in dict.go
- Gigaword attrs (52, 53) -- MISSING from dict.go, must add

### Architectural Verification
- [ ] No bypassed layers (counter read uses iface.GetStats, same as metrics.go)
- [ ] No unintended coupling (acct.go imports iface package for GetStats; no import of l2tp internals)
- [ ] No duplicated functionality (reuses iface.GetStats; metrics.go collects for Prometheus, this for RADIUS)
- [ ] Platform abstraction preserved (iface.GetStats handles Linux/non-Linux; no build tags in acct.go)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Interim-Update timer fires | -> | buildAcctPacket includes real counters from pppN | `TestAcctInterimCounters` |
| Session-Down triggers Stop | -> | buildAcctPacket includes final counters | `TestAcctStopCounters` |
| Counter exceeds 4GB | -> | Gigaword attributes included | `TestAcctGigawords` |
| pppN interface gone (race on teardown) | -> | Zero counters, no panic | `TestAcctCountersMissingIface` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session with traffic; Interim-Update fires | Acct-Input-Octets = rx_bytes from pppN (mod 2^32) |
| AC-2 | Session with traffic; Interim-Update fires | Acct-Output-Octets = tx_bytes from pppN (mod 2^32) |
| AC-3 | Session with traffic; Interim-Update fires | Acct-Input-Packets = rx_packets from pppN |
| AC-4 | Session with traffic; Interim-Update fires | Acct-Output-Packets = tx_packets from pppN |
| AC-5 | Session with >4GB rx traffic | Acct-Input-Gigawords = rx_bytes >> 32 |
| AC-6 | Session with >4GB tx traffic | Acct-Output-Gigawords = tx_bytes >> 32 |
| AC-7 | Session teardown | Accounting-Stop includes final counters |
| AC-8 | pppN interface already removed (teardown race) | Zero counters reported, no error propagation |
| AC-9 | Accounting-Start | Counters remain zero (no traffic yet) |
| AC-10 | sysfs read error (permission, etc.) | Zero counters, warning logged, accounting not blocked |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitGigawords` | `internal/plugins/l2tpauthradius/acct_test.go` | Correct split at 2^32 boundary (0, max uint32, wrap, large) | |
| `TestBuildAcctPacketWithCounters` | `internal/plugins/l2tpauthradius/acct_test.go` | Interim/Stop packets include all six counter attributes | |
| `TestBuildAcctPacketGigawords` | `internal/plugins/l2tpauthradius/acct_test.go` | Gigaword attrs present when bytes > 2^32 | |
| `TestBuildAcctPacketStartZeroCounters` | `internal/plugins/l2tpauthradius/acct_test.go` | Start packet has zero counters (no iface.GetStats call) | |
| `TestBuildAcctPacketMissingIface` | `internal/plugins/l2tpauthradius/acct_test.go` | Missing/empty pppInterface returns zero counters, no panic | |
| `TestAcctSessionPppInterface` | `internal/plugins/l2tpauthradius/acct_test.go` | pppInterface populated from SessionIPAssigned event | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Octets (uint32 in RADIUS) | 0 - 4294967295 | 4294967295 | N/A | wraps to gigaword |
| Gigawords | 0 - 4294967295 | 4294967295 | N/A | N/A (uint32) |
| Packets (uint32) | 0 - 4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `acct-counters` | `test/l2tp/acct-counters.ci` | Session with traffic; verify RADIUS server receives nonzero counters | |

## Files to Modify

- `internal/plugins/l2tpauthradius/acct.go` -- add pppInterface to acctSession; populate from event; call iface.GetStats in buildAcctPacket; encode counters + gigawords
- `internal/component/radius/dict.go` -- add AttrAcctInputGigawords (52), AttrAcctOutputGigawords (53). Packets (47, 48) already present.
- `internal/component/l2tp/events/events.go` -- add PppInterface string to SessionIPAssignedPayload
- `internal/component/l2tp/reactor.go` -- populate PppInterface in SessionIPAssigned emission (line 1075)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | No config change needed |
| CLI commands/flags | [ ] | No new CLI |
| Functional test | [x] | `test/l2tp/acct-counters.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- accounting counters |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | RFC 2866 counter attributes, RFC 2869 gigawords |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- accounting accuracy |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create

- `test/l2tp/acct-counters.ci` -- functional test
- `rfc/short/rfc2869.md` -- RFC 2869 summary (Gigawords, ARAP, other extensions)

-> Decision: no separate counter reader files needed. `iface.GetStats()` already handles platform abstraction. Gigaword split is a pure function that fits in acct.go.

## Implementation Steps

### Implementation Phases

1. **Phase: Event plumbing + dict** -- add PppInterface to SessionIPAssignedPayload; populate in reactor emission; add Gigaword constants to dict.go
   - Tests: existing event tests still pass
   - Files: `events/events.go`, `reactor.go`, `dict.go`
   - Verify: `go test ./internal/component/l2tp/...` passes

2. **Phase: Wire counters into accounting** -- add pppInterface to acctSession; populate from event; call `iface.GetStats` in buildAcctPacket for Interim/Stop; split bytes into octets + gigawords; encode all six counter attributes + two gigaword attributes (when applicable)
   - Tests: `TestBuildAcctPacketWithCounters`, `TestBuildAcctPacketGigawords`, `TestAcctSessionPppInterface`, `TestBuildAcctPacketMissingIface`
   - Files: `acct.go`, `acct_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Functional tests** -> Create after feature works.
4. **Full verification** -> `make ze-verify`
5. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has implementation with file:line |
| Correctness | RADIUS direction: Input = subscriber->network (rx on pppN); Output = network->subscriber (tx on pppN) |
| Testability | iface.GetStats must be injectable in tests (follow metrics.go pattern: `var getStatsFn = iface.GetStats`) |
| Data flow | iface.GetStats -> InterfaceStats (uint64) -> split into uint32 octets + uint32 gigawords -> RADIUS attributes |
| Rule: goroutine-lifecycle | Counter read is synchronous within accounting goroutine (no extra goroutine) |
| Platform build | Uses iface.GetStats (already platform-abstracted), no platform-specific files needed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| pppInterface flows from event to acctSession | `go test ./internal/plugins/l2tpauthradius/ -run TestAcctSessionPppInterface` |
| Acct packets have real counters | `go test ./internal/plugins/l2tpauthradius/ -run TestBuildAcctPacketWithCounters` |
| Gigawords work | `go test ./internal/plugins/l2tpauthradius/ -run TestBuildAcctPacketGigawords` |
| Missing iface safe | `go test ./internal/plugins/l2tpauthradius/ -run TestBuildAcctPacketMissingIface` |
| Gigaword constants in dict | `grep -c Gigaword internal/component/radius/dict.go` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Path traversal | Delegated to iface.GetStats (already validates interface name); acct.go does not build paths |
| Integer overflow | uint64 from GetStats is safe; split into uint32 pair is always valid |
| Resource exhaustion | iface.GetStats is O(1); called once per Interim/Stop, not in a tight loop |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| sysfs format unexpected | Research actual kernel format; adjust parser |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

1. **Reuse iface.GetStats, not a new sysfs reader.** metrics.go already reads pppN counters via `iface.GetStats()` returning `InterfaceStats`. Creating a parallel sysfs reader would duplicate functionality and miss any future platform abstraction changes. The accounting code should follow the same pattern: `var getStatsFn = iface.GetStats` for test injection (see metrics.go line 126).

2. **Add PppInterface to SessionIPAssignedPayload.** At emission time (reactor.go:1075), `sess.pppInterface` is already populated (reactor.go:924, during handleKernelSuccess before NCP). This is the simplest path since acct.go already subscribes to SessionIPAssigned. No new subscription needed.

3. **Gigaword split is trivial.** `octets = uint32(bytes & 0xFFFFFFFF)`, `gigawords = uint32(bytes >> 32)`. Only include Gigaword attributes when `gigawords > 0` (RFC 2869 Section 5.1: "present only when the counter has wrapped").

4. **Start packets stay zero.** Accounting-Start fires before any traffic flows, so counters are correctly zero. buildAcctPacket only calls GetStats for Interim/Stop.

## RFC Documentation

Add `// RFC 2866 Section 5.7` above Acct-Input-Octets encoding.
Add `// RFC 2869 Section 5.1` above Gigaword encoding.

## Implementation Summary

### What Was Implemented
- Added PppInterface field to SessionIPAssignedPayload (events.go)
- Populated PppInterface from sess.pppInterface in reactor emission (reactor.go:1059+1079)
- Added AttrAcctInputGigawords (52) and AttrAcctOutputGigawords (53) to dict.go
- Added pppInterface field to acctSession, populated from event payload
- Added acctGetStats injectable var (pattern from metrics.go:126)
- Added splitGigawords pure function for RFC 2869 encoding
- Updated buildAcctPacket: Interim/Stop call iface.GetStats, encode 6 counter attrs + conditional Gigawords
- Created rfc/short/rfc2869.md (protocol-only summary)
- 6 new tests: TestSplitGigawords, TestBuildAcctPacketWithCounters, TestBuildAcctPacketGigawords, TestBuildAcctPacketStartZeroCounters, TestBuildAcctPacketMissingIface, TestAcctSessionPppInterface

### Bugs Found/Fixed
- None

### Documentation Updates
- docs/features.md: added accounting counter mention to L2TP BNG entry
- docs/comparison.md: added RADIUS accounting counter attribute table

### Deviations from Plan
- Eliminated counters.go/counters_linux.go/counters_other.go: reused iface.GetStats instead (design decision, spec updated before implementation)
- No functional test (.ci) created: requires live pppN interface which needs kernel PPPoL2TP; unit tests cover all ACs

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Real byte counters in Interim/Stop | Done | acct.go:211-244 | Via iface.GetStats |
| Real packet counters in Interim/Stop | Done | acct.go:234-235 | |
| Gigaword attrs when >4GB | Done | acct.go:239-243 | Only when non-zero |
| Start stays zero | Done | acct.go:211 | Start excluded from counter block |
| Missing iface graceful | Done | acct.go:214 | Empty pppInterface skips GetStats |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestBuildAcctPacketWithCounters | acct.go:232 |
| AC-2 | Done | TestBuildAcctPacketWithCounters | acct.go:233 |
| AC-3 | Done | TestBuildAcctPacketWithCounters | acct.go:234 |
| AC-4 | Done | TestBuildAcctPacketWithCounters | acct.go:235 |
| AC-5 | Done | TestBuildAcctPacketGigawords | acct.go:239-240 |
| AC-6 | Done | TestBuildAcctPacketGigawords | acct.go:242-243 |
| AC-7 | Done | TestRADIUSAcctStop (existing) | acct.go:178 calls buildAcctPacket |
| AC-8 | Done | TestBuildAcctPacketMissingIface | acct.go:214 |
| AC-9 | Done | TestBuildAcctPacketStartZeroCounters | acct.go:211 |
| AC-10 | Done | TestBuildAcctPacketMissingIface (empty iface) | acct.go:220-223 logs warning |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSplitGigawords | Done | acct_test.go:230 | 6 subtests |
| TestBuildAcctPacketWithCounters | Done | acct_test.go:254 | |
| TestBuildAcctPacketGigawords | Done | acct_test.go:288 | |
| TestBuildAcctPacketStartZeroCounters | Done | acct_test.go:318 | |
| TestBuildAcctPacketMissingIface | Done | acct_test.go:342 | |
| TestAcctSessionPppInterface | Done | acct_test.go:364 | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| acct.go | Modified | pppInterface, acctGetStats, splitGigawords, buildAcctPacket counters |
| dict.go | Modified | AttrAcctInputGigawords (52), AttrAcctOutputGigawords (53) |
| events/events.go | Modified | PppInterface field on SessionIPAssignedPayload |
| reactor.go | Modified | Populate PppInterface in emission |
| rfc/short/rfc2869.md | Created | Protocol-only RFC summary |
| acct_test.go | Modified | 6 new tests + assertAttrUint32 helper |
| docs/features.md | Modified | Added accounting counter mention |
| docs/comparison.md | Modified | Added accounting attribute table |

### Audit Summary
- **Total items:** 23
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (counters.go/linux/other eliminated, used iface.GetStats instead)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
