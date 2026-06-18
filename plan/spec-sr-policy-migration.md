# Spec: sr-policy-migration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-06-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/config/bgp_routes.go` - route extraction dispatcher (switch on family)
4. `internal/component/bgp/config/bgp_routes_mup.go` - MUP route model (reference pattern)
5. `internal/exabgp/migration/migrate_routes.go` - flex SAFI migration

## Task

Add SR-Policy (SAFI 73) support to ze's ExaBGP config migration and config route builder so that `conf-sr-policy` exabgp-compat encoding tests pass.

Ze already has SR-Policy wire encoding/decoding (`internal/component/bgp/plugins/nlri/srpolicy/`),
CLI text parsing (`plugins/cmd/update/update_text_srpolicy.go`), and the ExaBGP YANG schema
now includes `sr-policy` as a flex container. The flex migration successfully parses the
ExaBGP config and splits tokens into attributes and NLRI parts. But two pieces are missing:

1. **Config route builder**: `extractRoutesFromUpdateBlock` in `bgp_routes.go` has no
   `case "ipv4/sr-policy", "ipv6/sr-policy":` branch. SR-Policy content falls through to
   the standard prefix parser, which calls `netip.ParsePrefix("distinguisher")` and fails.

2. **Tunnel Encapsulation attribute building**: SR-Policy routes carry preference,
   binding-SID, and segment-list data in the Tunnel Encapsulation attribute (type 23,
   RFC 9012/9830). The flex migration currently treats these as NLRI tokens (they are
   not in `flexAttrKeywords`). Either the migration must separate them into an attribute,
   or the config route builder must construct the TunnelEncap from the NLRI content string.

The parent spec `plan/spec-exabgp-compat-sync.md` (AC-5) created the test data files
(`test/exabgp-compat/encoding/conf-sr-policy.ci`, `test/exabgp-compat/etc/conf-sr-policy.conf`)
and added `sr-policy` to the YANG schema and `flexSafis` list. This spec completes the pipeline.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config tree structure and update block format
  -> Constraint: update blocks use `nlri { family content; }` structure
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  -> Constraint: Tunnel Encapsulation (type 23) uses TLV format

### RFC Summaries
- [ ] `rfc/short/rfc9830.md` - SR-Policy NLRI wire format (SAFI 73)
  -> Constraint: NLRI = distinguisher(4) + color(4) + endpoint(4 or 16)
- [ ] `rfc/short/rfc9012.md` - Tunnel Encapsulation attribute
  -> Constraint: TLV format: type(2) + length(2) + sub-TLVs

**Key insights:**
- Ze's `parseSRPolicySection` (update text command) already parses `distinguisher <d> color <c> endpoint <ep>` and builds the NLRI. The config route builder should reuse this logic.
- SR-Policy tunnel encap sub-TLVs: Preference (12), BindingSID (13), SegmentList (128), Priority (15). Constants are in `attribute/tunnel_encap.go`.
- MUP route building (`bgp_routes_mup.go`) is the closest pattern: config route type + parse function + convert function.
- The flex migration already sends `preference`, `binding-sid`, `segment-list` etc. as NLRI tokens. The route builder must extract them from the NLRI content line and build the TunnelEncap attribute.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/exabgp/migration/migrate_routes.go` - flex migration splits SR-Policy tokens; `next-hop` goes to attrs, everything else to NLRI parts. Produces content like `"add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001"`
- [ ] `internal/component/bgp/config/bgp_routes.go:128-191` - `extractRoutesFromUpdateBlock` dispatches on family name. Has cases for flow, vpls, mvpn, mup. Falls through to standard prefix parser for unknown families.
- [ ] `internal/component/bgp/config/bgp_routes_mup.go` - MUP reference: `parseMUPNLRILine` parses family, op, route-type, key-value pairs. `MUPRouteConfig` stores parsed fields. `convertMUPRoute` in `peers.go` builds wire objects.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text_srpolicy.go` - CLI parser for SR-Policy NLRI. Parses `distinguisher`, `color`, `endpoint` keywords. Builds `srpolicy.SRPolicy` NLRI. Does NOT handle tunnel encap attributes.
- [ ] `internal/component/bgp/attribute/tunnel_encap.go` - TunnelEncap type with TLV + SubTLV parsing. Sub-TLV constants: Preference (12), BindingSID (13), SegmentList (128), Priority (15).
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/types.go` - `SRPolicy` struct: AFI + distinguisher(u32) + color(u32) + endpoint(Addr). `New()` constructor, `Bytes()` for wire format.

**Behavior to preserve:**
- All existing exabgp-compat tests (41/42 currently pass)
- Existing SR-Policy decode path and CLI text command
- Existing flex SAFI migration for mup, mvpn, vpls

**Behavior to change:**
- `extractRoutesFromUpdateBlock` must dispatch `ipv4/sr-policy` and `ipv6/sr-policy` to a new parser
- New `SRPolicyRouteConfig` type to carry parsed fields
- New `parseSRPolicyNLRILine` to parse the flex-migrated content
- New `convertSRPolicyRoute` to build wire NLRI + TunnelEncap attribute
- `UpdateBlockRoutes` gains `SRPolicyRoutes` field
- `peers.go` dispatches SR-Policy routes to the reactor

## Data Flow (MANDATORY)

### Entry Point
- ExaBGP config file `conf-sr-policy.conf` with SR-Policy routes
- Enters via `ze exabgp migrate` which uses the ExaBGP YANG schema

### Transformation Path
1. **ExaBGP YANG parse**: `ParseExaBGPConfig` parses `announce { ipv4 { sr-policy ...; } }`
2. **Flex migration**: `convertFlexToUpdate` splits tokens. `next-hop` -> attribute block. `distinguisher color endpoint preference binding-sid segment-list` -> NLRI content string.
3. **Config tree**: Migrated tree has `update { attribute { origin igp; next-hop 192.0.2.1; } nlri { ipv4/sr-policy "add distinguisher 0 color 100 endpoint 10.0.0.1 preference 100 ..."; } }`
4. **Route builder**: `extractRoutesFromUpdateBlock` dispatches on `"ipv4/sr-policy"` -> `parseSRPolicyNLRILine`
5. **SR-Policy parse**: Extracts distinguisher, color, endpoint (NLRI fields) + preference, binding-sid, segment-list (TunnelEncap sub-TLV fields)
6. **Convert**: `convertSRPolicyRoute` builds `srpolicy.SRPolicy` NLRI + `attribute.TunnelEncap` with sub-TLVs
7. **Reactor**: Peer settings stores SR-Policy routes; announces on session establishment

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ExaBGP config -> Ze config tree | flex migration (already works) | [ ] |
| Config tree -> Route config | `parseSRPolicyNLRILine` (new) | [ ] |
| Route config -> Wire objects | `convertSRPolicyRoute` (new) | [ ] |
| Wire objects -> TCP | reactor static route announce | [ ] |

### Integration Points
- `internal/component/bgp/config/bgp_routes.go:159` - new `case "ipv4/sr-policy", "ipv6/sr-policy":` in family switch
- `internal/component/bgp/config/peers.go:458` - new SR-Policy route dispatch (after MUP)
- `internal/component/bgp/plugins/nlri/srpolicy/types.go` - reuse `srpolicy.New()` to build NLRI
- `internal/component/bgp/attribute/tunnel_encap.go` - construct `TunnelEncap` with sub-TLVs

### Architectural Verification
- [ ] No bypassed layers (uses existing config tree + route builder pipeline)
- [ ] No unintended coupling (new file `bgp_routes_srpolicy.go` follows MUP pattern)
- [ ] No duplicated functionality (reuses `srpolicy.New()` from the NLRI plugin)
- [ ] Zero-copy preserved where applicable (TunnelEncap sub-TLV bytes are built once)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Flex migration puts all SR-Policy keywords in NLRI parts (not attrs) | `flexAttrKeywords` has no SR-Policy-specific entries | Keywords split wrong, route builder gets wrong input | Run `ze exabgp migrate conf-sr-policy.conf` and inspect output | unvalidated |
| A-2 | Ze's SR-Policy wire encoding matches ExaBGP's for same input | SR-Policy NLRI is simple (RFC 9830 Section 2.1) | Wire bytes differ, test fails | Compare ze encode output vs `.ci` expected raw | unvalidated |
| A-3 | Tunnel type 15 is the correct SR Policy type for tunnel encap | ExaBGP wire data starts tunnel TLV with 000F | Wrong tunnel type -> interop failure | Check RFC 9012 IANA registry | unvalidated |
| A-4 | Reactor can announce SR-Policy routes via static route path | MUP/MVPN use this pattern | Need separate announce path | Trace MUP route flow in reactor | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | TunnelEncap sub-TLV byte order differs from ExaBGP | Wire test fails on tunnel encap bytes | Compare byte-by-byte against RFC 9012/9830 wire format definitions |
| R-2 | Segment-list sub-TLV has nested segment sub-sub-TLVs | Wire bytes more complex than expected | Parse ExaBGP `.ci` raw bytes to reverse-engineer the exact format |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ExaBGP config with SR-Policy routes | -> | `parseSRPolicyNLRILine` + `convertSRPolicyRoute` | `test/exabgp-compat/encoding/conf-sr-policy.ci` |
| Config tree with `ipv4/sr-policy` NLRI | -> | `extractRoutesFromUpdateBlock` dispatch | `TestExtractSRPolicyRoutes` in `bgp_routes_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze exabgp migrate conf-sr-policy.conf` | Produces valid ze config with SR-Policy update blocks |
| AC-2 | Migrated config loaded by ze | No error; SR-Policy routes parsed into `SRPolicyRouteConfig` |
| AC-3 | SR-Policy route with MPLS binding-sid + segment type-A | TunnelEncap attribute built with correct sub-TLVs; wire bytes match `.ci` raw |
| AC-4 | SR-Policy route with SRv6 binding-sid + segment type-B + endpoint-behavior + names | TunnelEncap built with all sub-TLVs including policy-name and candidate-path-name |
| AC-5 | SR-Policy route with multiple segment lists | TunnelEncap has multiple SegmentList sub-TLVs; wire bytes match `.ci` raw |
| AC-6 | IPv6 SR-Policy route | Correct AFI=2 NLRI with IPv6 endpoint; wire bytes match `.ci` raw |
| AC-7 | `make ze-exabgp-test` | `conf-sr-policy` test passes (all 42/42 green) |
| AC-8 | All existing exabgp-compat tests | No regressions (41 that currently pass still pass) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Migrates ExaBGP SR-Policy config | `ze exabgp migrate` -> flex parse -> config tree with sr-policy NLRI + tunnel-encap attrs | `conf-sr-policy.ci` encoding test |
| 2 | Ze announces SR-Policy to mock peer | config load -> `extractRoutesFromUpdateBlock` -> `parseSRPolicyNLRILine` -> `convertSRPolicyRoute` -> reactor announce -> wire bytes match expected | `conf-sr-policy.ci` raw comparison |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseSRPolicyNLRILine_MPLS` | `bgp_routes_test.go` | MPLS preference + binding-sid + segment type-A parsing | |
| `TestParseSRPolicyNLRILine_SRv6` | `bgp_routes_test.go` | SRv6 binding-sid + type-B + endpoint-behavior + names | |
| `TestParseSRPolicyNLRILine_MultiSegList` | `bgp_routes_test.go` | Multiple segment lists | |
| `TestConvertSRPolicyRoute_WireBytes` | `peers_test.go` | Converted route produces expected NLRI + TunnelEncap wire bytes | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| distinguisher | 0-4294967295 | 4294967295 | N/A (uint32) | N/A (uint32) |
| color | 0-4294967295 | 4294967295 | N/A | N/A |
| MPLS label | 0-1048575 | 1048575 | N/A | 1048576 |
| segment-list weight | 0-4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conf-sr-policy` | `test/exabgp-compat/encoding/conf-sr-policy.ci` | ExaBGP SR-Policy config round-trip: migrate + encode + wire compare | existing, currently failing |

### Interop Tests
Not applicable: this spec adds config migration, not new wire behavior. SR-Policy encoding already exists.

## Files to Modify
- `internal/component/bgp/config/bgp_routes.go` - add `case "ipv4/sr-policy", "ipv6/sr-policy":` dispatch and `SRPolicyRoutes` field to `UpdateBlockRoutes`
- `internal/component/bgp/config/peers.go` - add SR-Policy route dispatch in `patchRoutes`
- `internal/component/bgp/reactor/peersettings.go` - add `SRPolicyRoutes` field (if not already present)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | SR-Policy YANG already exists |
| CLI commands/flags | No | CLI text command already works |
| Functional test | Yes | `test/exabgp-compat/encoding/conf-sr-policy.ci` (already exists) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | ExaBGP migration is internal |
| 2 | Config syntax changed? | No | Ze native SR-Policy config already documented |
| 3-17 | Other categories | No | Migration-only change |

## Files to Create
- `internal/component/bgp/config/bgp_routes_srpolicy.go` - `SRPolicyRouteConfig` type + `parseSRPolicyNLRILine` + `convertSRPolicyRoute`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-exabgp-test` |

### Implementation Phases

1. **Phase: Wiring** - Add `case "ipv4/sr-policy", "ipv6/sr-policy":` in `extractRoutesFromUpdateBlock` that calls a stub `parseSRPolicyNLRILine` returning an error. Write wiring test that confirms dispatch.

2. **Phase: NLRI parsing** - Implement `parseSRPolicyNLRILine` to extract distinguisher, color, endpoint from content line. Store in `SRPolicyRouteConfig`. Unit test with MPLS and SRv6 cases.

3. **Phase: TunnelEncap building** - Parse preference, binding-sid, srv6-binding-sid, segment-list, policy-name, candidate-path-name from content line. Build `attribute.TunnelEncap` with sub-TLVs matching RFC 9012/9830 wire format. Verify wire bytes against `.ci` expected raw.

4. **Phase: Route conversion + reactor wiring** - Implement `convertSRPolicyRoute` to produce wire NLRI + TunnelEncap. Wire into `peers.go` dispatch and reactor `PeerSettings`. Run `make ze-exabgp-test`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Wire bytes match ExaBGP `.ci` expected raw for all 4 SR-Policy test cases |
| Wire compat | TunnelEncap sub-TLV format matches RFC 9012 (1-byte vs 2-byte length) |
| No regression | All 41 existing exabgp-compat tests still pass |
| Pattern consistency | `bgp_routes_srpolicy.go` follows `bgp_routes_mup.go` structure |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SR-Policy route parsing | `go test ./internal/component/bgp/config/ -run SRPolicy` |
| TunnelEncap building | Unit test comparing wire output against `.ci` raw hex |
| Encoding test passes | `make ze-exabgp-test` shows 42/42 pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | TunnelEncap sub-TLV lengths validated before allocation |
| Resource exhaustion | Segment-list count bounded (ExaBGP configs are trusted input) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Wire bytes don't match `.ci` raw | Decode ExaBGP raw hex byte-by-byte against RFC; fix encoding |
| Flex migration splits tokens wrong | Adjust `flexAttrKeywords` or add SR-Policy-specific splitter |
| Reactor can't announce SR-Policy | Trace MUP announce path; mirror for SR-Policy |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build TunnelEncap in config route builder, not in migration | Could build TunnelEncap bytes in migration `convertFlexToUpdate` | Config route builder already has the pattern (MUP builds prefix-sid-srv6 from attr tree). Migration should stay format-agnostic. |
| New `bgp_routes_srpolicy.go` file | Could inline in `bgp_routes.go` | Follows existing pattern: one file per complex SAFI (flowspec, vpls, mvpn, mup). |

## Known Limitations
- SR-Policy `priority` sub-TLV (type 15) is in the ExaBGP config but not all test cases exercise it
- No SR-Policy decode test (separate from encoding test): can follow in a decode spec

## RFC Documentation

Add `// RFC 9830 Section 2.1: "<quoted requirement>"` above NLRI parsing.
Add `// RFC 9012 Section 3: "<quoted requirement>"` above TunnelEncap sub-TLV construction.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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

| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| SR-Policy migration works | functional test | `conf-sr-policy.ci` passes |
| No regressions | full suite | `make ze-exabgp-test` 42/42 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during review)

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-exabgp-test` passes (42/42)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
