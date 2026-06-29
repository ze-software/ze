# Spec: SR-Policy NLRI (SAFI 73) with Tunnel Encapsulation Attribute and ExaBGP Bridge

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | implement |
| Updated | 2026-06-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9830.md` - SR-Policy NLRI wire format (SAFI 73)
4. `rfc/short/rfc9012.md` - Tunnel Encapsulation attribute (code 23)
5. `rfc/short/rfc9256.md` - SR-Policy architecture (concepts)
6. `docs/architecture/wire/nlri.md` - NLRI type hierarchy and patterns
7. `internal/core/bgp/nlri/nlrisplit/nlrisplit.go` - splitter registry
8. `internal/exabgp/bridge/bridge.go` - ExaBGP bridge family map
9. `internal/core/bgp/attribute/attribute.go` - attribute code constants

## Task

Add full SR-Policy (SAFI 73, RFC 9830) support to Ze:
1. Register ipv4/sr-policy and ipv6/sr-policy families
2. Implement SR-Policy NLRI type and splitter
3. Add Tunnel Encapsulation attribute (code 23, RFC 9012) with SR-Policy sub-TLV parsing
4. Update ExaBGP bridge to support sr-policy family declaration, command translation, and event forwarding

This enables Ze to negotiate, decode, encode, and forward SR-Policy UPDATEs, and allows ExaBGP plugins running through the bridge to announce/withdraw SR-Policy routes. Motivated by ExaBGP PR 1393 which adds SR-Policy to ExaBGP.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri.md` - NLRI type hierarchy, encoding patterns, splitter registry
- [ ] `docs/architecture/wire/attributes.md` - attribute encoding, lazy parsing, OpaqueAttribute
- [ ] `ai/patterns/registration.md` - plugin registration pattern
- [ ] `ai/rules/plugin-design.md` - plugin placement, cross-boundary types
- [ ] `ai/rules/buffer-first.md` - wire encoding must use WriteTo pattern

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9830.md` - SR-Policy NLRI wire format, Tunnel Type 15 sub-TLVs
- [ ] `rfc/short/rfc9012.md` - Tunnel Encapsulation attribute (code 23), TLV/sub-TLV framing
- [ ] `rfc/short/rfc9256.md` - SR-Policy architecture, segment type taxonomy
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI / MP_UNREACH_NLRI carrying SAFI 73

**Key insights:**
- SR-Policy NLRI is fixed-size per AFI (12 bytes IPv4, 24 bytes IPv6) with 1-byte bit-count length prefix
- Tunnel Encap attribute (code 23) is Optional Transitive; Ze already forwards unknown attrs as OpaqueAttribute with Partial bit
- Sub-TLV length encoding bifurcates at type 128: types 0-127 use 1-byte length, 128-255 use 2-byte length
- ExaBGP bridge has hardcoded family map in parseFamilyToAFISAFI and regex in convertAnnounceFamily
- SR-Policy command syntax differs fundamentally from prefix-based routes (no CIDR prefix, uses distinguisher/color/endpoint)
- Ze's NLRI type hierarchy has standalone types (MUP, EVPN) and embedded types (PrefixNLRI); SR-Policy fits the standalone pattern
- Ze's splitter registry is family-keyed; SR-Policy needs its own splitter registered for both ipv4/sr-policy and ipv6/sr-policy
- ADD-PATH is not used with SR-Policy per RFC 9830

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/family/family.go` - SAFI constants
- [ ] `internal/core/family/registry.go` - family registration via MustRegister
- [ ] `internal/core/bgp/nlri/nlrisplit/nlrisplit.go` - splitter registry
- [ ] `internal/core/bgp/nlri/nlrisplit/register.go` - existing splitter registrations
- [ ] `internal/core/bgp/attribute/attribute.go` - attribute codes
- [ ] `internal/core/bgp/attribute/opaque.go` - OpaqueAttribute for unknown attributes
- [ ] `internal/component/bgp/plugins/nlri/mup/` - reference NLRI plugin
- [ ] `internal/exabgp/bridge/bridge.go` - parseFamilyToAFISAFI, ValidateFamily
- [ ] `internal/exabgp/bridge/bridge_command.go` - ExaBGP command translation
- [ ] `internal/exabgp/bridge/bridge_event.go` - ZeBGP to ExaBGP JSON translation

**Behavior to preserve:**
- Existing family registrations and SAFI constants unchanged
- Existing splitter registrations for CIDR, EVPN, Labeled unchanged
- ExaBGP bridge parseFamilyToAFISAFI existing cases unchanged
- ExaBGP bridge convertAnnounce/convertWithdraw for prefix-based routes unchanged
- OpaqueAttribute passthrough for unknown attributes still works (code 23 gets parsed instead of opaque when present)
- NLRI interface contract: Family(), Bytes(), Len(), String(), PathID(), WriteTo(), SupportsAddPath()

**Behavior to change:**
- Add SAFISRPolicy=73 to family constants (already done)
- Register ipv4/sr-policy and ipv6/sr-policy families via MustRegister
- Add SR-Policy NLRI type implementing NLRI interface
- Add SR-Policy splitter to nlrisplit registry
- Add AttrTunnelEncap=23 to attribute code constants
- Add TunnelEncap attribute type with TLV/sub-TLV parsing for SR-Policy (Tunnel Type 15)
- Add sr-policy to bridge parseFamilyToAFISAFI
- Add sr-policy to bridge convertAnnounceFamily/convertWithdrawFamily regex
- Add SR-Policy command parser to bridge (distinguisher/color/endpoint syntax)

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes: BGP UPDATE with MP_REACH_NLRI/MP_UNREACH_NLRI for AFI 1|2, SAFI 73
- Bridge command: ExaBGP plugin sends "neighbor X announce ipv4 sr-policy distinguisher D color C endpoint E next-hop NH ..."
- Bridge event: Ze sends JSON event with nlri under "ipv4/sr-policy" key

### Transformation Path
1. Wire bytes arrive in UPDATE message, MP_REACH_NLRI attribute parsed by mpnlri.go
2. NLRI bytes extracted per family, dispatched to nlrisplit.SplitSRPolicy via splitter registry
3. Each NLRI slice parsed into SRPolicy struct (distinguisher, color, endpoint)
4. Tunnel Encapsulation attribute (code 23) parsed by wire.go lazy parser into TunnelEncap
5. TunnelEncap contains TLV(s); Tunnel Type 15 parsed into SRPolicyTunnel with sub-TLVs
6. For forwarding: NLRI re-encoded via WriteTo, attribute re-encoded (or forwarded as-is if context matches)
7. Bridge path: ExaBGP text command parsed by bridge_command.go, translated to Ze command with nlri/attr fields

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> NLRI struct | nlrisplit.Split + srpolicy.Parse | [ ] |
| Wire -> Attribute struct | attribute.wire.go lazy parse | [ ] |
| Ze engine -> ExaBGP bridge | JSON events via MuxConn deliver-batch | [ ] |
| ExaBGP bridge -> Ze engine | Text commands via MuxConn dispatch-command | [ ] |

### Integration Points
- nlrisplit.Register: SR-Policy splitter plugs into existing registry
- attribute.AttributeCode: AttrTunnelEncap=23 adds to existing constant list
- bridge.parseFamilyToAFISAFI: sr-policy case added to existing switch
- bridge.convertAnnounceFamily: sr-policy handling added to existing regex/dispatch

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | OpaqueAttribute preserves unknown attrs with Partial bit for forwarding | opaque.go review | Attr 23 silently dropped on forward | grep OpaqueAttribute usage in wire.go forward path | unvalidated |
| A-2 | SR-Policy does not use ADD-PATH | RFC 9830 Section 2.1, no path-id mention | Splitter would need addPath=true handling | confirm with RFC 9830 summary | unvalidated |
| A-3 | ParseMPReachNLRI extracts NLRI bytes generically for any AFI/SAFI; splitting happens downstream via nlrisplit registry | mpnlri.go:208 ParseMPReachNLRI | If mpnlri.go filters by SAFI, SR-Policy NLRIs would be dropped | confirmed by reading mpnlri.go:208-244 | confirmed |
| A-4 | ExaBGP bridge JSON event format for non-prefix NLRIs passes objects through | bridge_event.go line 276 else branch | Would need bridge event translation changes | test with SR-Policy JSON event | unvalidated |
| A-5 | ExaBGP PR 1393 command syntax matches what the bridge needs to generate | PR review of configuration/static/sr_policy.py | Bridge generates wrong commands | compare bridge output with ExaBGP parser expectations | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | mpnlri.go has hardcoded family dispatch that skips unknown SAFIs | Compile-time: SR-Policy NLRIs not parsed | Add SAFI 73 case to mpnlri.go if needed |
| R-2 | Tunnel Encap attribute parsing conflicts with wire.go lazy parse model | AttrTunnelEncap not found by wire.Get(23) | Register code 23 in attribute parse dispatch table |
| R-3 | Bridge SR-Policy command syntax diverges from Ze's internal command format | Bridge integration test fails on round-trip | Design bridge command format to match Ze's peer update text syntax |
| R-4 | Segment types C-K from RFC 9831 not fully covered in first pass | ExaBGP configs using types C-K fail | Start with types A and B (RFC 9830), add C-K incrementally |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| family.LookupFamily("ipv4/sr-policy") | -> | family.MustRegister in srpolicy/types.go | TestSRPolicyFamilyRegistered |
| nlrisplit.Split(ipv4SRPolicy, wireBytes, false) | -> | srpolicy.SplitSRPolicy | TestSRPolicySplitRegistered |
| bridge.parseFamilyToAFISAFI("ipv4/sr-policy") | -> | sr-policy case in bridge.go | TestBridgeSRPolicyFamilyParse |
| bridge.ValidateFamily("ipv4/sr-policy") | -> | parseFamilyToAFISAFI | TestBridgeSRPolicyValidation |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | family.LookupFamily("ipv4/sr-policy") and "ipv6/sr-policy" | Returns valid Family with AFI 1/2, SAFI 73 |
| AC-2 | Wire bytes: length-bit-prefix + 12-byte IPv4 SR-Policy NLRI body | SplitSRPolicy returns one NLRI slice of 13 bytes |
| AC-3 | Wire bytes: two concatenated IPv4 SR-Policy NLRIs | SplitSRPolicy returns two NLRI slices |
| AC-4 | srpolicy.Parse(AFIIPv4, 12-byte body) | Returns SRPolicy with correct distinguisher, color, endpoint |
| AC-5 | srpolicy.Parse(AFIIPv6, 24-byte body) | Returns SRPolicy with correct distinguisher, color, IPv6 endpoint |
| AC-6 | SRPolicy.WriteTo round-trips with Parse | Encoded bytes match original wire bytes |
| AC-7 | SRPolicy.Family() | Returns {AFI: 1, SAFI: 73} for IPv4, {AFI: 2, SAFI: 73} for IPv6 |
| AC-8 | bridge.parseFamilyToAFISAFI("ipv4/sr-policy") | Returns AFI=1, SAFI=73 |
| AC-9 | bridge.parseFamilyToAFISAFI("ipv6/sr-policy") | Returns AFI=2, SAFI=73 |
| AC-10 | bridge.ValidateFamily("ipv4/sr-policy") | Returns nil (valid) |
| AC-11 | ExaBGP command "neighbor X announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4" | Bridge translates to Ze command format |
| AC-12 | Truncated SR-Policy NLRI wire bytes | SplitSRPolicy returns error |
| AC-13 | AttrTunnelEncap constant equals 23 | Attribute code registered |
| AC-14 | Tunnel Encap wire bytes with Tunnel Type 15 and Preference sub-TLV | TunnelTLV.SubTLVs() parses on demand; TunnelTLV.Preference() returns uint32 value |
| AC-15 | Tunnel Encap wire bytes with unknown tunnel type | Preserved as opaque TLV for forwarding |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSRPolicyFamilyRegistered | srpolicy/types_test.go | AC-1: ipv4/sr-policy and ipv6/sr-policy registered | |
| TestSRPolicySplitIPv4Single | srpolicy/split_test.go | AC-2: single IPv4 NLRI split | |
| TestSRPolicySplitIPv4Multiple | srpolicy/split_test.go | AC-3: multiple IPv4 NLRIs split | |
| TestSRPolicyParseIPv4 | srpolicy/types_test.go | AC-4: parse IPv4 NLRI body | |
| TestSRPolicyParseIPv6 | srpolicy/types_test.go | AC-5: parse IPv6 NLRI body | |
| TestSRPolicyRoundTrip | srpolicy/types_test.go | AC-6: WriteTo + Parse round-trip | |
| TestSRPolicyFamily | srpolicy/types_test.go | AC-7: Family() returns correct AFI/SAFI | |
| TestSRPolicySplitTruncated | srpolicy/split_test.go | AC-12: truncated data returns error | |
| TestSRPolicySplitRegistered | srpolicy/split_test.go | wiring: splitter registered in nlrisplit | |
| TestBridgeSRPolicyFamilyParse | bridge/bridge_test.go | AC-8, AC-9: parseFamilyToAFISAFI for sr-policy | |
| TestBridgeSRPolicyValidation | bridge/bridge_test.go | AC-10: ValidateFamily for sr-policy | |
| TestBridgeSRPolicyCommand | bridge/bridge_test.go | AC-11: ExaBGP sr-policy command translation | |
| TestAttrTunnelEncapConstant | attribute/attribute_test.go | AC-13: AttrTunnelEncap == 23 | |
| TestValidNextHopLensSRPolicy | attribute/mpnlri_test.go | SAFI 73 next-hop lengths registered | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI length bits (IPv4) | 96 | 96 | 0 (empty) | 97 (not byte-aligned) |
| NLRI length bits (IPv6) | 192 | 192 | 0 (empty) | 193 (not byte-aligned) |
| Distinguisher | 0-4294967295 | 4294967295 | N/A (unsigned) | N/A (4 bytes) |
| Color | 0-4294967295 | 4294967295 | N/A (unsigned) | N/A (4 bytes) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| decode-sr-policy | test/decode/ | ze bgp decode of SR-Policy UPDATE hex | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| sr-policy-announce | test/interop/ | ExaBGP (via bridge) | SR-Policy UPDATE round-trip through Ze | deferred (needs ExaBGP PR merged first) |

## Files to Modify
- `internal/core/family/family.go` - SAFISRPolicy constant (already done)
- `internal/core/bgp/attribute/attribute.go` - AttrTunnelEncap = 23
- `internal/core/bgp/attribute/mpnlri.go` - ValidNextHopLens for SAFI 73
- `internal/exabgp/bridge/bridge.go` - parseFamilyToAFISAFI sr-policy case
- `internal/exabgp/bridge/bridge_command.go` - sr-policy command translation
- `docs/architecture/wire/nlri.md` - add SR-Policy to family table

## Files to Create
- `internal/component/bgp/plugins/nlri/srpolicy/types.go` - SRPolicy NLRI type
- `internal/component/bgp/plugins/nlri/srpolicy/types_test.go` - SRPolicy tests
- `internal/component/bgp/plugins/nlri/srpolicy/split.go` - NLRI splitter
- `internal/component/bgp/plugins/nlri/srpolicy/split_test.go` - splitter tests
- `internal/component/bgp/plugins/nlri/srpolicy/register.go` - plugin registration
- `internal/component/bgp/plugins/nlri/srpolicy/json.go` - JSON serialization
- `internal/core/bgp/attribute/tunnel_encap.go` - Tunnel Encapsulation attribute (code 23)
- `internal/core/bgp/attribute/tunnel_encap_test.go` - Tunnel Encap tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|

### Implementation Phases

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations

## RFC Documentation

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim | Source evidence | Verified |
|---------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Architecture docs updated where needed
- [ ] Critical Review passes
- [ ] Risks & Assumptions resolved

### Quality Gates
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written and pass
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
