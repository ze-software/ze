# Spec: srv6-ebgp-egress-filter -- Suppress Prefix-SID on EBGP Egress

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-09-05 |

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
- [ ] `docs/architecture/bgp/structural-forwarding.md` - what left the critical path to close the route-server forwarding gap against BIRD
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
| A-2 | `isEBGP` field in `peerForwardFacts` correctly reflects the peer's AS relationship at egress time | `refreshForwardFacts` sets `isEBGP` from `s.IsEBGP()` (line 107) | Suppress fires on wrong peers | grep/read `IsEBGP()` definition | confirmed -- `PeerSettings.IsEBGP` is `LocalAS != PeerAS` (`peer_settings.go`), read under `p.mu` in `buildForwardFacts` |
| A-3 | The RS path (`forward_rs.go`) uses the same `peerForwardFacts` struct and pattern | Research shows same `applyFactsNextHop`/`applyFactsSendCommunity` calls | RS peers would not get EBGP egress suppression | grep `applyFacts` in `forward_rs.go` | confirmed -- `reactorForwardRS` calls the same `applyFacts*` sequence and takes the new `applyFactsPrefixSID` beside `applyFactsLocalPref` |

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
| ForwardUpdate per-peer loop | -> | `applyFactsPrefixSID()` | `test/plugin/prefixsid-ebgp-egress-boundary.ci` |

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
| 1 | Receives SRv6 route from iBGP, re-advertises to EBGP peer (default config) | RIB -> ForwardUpdate -> peerForwardFacts.suppressPrefixSID=true -> mods.Op(40,suppress) -> buildModifiedPayload strips attr 40 | `test/plugin/prefixsid-ebgp-egress-boundary.ci` |
| 2 | Receives SRv6 route from iBGP, re-advertises to EBGP peer with propagation enabled | RIB -> ForwardUpdate -> peerForwardFacts.suppressPrefixSID=false -> no suppress op -> attr 40 preserved | `test/plugin/prefixsid-ebgp-egress-boundary.ci` |
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
| `ebgp-prefix-sid-suppress` | `test/plugin/prefixsid-ebgp-egress-boundary.ci` | EBGP peer receives UPDATE without Prefix-SID (default suppress) | |
| `ebgp-prefix-sid-propagate` | `test/plugin/prefixsid-ebgp-egress-boundary.ci` | EBGP peer receives UPDATE with Prefix-SID (explicit propagation) | |

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
| Functional test | Yes | `test/plugin/prefixsid-ebgp-egress-boundary.ci` (one run, two destinations; see Deviations 3) |
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
- `test/plugin/prefixsid-ebgp-egress-boundary.ci` - functional test: one run, two eBGP destinations, opposite outcomes (strip by default, keep when configured). See Deviations 3 for why this replaced the two planned `test/encode/` files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` |
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
   - Files: `test/plugin/prefixsid-ebgp-egress-boundary.ci` (see Deviations 3)
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
| Unit tests | `grep 'TestPrefixSIDEgressBoundary\|TestPrefixSIDAllowedTo\|TestPrefixSIDSuppressIsRecordedOnce\|TestPrefixSIDOriginationBoundary' internal/component/bgp/reactor/forward_prefix_sid_test.go` |
| Functional test (both polarities) | `ls test/plugin/prefixsid-ebgp-egress-boundary.ci` |

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
| Gating four of five rails was enough to record `RFC8669-8-1` as met | A MUST is met or it is not. The fifth rail propagates to another AS with no explicit configuration, so the requirement is unmet and the ledger said otherwise | Closure read `buildBatchAnnounceUpdate` at the producer, then measured it: the rail returns attribute 40 toward an external destination | The public ledger claimed a proven MUST for one week. `{gap}` restored, `docs/features/srv6.md` corrected |
| A page that names four rails is accurate enough | `docs/features/srv6.md` said "every UPDATE sent to an EBGP peer", which is the universal claim, then the RFC table said `Implemented`. A reader takes the row, not the prose around it | The same read | Two rows corrected in the closure commit |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Gate the announce rail without widening `buildBatchAnnounceUpdate` | With update groups enabled one built UPDATE is shared by every peer of one `announceBuildKey`, so a destination-scoped strip outside the key applies to the wrong peers. A post-build strip also re-adds the memmove the announce writer exists to remove | The bool joins the key AND the parameter list, which is what makes the edit reach an RFC-tagged test |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A spec closes its own rails and records the RFC requirement as met, when a rail it did not enumerate still emits the attribute | Second occurrence in this package: the same shape produced the LOCAL_PREF disagreement `localPrefAllowedTo` was written to end | Before recording a MUST as met, enumerate every producer that WRITES the artifact the requirement governs, not every producer the spec planned to change | Journal row written: `plan/journal/rail-enumeration-incomplete.md` |

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

`prefixSIDAllowedTo(isIBGP, propagate)` in `internal/component/bgp/reactor/forward_prefix_sid.go`
is the single site that answers RFC 8669 Section 8 for one destination. Every egress rail asks
there, so no two rails can disagree, which is the shape `localPrefAllowedTo` already set for
RFC 4271 Section 5.1.5.

| Rail | Producer | What it does now |
|------|----------|------------------|
| General forward | `forwardUpdateCore` (`reactor_api_forward.go`) | `applyFactsPrefixSID` records an attribute suppression for a destination Section 8 refuses |
| Route server | `reactorForwardRS` (`forward_rs.go`) | the same call. This is the rail the defect left open: an RS client keeps the source next-hop, so `applyFactsNextHop` returned at `nhModeNone` and nothing removed code 40 |
| Static-route origination | `buildStaticRouteUpdateNew` (`peer_static_routes.go`) | drops the configured `PrefixSIDBytes` and any raw attribute under code 40, for unicast, labeled unicast and VPN |
| Plugin-route origination | `toPluginParams` (`peer_static_routes.go`) | drops a plugin-supplied raw attribute under code 40 |

Config: YANG leaf `propagate-srv6-prefix-sid` (default `false`) under `bgp/peer/session`,
resolved into `PeerSettings.PropagateSRv6PrefixSID` and carried into
`peerForwardFacts.propagatePrefixSID`.

### Bugs Found/Fixed

- The defect the spec names: `applyFactsNextHop` returned at `nhModeNone`, so every eBGP peer
  that keeps the next hop kept the Prefix-SID. A route-server client always keeps it.
- Three egress rails the spec did not name carry attribute 40 and had no gate at all: the two
  origination rails above, and the readvertise rail (see Deviations).
- The raw `attribute` leaf-list is an escape hatch for any type code, and
  `config.parseRawAttributeInto` leaves code 40 raw, so a hand-written Prefix-SID reached the
  wire by a path the modeled field never touched.

### Documentation Updates

- `docs/features/srv6.md`: the new leaf, why the boundary is configured rather than derived, and
  the egress decision in the data-flow section.
- `docs/features/rfc-status.md`: RFC 8669 gap count Eleven -> Ten, RFC8669-8-1 removed from the
  Remaining cell, the Section 8 egress gate stated in the Support cell.
- `rfc/short/rfc8669.md`: the `{gap}` annotation on RFC8669-8-1 removed.

### Deviations from Plan

| # | Plan | Actual | Why |
|---|------|--------|-----|
| 1 | Code in `peer_forward_facts.go` | New file `forward_prefix_sid.go` | `forward_local_pref.go` and `forward_med.go` are the established one-file-per-egress-concern pattern, and this concern now spans four rails |
| 2 | Two rails gated | Four gated | The spec named only the forward rails. Attribute 40 also reaches the wire from both origination rails |
| 3 | `test/encode/ebgp-prefix-sid-{suppress,propagate}.ci` | One `test/plugin/prefixsid-ebgp-egress-boundary.ci` | The encode suite has one peer, so it cannot exercise a forward rail. One run with two destinations and opposite outcomes is what makes each assertion discriminate |
| 4 | Suppression precomputed as a `suppressPrefixSID` fact | `propagatePrefixSID` fact, decision taken in `applyFactsPrefixSID` | The decision also needs the operations recorded so far, so that the RFC 9252 next-hop suppression and this one never record two operations for one code |
| 5 | -- | The readvertise rail was left ungated by this spec | Handed to `spec-prefix-sid-announce-rail-boundary`, which fixed it on 2026-09-05. See below |

### Handed on and now fixed: the readvertise announce rail

`buildBatchAnnounceUpdate` (`reactor_api_batch.go`) copied a stored or relayed attribute block
verbatim and had no code-40 handling, so an iBGP-learned Prefix-SID still reached an external
peer on that rail. The fix is the same one-line `plan.drop` the LOCAL_PREF branch beside it
already uses, plus one per-destination bool on the function and on `announceBuildKey`.

Adding that parameter mechanically edits `TestAnnounceStripsLocalPrefTowardExternalPeer`
(`reactor_api_origin_test.go`), which carries `RFC requirement: RFC4271-5.1.5-1/-2`. The write
hook refuses the edit without an owner approval row in `test/rfc-changed.md`. This spec's session
was instructed not to owe a third such row, so the rail was left for the owner to answer.

**Closure measured it rather than inferring it (2026-09-05).**
`TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`
(`internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go`) drives
`buildBatchAnnounceUpdate` with `isIBGP=false` and a base carrying attribute 40. At that HEAD the
rail emitted `c0280a01000700000000000064` toward the external destination, and the test was RED.

The key field is not optional either: with update groups enabled one built UPDATE is shared by
every peer of one `announceBuildKey`, so a destination-scoped strip outside the key would apply
to the wrong peers.

**The owner answered on 2026-09-05 and the rail is gated (`4054ed854`).** The remainder was owned
by `spec-prefix-sid-announce-rail-boundary`, which is closed. The five rails now ask
`prefixSIDAllowedTo`, the leaf is a field of `announceBuildKey`, `RFC8669-8-1` carries no `{gap}`
in `rfc/short/rfc8669.md` (Meta count back to ten), and `docs/features/srv6.md` says every rail
that writes an UPDATE asks.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Default is "do not propagate" on every eBGP egress rail | Done | `prefixSIDAllowedTo` (`internal/component/bgp/reactor/forward_prefix_sid.go`) | `false` unless the peer is internal or the leaf is set |
| Explicit per-peer configuration permits propagation | Done | YANG `propagate-srv6-prefix-sid`, `PeerSettings.PropagateSRv6PrefixSID`, `reactor/config.go` | |
| iBGP is untouched | Done | `prefixSIDAllowedTo` returns true for `isIBGP` | Proven by `TestPrefixSIDEgressBoundary/ibgp_keeps_it` |
| Every rail that can emit attribute 40 is gated | Done | four rails here, the fifth in `4054ed854` | This spec gated four of five. The API/readvertise announce rail was measured red at HEAD by `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain`, homed at `spec-prefix-sid-announce-rail-boundary`, and gated there once the owner wrote the `test/rfc-changed.md` approval row. All five now ask `prefixSIDAllowedTo` |
| RFC 8669 Section 8 cited above the enforcing code | Done | `forward_prefix_sid.go`, `peer_static_routes.go`, `peer_initial_sync.go` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPrefixSIDEgressBoundary/ebgp_without_configuration_is_stripped`, `test/plugin/prefixsid-ebgp-egress-boundary.ci` conn=2 | Asserted on the destination's own bytes |
| AC-2 | Done | `TestPrefixSIDEgressBoundary/ebgp_configured_for_propagation_keeps_it`, the same `.ci` conn=3 | Byte-identical to the source frame |
| AC-3 | Done | `TestPrefixSIDEgressBoundary/ibgp_keeps_it` | |
| AC-4 | Done | `TestPrefixSIDSuppressIsRecordedOnce` | Exactly one operation on code 40 whether or not the next hop changes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPrecomputePrefixSIDSuppression` | Changed | `TestPrefixSIDEgressBoundary` (`forward_prefix_sid_test.go`) | Drives `reactorForwardRS` end to end instead of the precomputation alone, so the evidence is the wire rather than a struct field |
| `TestApplyFactsPrefixSID` | Changed | `TestPrefixSIDAllowedTo` and `TestPrefixSIDSuppressIsRecordedOnce` | Split into the rule and the operation |
| `TestPrefixSIDSuppressWithNHChange` | Done | `TestPrefixSIDSuppressIsRecordedOnce/next-hop_self_still_records_exactly_one` | |
| `ebgp-prefix-sid-suppress` / `ebgp-prefix-sid-propagate` | Changed | `test/plugin/prefixsid-ebgp-egress-boundary.ci` | One run, two destinations; see Deviations 3 |
| (added) `TestPrefixSIDOriginationBoundary` | Done | `forward_prefix_sid_test.go` | The two origination rails the spec did not name |
| (added) `TestRawAttrsWithoutPrefixSID` | Done | `forward_prefix_sid_test.go` | The raw-attribute filter's edges |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | |
| `internal/component/bgp/reactor/peer_settings.go` | Done | |
| `internal/component/bgp/reactor/config.go` | Done | |
| `internal/component/bgp/reactor/peer_forward_facts.go` | Done | Field only; the logic went to `forward_prefix_sid.go` |
| `internal/component/bgp/reactor/reactor_api_forward.go` | Done | |
| `internal/component/bgp/reactor/forward_rs.go` | Done | |
| `internal/component/bgp/reactor/peer_forward_facts_test.go` | Changed | New tests went to `forward_prefix_sid_test.go`, beside the code they cover |
| (added) `internal/component/bgp/reactor/forward_prefix_sid.go` | Done | |
| (added) `internal/component/bgp/reactor/forward_local_pref.go` | Done | `payloadHasAttr` extracted from `payloadHasLocalPref` |
| (added) `internal/component/bgp/reactor/peer_static_routes.go`, `peer_initial_sync.go` | Done | The origination rails |
| (added) `internal/test/fixture/plugin_fixture_04.go` | Done | Observer for the new `.ci` |
| `test/encode/ebgp-prefix-sid-*.ci` | Changed | Replaced by `test/plugin/prefixsid-ebgp-egress-boundary.ci` |

### Audit Summary
- **Total items:** 27
- **Done:** 21
- **Partial:** 1 (rail coverage: four of five)
- **Skipped:** 0
- **Changed:** 5

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| EBGP egress suppresses Prefix-SID by default | Functional test | `test/plugin/prefixsid-ebgp-egress-boundary.ci` conn=2 expects a 47-octet frame with no attribute 40; the run passed at 517/705 in `./le functional plugin` |
| Explicit config enables EBGP propagation | Functional test | The same `.ci` conn=3 expects the 60-octet source frame byte for byte |
| iBGP unaffected | Unit test | `TestPrefixSIDEgressBoundary/ibgp_keeps_it`, over the real `reactorForwardRS` rail |
| The route-server rail, which the old code missed | Unit test | `TestPrefixSIDEgressBoundary/route-server_client_without_configuration_is_stripped` and its configured twin |
| Each assertion discriminates | Mutation | `prefixSIDAllowedTo -> true` reddens both strip cases; `-> isIBGP` reddens both keep cases; removing the origination strip reddens all four of `TestPrefixSIDOriginationBoundary` |
| RFC 8669 Section 8 compliance | RFC gate | NOT achieved. `./le rfc check` binds RFC8669-8-1 to a positive and a negative tag (`rfc/requirements/rfc8669.md`), but those tags cover the four gated rails only. Closure restored the `{gap}` annotation on `RFC8669-8-1` (`rfc/short/rfc8669.md`) and the Meta count ten -> eleven, because the fifth rail propagates to another AS with no explicit configuration. RFC 8669 Section 8, `rfc/full/rfc8669.txt`: "The propagation to other ASes MUST be explicitly configured." |
| Each tagged unit discriminates by RECORD | Not achieved | `./le rfc discriminate stem rfc8669` lists all 29 tags as `unproven` and `rfc/discrimination/rfc8669.json` does not exist. The six tags this spec added owe a `./le rfc discriminate-record` run. The hand-run mutations in the row above were observed by the implementation session and are not the machine artifact. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The API/readvertise announce rail is not gated, so attribute 40 crosses an AS boundary there with no leaf set | The per-destination bool must join `announceBuildKey` and the parameter list of `buildBatchAnnounceUpdate`, which mechanically edits `TestAnnounceStripsLocalPrefTowardExternalPeer`, an `RFC requirement: RFC4271-5.1.5` carrier. `test/rfc-changed.md` reserves that approval to the owner | RESOLVED. `spec-prefix-sid-announce-rail-boundary` owned it and closed on 2026-09-05: the owner wrote the approval row, `4054ed854` gated the rail, and all five rails now ask `prefixSIDAllowedTo` |
| No discrimination record for the six `RFC8669-8-1` tags this spec added | `rfc/discrimination/rfc8669.json` does not exist and all 29 of the RFC's tags read `unproven`. The gate's change-scoped window (a tag new against `HEAD^`) closed when the implementation commit landed without one | RESOLVED in this spec's own closure, below: `rfc/discrimination/rfc8669.json` now carries a recorded red for each of the six |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not recorded: the gate cannot be declared CLEAN while a BLOCKER stands |
| `./le commit review-check` | not run, for the same reason |
| Rounds | 1 |
| Reviewer lenses used | wiring + functional-test coverage, logic + guard audit + RFC compliance, documentation drift + style pass |

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The API/readvertise announce rail emits attribute 40 toward an external peer with no explicit configuration. RFC 8669 Section 8 MUST. Measured: the rail returns `c0280a01000700000000000064` | `buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go`), reached by the API announce, the grouped announce and `sendStaleReadvertise` | Homed at `spec-prefix-sid-announce-rail-boundary`, which needed an owner approval row in `test/rfc-changed.md`. FIXED there on 2026-09-05 in `4054ed854`; the rail now asks `prefixSIDAllowedTo` |
| 2 | ISSUE | `docs/features/srv6.md` claimed the removal happened on "every UPDATE sent to an EBGP peer", and its RFC 8669 table read `Implemented`. Both are wider than the code. `ai/rules/rfc-compliance.md`: a claim wider than the assertion converts an unproven MUST into a proven one | `docs/features/srv6.md`, the EBGP-propagation row and the RFC 8669 Section 8 row | FIXED. Both rows name the four gated rails and the fifth; a "Gap" section names the producing function |
| 3 | ISSUE | `RFC8669-8-1` carried no `{gap}`, so the generated ledger reported the MUST as met | `rfc/short/rfc8669.md`, the requirement line and the Meta `Support coverage` and `Support remaining` rows | FIXED. `{gap}` restored, count ten -> eleven, regenerated with `./le rfc index-update` |
| 4 | ISSUE | Six `RFC8669-8-1` tags were added with no discrimination record | `rfc/discrimination/rfc8669.json` absent; `./le rfc discriminate stem rfc8669` lists all 29 tags `unproven` | NOT FIXED. Recorded in Work Not Done |
| 5 | NOTE | The spec header read `Phase 1/6` with every closure section filled, and five sections named `.ci` files under names the implementation replaced | this file | FIXED |

### Fixes applied

Findings 2, 3 and 5 are fixed in the closure commit. Finding 1 is the blocker and
finding 4 rides with it: both are homed at the new spec, and both stay on the
public ledger until the behavior exists (`ai/rules/rfc-compliance.md`).

Wiring: `prefixSIDAllowedTo`, `Peer.prefixSIDAllowed`, `applyFactsPrefixSID`,
`prefixSIDOnWire` and `rawAttrsWithoutPrefixSID` each have a production caller
outside `forward_prefix_sid.go`: `reactorForwardRS` (`forward_rs.go`),
`forwardUpdateCore` (`reactor_api_forward.go`), `toPluginParams`,
`toStaticRouteUnicastParams` and `toStaticRouteLabeledUnicastParams`
(`peer_static_routes.go`), and `sendStaticRoutes` and `sendPluginRoutesVia`
(`peer_initial_sync.go`). No unwired symbol.

Ordering: on both forward rails the call sits after the egress filter pass and
after `applyFactsNextHop`, so `filterapi.LastSetOrSuppress` makes the
prohibition win over a filter's Set. Read at both call sites.

Idempotency: `prefixSIDOnWire` folds the operations already recorded, so the
RFC 9252 Section 3.3 next-hop suppression and this one never record two
operations for code 40. That is AC-4.

Style pass: no `panic()` in the diff, no unbounded loop, no new lifecycle pair,
`payloadHasAttr` generalizes `payloadHasLocalPref` rather than copying it, and
every new function returns the narrowest type it can. No style finding.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

No run 2. A second pass cannot clear finding 1: the fix is refused by a gate only
the owner can open, so another round would re-find the same BLOCKER.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE -- NO. One BLOCKER stands.
- [ ] All NOTEs recorded above (or explicitly "none") -- one NOTE, finding 5, fixed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_prefix_sid.go` | Yes | read in full at closure; carries `prefixSIDAllowedTo`, `prefixSIDAllowed`, `prefixSIDOnWire`, `applyFactsPrefixSID`, `rawAttrsWithoutPrefixSID`, `isRawPrefixSID` |
| `internal/component/bgp/reactor/forward_prefix_sid_test.go` | Yes | 365 lines in `f3379e684`; carries `TestPrefixSIDAllowedTo`, `TestPrefixSIDEgressBoundary`, `TestPrefixSIDSuppressIsRecordedOnce`, `TestPrefixSIDOriginationBoundary`, `TestRawAttrsWithoutPrefixSID` |
| `test/plugin/prefixsid-ebgp-egress-boundary.ci` | Yes | 227 lines in `f3379e684`; `option=tcp_connections:value=3`, conn=1 source, conn=2 strip, conn=3 keep |
| `internal/component/bgp/reactor/forward_prefix_sid_announce_rail_test.go` | Yes | added at closure; RED then, and that was the point. Green since `4054ed854` gated the rail |
| the spec that owned the remainder | Yes | `spec-prefix-sid-announce-rail-boundary`, written at this spec's closure and itself closed on 2026-09-05 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | eBGP, leaf unset -> stripped | `prefixSIDAllowedTo(false, false)` is `false` (`forward_prefix_sid.go`), so `applyFactsPrefixSID` records the Suppress. The `.ci` conn=2 expects a 47-octet frame. On the forward rails only |
| AC-2 | eBGP, leaf true -> kept | `prefixSIDAllowedTo(false, true)` is `true`, so the function returns before recording. The `.ci` conn=3 expects the 60-octet source frame |
| AC-3 | iBGP -> kept | `applyFactsPrefixSID` asks `prefixSIDAllowedTo(!f.isEBGP, ...)`, true for an internal peer whatever the leaf says |
| AC-4 | next-hop change plus the boundary -> one operation | `prefixSIDOnWire` folds `mods.Ops()` and returns false once `applyFactsNextHop` recorded its RFC 9252 Suppress, so the second is not recorded |
| (rail coverage) | every rail gated | FALSE for the announce rail. `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` is red |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| eBGP source relays a Prefix-SID to two eBGP destinations | `test/plugin/prefixsid-ebgp-egress-boundary.ci` | Yes. Read in full: `rs-fast-path` is deliberately ABSENT, so the run drives `forwardUpdateCore` rather than the rail the unit tests drive; `accept-srv6-prefix-sid` on the source keeps the RFC 8669 Section 4 ingress discard from making the negative vacuous; `conn_map:value=remote-ip` binds each receiver by identity rather than by accept order |
| YANG leaf -> `PeerSettings` -> facts | -- | `leaf propagate-srv6-prefix-sid` (`ze-bgp-conf.yang`) -> `mapBool(sessionMap, "propagate-srv6-prefix-sid")` (`config.go`) -> `PeerSettings.PropagateSRv6PrefixSID` -> `buildForwardFacts` sets `peerForwardFacts.propagatePrefixSID` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, and superseded | The double-suppress question was answered by not recording a second operation at all: `prefixSIDOnWire` reads the fold before `applyFactsPrefixSID` records anything |
| A-2 | confirmed | `peerForwardFacts.isEBGP` is set in `buildForwardFacts` from `PeerSettings.IsEBGP` (`LocalAS != PeerAS`), read under `p.mu` |
| A-3 | confirmed | `reactorForwardRS` calls `applyFactsPrefixSID` in the same sequence as `applyFactsLocalPref` and `applyFactsMED` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| New user-facing feature, config syntax, user guide | `docs/features/srv6.md` carries the leaf, the config example, the option table and the egress data flow | Yes, and CORRECTED at closure: two rows overclaimed universal coverage |
| RFC behavior implemented | `rfc/short/rfc8669.md` `RFC8669-8-1` now carries a `{gap}`; Meta count ten -> eleven; `docs/features/rfc-status.md` and `rfc/requirements/rfc8669.md` regenerated by `./le rfc index-update` | Yes |
| Changed source file referenced by doc anchors | `docs/architecture/core-design.md`, `docs/comparison.md` and `docs/architecture/api/architecture.md` anchor `reactor_api_forward.go` and `forward_rs.go`, naming the hoisted accumulator, the egress filter chain, `reactorForwardRS` and `ForwardUpdate`. The diff added a call inside those functions and changed nothing the anchors claim | Yes, no edit owed |
| `./le doc check verify` | exit 1, 3482 drift findings; none names a file this spec touched (grepped for `srv6`, `prefix-sid`, `8669`) | Foreign |
| CLI, API/RPC, plugin, wire format, metrics, doctor | No new command, RPC, event, send type, metric or runtime dependency; the change is one YANG boolean and reactor-internal code | No update owed |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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
- [ ] **Commit B:** `git rm plan/immediate/spec-srv6-ebgp-egress-filter.md`
