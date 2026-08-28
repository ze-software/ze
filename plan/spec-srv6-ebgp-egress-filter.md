# Spec: srv6-ebgp-egress-filter -- Suppress Prefix-SID on EBGP Egress

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-16 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable;
all citations below updated in-body to the verified current lines --
`AcceptSRv6PrefixSID` `peer_settings.go`, egress insertion points
`reactor_api_forward.go` and `forward_rs.go`,
`ze-bgp-conf.yang`; `config.go` unchanged. These reactor files are
churny: re-verify by symbol at implementation start). (Re-verified
2026-07-23 after the origin/main fast-forward to 822029463: `forward_rs.go`
grew, moving its `applyFactsNextHop`/`applyFactsSendCommunity` pair
`:347-348` -> `:365-366`; the rest held.)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc8669.md` - Section 4/5/8: EBGP propagation rules
4. `internal/component/bgp/reactor/peer_forward_facts.go` - egress fact precomputation + apply
5. `internal/component/bgp/reactor/reactor_api_forward.go` - egress pipeline insertion point
6. `internal/component/bgp/reactor/forward_rs.go` - RS egress pipeline insertion point

## Task

RFC 8669 Section 4: "A BGP speaker receiving a BGP Prefix-SID attribute from an
External BGP (EBGP) neighbor residing outside the boundaries of the SR domain
MUST discard the attribute unless it is configured to accept the attribute from
the EBGP neighbor."

Ze implements the ingress side (`accept-srv6-prefix-sid` config option). But on
egress, Ze does not suppress Prefix-SID when advertising to EBGP peers outside
the SR domain.

RFC 8669 Section 8: "The propagation to other ASes MUST be explicitly configured."
This is a SHOULD-level concern (Section 5 says "SHOULD NOT advertise...outside an
AS unless explicitly configured"), but proper domain boundary enforcement needs an
egress policy knob.

**Goal:** Add a per-peer config boolean `propagate-srv6-prefix-sid` (default `false`)
that controls whether Prefix-SID (attr code 40) is included in egress UPDATEs to
EBGP peers. Default behavior: strip. Explicit `true` required to propagate across
AS boundaries.

### Design decisions (resolved)

1. **Default = strip on EBGP egress (safe default).** RFC 8669 Section 8 MUST:
   "propagation to other ASes MUST be explicitly configured." Matches the ingress
   side where `accept-srv6-prefix-sid` defaults to `false`.
2. **Config surface: per-peer session boolean** (`propagate-srv6-prefix-sid`), not
   an export filter action. Mirrors the ingress pattern exactly. RFC-mandated
   attribute suppression belongs in reactor forwarding facts, not user policy filters.
3. **Scope: entire attribute code 40** (all Prefix-SID TLVs). RFC 8669 Section 8
   refers to "the attribute" wholesale. The ingress side discards the entire attribute.
   Egress should be symmetric.

### Key source files

- `internal/component/bgp/reactor/peer_forward_facts.go` - egress fact precomputation (pattern to follow)
- `internal/component/bgp/reactor/reactor_api_forward.go` - egress pipeline insertion point
- `internal/component/bgp/reactor/forward_rs.go` - RS egress pipeline insertion point
- `internal/component/bgp/reactor/peer_settings.go` - `AcceptSRv6PrefixSID` (ingress counterpart)
- `internal/component/bgp/reactor/config.go` - config resolution for ingress counterpart
- `internal/component/bgp/yang/ze-bgp-conf.yang` - YANG leaf for ingress counterpart

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation
  -> Decision: peerForwardFacts precomputes per-peer forwarding decisions at session boundaries
  -> Constraint: egress attribute mods use ModAccumulator, not direct wire mutation
- [ ] The SRv6 prefix-SID record (retired with the learned corpus) - SRv6 design decisions
  -> Decision: PrefixSID suppression on NH change uses `mods.Op(40, AttrModSuppress, nil)`
  -> Decision: EBGP filtering via existing attr-discard mechanism (ingress)
  -> Constraint: Ze does not originate local SRv6 SIDs; suppress rather than rebuild

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8669.md` - Section 4/5/8: EBGP propagation rules
  -> Constraint: "propagation to other ASes MUST be explicitly configured" (Section 8)
  -> Constraint: "SHOULD NOT advertise...outside an AS unless explicitly configured" (Section 5.1)
  -> Constraint: "MUST discard the attribute unless configured to accept" (Section 4, ingress)
- [ ] `rfc/short/rfc9252.md` - Section 3.3: propagation rules for SRv6 TLVs
  -> Constraint: strip PrefixSID when next-hop changes (already implemented in applyFactsNextHop)

**Key insights:**
- Ingress (accept) and egress (propagate) are symmetric: both default false, both per-peer session booleans
- The existing NH-change suppression (`applyFactsNextHop`) covers RFC 9252 Section 3.3 but not RFC 8669 Section 8
- EBGP egress suppression is a separate concern that fires based on peer type + config, not NH change
- iBGP peers are unaffected (within SR domain)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - precomputes nhMode, scMask; applies via applyFactsNextHop, applyFactsSendCommunity. PrefixSID suppress only on NH change (line 233).
  -> Constraint: new fact must follow precompute + apply pattern
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - calls applyFactsNextHop then applyFactsSendCommunity in ForwardUpdate per-peer loop
  -> Constraint: insertion point for new applyFactsPrefixSID is after line 517
- [ ] `internal/component/bgp/reactor/forward_rs.go` - same pattern in RS path
  -> Constraint: must add call in both paths
- [ ] `internal/component/bgp/reactor/peer_settings.go` - AcceptSRv6PrefixSID bool field
- [ ] `internal/component/bgp/reactor/config.go` - mapBool(sessionMap, "accept-srv6-prefix-sid")
- [ ] `internal/component/bgp/reactor/session_validation.go` - ingress EBGP filter via DiscardEntries
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - YANG leaf accept-srv6-prefix-sid

**Behavior to preserve:**
- Ingress filtering via `accept-srv6-prefix-sid` (unchanged)
- iBGP propagation of Prefix-SID (within SR domain, unchanged)
- NH-change PrefixSID suppression in `applyFactsNextHop` (RFC 9252 Section 3.3, unchanged)
- Zero-copy forward path via ContextID match (unchanged)

**Behavior to change:**
- Add egress suppression of Prefix-SID on EBGP sessions by default
- Add config knob `propagate-srv6-prefix-sid` to explicitly enable EBGP egress propagation

## Data Flow (MANDATORY)

### Entry Point
- Egress UPDATE building in reactor ForwardUpdate/reactorForwardRS for EBGP peers

### Transformation Path
1. Best-path selected, route has PrefixSID attribute (code 40)
2. Egress UPDATE built for each peer via ForwardUpdate per-peer loop
3. `peerForwardFacts` checked: if `suppressPrefixSID` is true, `mods.Op(40, AttrModSuppress, nil)` added
4. `buildModifiedPayload()` applies accumulated mods, stripping attr 40
5. UPDATE sent to EBGP peer without Prefix-SID

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> PeerSettings | `mapBool(sessionMap, "propagate-srv6-prefix-sid")` at config resolution | [ ] |
| PeerSettings -> peerForwardFacts | precomputed at `refreshForwardFacts()` | [ ] |
| peerForwardFacts -> ModAccumulator | `applyFactsPrefixSID()` emits suppress op | [ ] |

### Integration Points
- `peerForwardFacts` struct (new field `suppressPrefixSID`) - same struct used by NH and community
- `filterapi.ModAccumulator` (same `Op(40, AttrModSuppress, nil)` call) - existing mechanism
- `buildModifiedPayload()` - unchanged, already handles AttrModSuppress for attr 40

### Architectural Verification
- [ ] No bypassed layers (flows through forwarding facts -> mod accumulator -> payload builder)
- [ ] No unintended coupling (self-contained in reactor, config flows through existing resolution)
- [ ] No duplicated functionality (reuses AttrModSuppress, same mechanism as NH-change suppress)
- [ ] Zero-copy preserved where applicable (suppress op modifies in buildModifiedPayload, not on hot path)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `mods.Op(40, AttrModSuppress, nil)` is idempotent -- calling it twice (once from NH change, once from EBGP suppress) produces correct behavior | `applyFactsNextHop` already emits this op; ModAccumulator design | Double suppress could corrupt payload or panic | Unit test: apply both NH-change and EBGP suppress, verify single clean strip | confirmed |
| A-2 | `isEBGP` field in `peerForwardFacts` correctly reflects the peer's AS relationship at egress time | `refreshForwardFacts` sets `isEBGP` from `s.IsEBGP()` (line 107) | Suppress fires on wrong peers | grep/read `IsEBGP()` definition | unvalidated |
| A-3 | The RS path (`forward_rs.go`) uses the same `peerForwardFacts` struct and pattern | Research shows same `applyFactsNextHop`/`applyFactsSendCommunity` calls | RS peers would not get EBGP egress suppression | grep `applyFacts` in `forward_rs.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Double suppress (NH-change + EBGP suppress both fire) causes unexpected behavior | Unit test with both conditions | Test idempotency; if not idempotent, guard with `nhMode != nhModeNone` check |
| R-2 | Breaking existing SRv6 functional tests by changing default behavior | `./le functional` failure on prefix-sid tests | Existing tests use iBGP (same ASN), so should be unaffected |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `propagate-srv6-prefix-sid` config leaf | -> | `PeerSettings.PropagateSRv6PrefixSID` | `TestPrecomputePrefixSIDSuppression` |
| `refreshForwardFacts()` precomputation | -> | `peerForwardFacts.suppressPrefixSID` | `TestPrecomputePrefixSIDSuppression` |
| ForwardUpdate per-peer loop | -> | `applyFactsPrefixSID()` | `test/encode/ebgp-prefix-sid-suppress.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route with Prefix-SID, EBGP peer, no `propagate-srv6-prefix-sid` config (default) | Prefix-SID attribute stripped from egress UPDATE |
| AC-2 | Route with Prefix-SID, EBGP peer, `propagate-srv6-prefix-sid true` | Prefix-SID attribute included in egress UPDATE |
| AC-3 | Route with Prefix-SID, iBGP peer | Prefix-SID attribute included (unchanged behavior) |
| AC-4 | Route with Prefix-SID, EBGP peer with NH-change + suppress (both conditions) | Prefix-SID stripped cleanly (no double-suppress issue) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives SRv6 route from iBGP, re-advertises to EBGP peer (default config) | RIB -> ForwardUpdate -> peerForwardFacts.suppressPrefixSID=true -> mods.Op(40,suppress) -> buildModifiedPayload strips attr 40 | `test/encode/ebgp-prefix-sid-suppress.ci` |
| 2 | Receives SRv6 route from iBGP, re-advertises to EBGP peer with propagation enabled | RIB -> ForwardUpdate -> peerForwardFacts.suppressPrefixSID=false -> no suppress op -> attr 40 preserved | `test/encode/ebgp-prefix-sid-propagate.ci` |
| 3 | Receives SRv6 route from iBGP, re-advertises to iBGP peer | RIB -> ForwardUpdate -> isEBGP=false -> no suppress -> attr 40 preserved | `TestPrecomputePrefixSIDSuppression/ibgp_no_suppress` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrecomputePrefixSIDSuppression` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | EBGP default suppresses, EBGP with propagate does not, iBGP never suppresses | |
| `TestApplyFactsPrefixSID` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | Correct mods.Op emitted when suppressPrefixSID=true, no op when false | |
| `TestPrefixSIDSuppressWithNHChange` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | Both NH-change and EBGP suppress fire cleanly (idempotency) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (boolean config) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ebgp-prefix-sid-suppress` | `test/encode/ebgp-prefix-sid-suppress.ci` | EBGP peer receives UPDATE without Prefix-SID (default suppress) | |
| `ebgp-prefix-sid-propagate` | `test/encode/ebgp-prefix-sid-propagate.ci` | EBGP peer receives UPDATE with Prefix-SID (explicit propagation) | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | Egress attribute suppression is a local policy decision; interop is validated by functional encode tests proving the wire format is correct | |

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - add `leaf propagate-srv6-prefix-sid` after `accept-srv6-prefix-sid`
- `internal/component/bgp/reactor/peer_settings.go` - add `PropagateSRv6PrefixSID bool` after `AcceptSRv6PrefixSID`
- `internal/component/bgp/reactor/config.go` - add `mapBool(sessionMap, "propagate-srv6-prefix-sid")` resolution
- `internal/component/bgp/reactor/peer_forward_facts.go` - add `suppressPrefixSID bool` field, `precomputePrefixSIDSuppression()`, `applyFactsPrefixSID()`
- `internal/component/bgp/reactor/reactor_api_forward.go` - call `applyFactsPrefixSID()` after `applyFactsSendCommunity()`
- `internal/component/bgp/reactor/forward_rs.go` - call `applyFactsPrefixSID()` after `applyFactsSendCommunity()`
- `internal/component/bgp/reactor/peer_forward_facts_test.go` - add unit tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang` |
| YANG validation constraints | No | boolean type has no additional constraints needed |
| YANG custom validators | No | boolean leaf |
| CLI commands/flags | No | config-only, no new CLI commands |
| CLI grammar | No | no CLI commands added |
| Editor autocomplete | No | boolean leaf has automatic true/false completion |
| Functional test | Yes | `test/encode/ebgp-prefix-sid-suppress.ci`, `test/encode/ebgp-prefix-sid-propagate.ci` |
| Pipe completeness | No | no command output |
| Env var registration | No | session-level config, not environment |
| Doctor check | No | no runtime dependencies (file, socket, binary, kernel module) |
| Prometheus counters | No | no observable state (attr suppression is a one-shot decision per UPDATE) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/srv6.md` - add egress suppression knob |
| 2 | Config syntax changed? | Yes | Document new `propagate-srv6-prefix-sid` leaf in SRv6 config section |
| 3 | CLI command added/changed? | No | no CLI changes |
| 4 | API/RPC added/changed? | No | no API changes |
| 5 | Plugin added/changed? | No | reactor-internal change |
| 6 | Has a user guide page? | Yes | `docs/features/srv6.md` - add config example |
| 7 | Wire format changed? | No | wire format unchanged, only attribute presence controlled |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | Note RFC 8669 Section 8 compliance in `docs/features/srv6.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | follows existing pattern exactly |
| 13 | Route metadata keys added? | No | |
| 14 | Prometheus counters added? | No | |
| 15 | Plugin/capability inventory changed? | No | |
| 16 | Changed source file referenced by doc anchors? | Yes | grep docs for source anchors pointing at modified reactor files |
| 17 | Existing docs show config examples for this area? | Yes | verify SRv6 config examples include new leaf |

## Files to Create
- `test/encode/ebgp-prefix-sid-suppress.ci` - functional test: EBGP egress strips Prefix-SID by default
- `test/encode/ebgp-prefix-sid-propagate.ci` - functional test: EBGP egress preserves Prefix-SID when configured

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify-lint run && ./le test-unit  && ./le functional` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- YANG leaf + PeerSettings field + config resolution
   - Tests: `TestPrecomputePrefixSIDSuppression` (fails: field exists but not precomputed)
   - Files: `ze-bgp-conf.yang`, `peer_settings.go`, `config.go`
   - Verify: config parses new leaf; test fails because precomputation not yet implemented

2. **Phase: Precomputation** -- `suppressPrefixSID` field + `precomputePrefixSIDSuppression()`
   - Tests: `TestPrecomputePrefixSIDSuppression` (passes), `TestApplyFactsPrefixSID` (fails: apply fn not written)
   - Files: `peer_forward_facts.go`
   - Verify: precomputation sets field correctly for EBGP/iBGP/propagate combinations

3. **Phase: Apply** -- `applyFactsPrefixSID()` + wiring into both egress pipelines
   - Tests: `TestApplyFactsPrefixSID` (passes), `TestPrefixSIDSuppressWithNHChange` (passes)
   - Files: `peer_forward_facts.go`, `reactor_api_forward.go`, `forward_rs.go`
   - Verify: correct mods emitted; both egress paths call the new function

4. **Functional tests** -- `.ci` tests for egress behavior
   - Files: `test/encode/ebgp-prefix-sid-suppress.ci`, `test/encode/ebgp-prefix-sid-propagate.ci`
   - Verify: `./le functional` passes

5. **RFC refs** -- Add `// RFC 8669 Section 8` comments above enforcing code

6. **Full verification** -- `./le verify current mode full`

7. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-4 each have implementation with file:line |
| Correctness | `suppressPrefixSID` is true only for EBGP peers without explicit propagation config |
| Correctness | `applyFactsPrefixSID` is called in both `reactor_api_forward.go` and `forward_rs.go` |
| Correctness | iBGP peers are never affected (isEBGP=false skips suppression) |
| Idempotency | Double suppress (NH change + EBGP) does not corrupt payload |
| Data flow | Suppression flows through forwarding facts, not validation or filter chain |
| Naming | YANG leaf is `propagate-srv6-prefix-sid`, Go field is `PropagateSRv6PrefixSID` |
| Symmetry | Egress leaf mirrors ingress `accept-srv6-prefix-sid` pattern (default false, boolean, session-level) |
| Rule: no-layering | No duplicate suppression logic; single `applyFactsPrefixSID` function used by both paths |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaf `propagate-srv6-prefix-sid` | `grep 'propagate-srv6-prefix-sid' internal/component/bgp/yang/ze-bgp-conf.yang` |
| `PropagateSRv6PrefixSID` field in PeerSettings | `grep 'PropagateSRv6PrefixSID' internal/component/bgp/reactor/peer_settings.go` |
| Config resolution | `grep 'propagate-srv6-prefix-sid' internal/component/bgp/reactor/config.go` |
| `suppressPrefixSID` field in peerForwardFacts | `grep 'suppressPrefixSID' internal/component/bgp/reactor/peer_forward_facts.go` |
| `applyFactsPrefixSID` function | `grep 'applyFactsPrefixSID' internal/component/bgp/reactor/peer_forward_facts.go` |
| Call in ForwardUpdate | `grep 'applyFactsPrefixSID' internal/component/bgp/reactor/reactor_api_forward.go` |
| Call in RS path | `grep 'applyFactsPrefixSID' internal/component/bgp/reactor/forward_rs.go` |
| Unit tests | `grep 'TestPrecomputePrefixSIDSuppression\|TestApplyFactsPrefixSID\|TestPrefixSIDSuppressWithNHChange' internal/component/bgp/reactor/peer_forward_facts_test.go` |
| Functional test (suppress) | `ls test/encode/ebgp-prefix-sid-suppress.ci` |
| Functional test (propagate) | `ls test/encode/ebgp-prefix-sid-propagate.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Boolean config leaf: YANG enforces type; no additional validation needed |
| Default safety | Default must be `false` (strip), matching RFC 8669 MUST requirement |
| Resource exhaustion | No new allocations; suppress op is a fixed-size struct on the stack |
| Information leakage | No sensitive data exposed; attribute suppression is silent to the peer |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; verify hex expectations match wire format |
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

## Core Insight

Egress attribute suppression for protocol compliance belongs in `peerForwardFacts`
(precomputed, per-peer, checked on every UPDATE), not in the user-configurable filter
chain. The filter chain is for policy; forwarding facts are for protocol requirements.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-peer session boolean over export filter plugin | Export filter plugin in `plugins/filter_*` | RFC 8669 egress stripping is protocol-mandated, not user policy. The ingress counterpart uses a session boolean. Existing egress attribute control (NH, community) uses peerForwardFacts. |
| Default false (strip) over default true (propagate) | Default true (current implicit behavior) | RFC 8669 Section 8 MUST: "propagation to other ASes MUST be explicitly configured." Safe default aligns with ingress side. |
| Suppress entire attr code 40 over per-TLV-type filtering | Strip only SRv6 TLV types 5/6 | RFC 8669 Section 8 refers to "the attribute" wholesale. Ingress discards entire attribute. Symmetry. |

## Known Limitations
- Ze does not originate local SRv6 SIDs, so egress suppress always strips rather than rebuilding with a local SID
- No per-family suppression control (attr 40 stripped for all families if suppress fires)

## RFC Documentation

Add `// RFC 8669 Section 8: "<quoted requirement>"` above enforcing code.
MUST document: the EBGP egress suppression condition, the explicit-configuration override.

## Implementation Summary

### What Was Implemented
- (fill after implementation)

### Bugs Found/Fixed
- (fill after implementation)

### Documentation Updates
- (fill after implementation)

### Deviations from Plan
- (fill after implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| EBGP egress suppresses Prefix-SID by default | Functional test | `test/encode/ebgp-prefix-sid-suppress.ci` |
| Explicit config enables EBGP propagation | Functional test | `test/encode/ebgp-prefix-sid-propagate.ci` |
| iBGP unaffected | Unit test | `TestPrecomputePrefixSIDSuppression/ibgp_no_suppress` |
| RFC 8669 Section 8 compliance | RFC comment + test | `// RFC 8669 Section 8` in `peer_forward_facts.go` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-srv6-ebgp-egress-filter.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-srv6-ebgp-egress-filter.md`
