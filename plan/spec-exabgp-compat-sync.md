# Spec: exabgp-compat-sync

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `test/exabgp-compat/encoding/` - ze encoding test data
4. `test/exabgp-compat/etc/` - ze config test data
5. `test/decode/` - ze decode test data
6. `~/Code/github.com/Exa-Networks/exabgp/main/qa/` - ExaBGP test data (external)

## Task

Synchronize ExaBGP compatibility test data between ze and ExaBGP. A side-by-side comparison (2026-06-17) found gaps in both directions: missing test cases, divergent expected output, and one real decoding gap in ze's JSON rendering of IPv6 extended communities.

This spec covers three categories:
1. **Ze-side fixes** - code and test data changes in this repo
2. **ExaBGP-side fixes** - test data to port to the ExaBGP repo (out of scope for implementation here, tracked for the user)
3. **Decisions needed** - divergences that require a design choice

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - wire abstractions, JSON output format
  -> Constraint: ze uses its own JSON schema for BGP messages, not ExaBGP's format
- [ ] `ai/rules/testing.md` - test data modification rules
  -> Constraint: never modify test data to make tests pass without user authorization

### Source Files
- [ ] `internal/component/bgp/attribute/community.go` - extended community handling, `IPv6ExtendedCommunity` at line 469
- [ ] `internal/component/bgp/config/loader_routes.go` - IPv6 redirect-to-nexthop building, `buildIPv6ExtCommunityFromString` at line 353
- [ ] `internal/component/bgp/config/routeattr_community.go` - community parsing
- [ ] `test/exabgp-compat/bin/functional` - exabgp-compat test runner
- [ ] `test/exabgp-compat/bin/exabgp` - ExaBGP compatibility wrapper

**Key insights:**
- Ze has SR-Policy support (SAFI 73) already implemented
- Ze has extended-nexthop capability support already implemented
- Ze renders IPv6 extended communities as opaque `attribute-0x19-0xC0` in exabgp-compat JSON
- ExaBGP recently (2025-2026) added `router-id` to all JSON output, SR-Policy tests, and fixed IPv6 ext community rendering
- Ze's exabgp-compat tests use the ExaBGP `.ci` format with `:json:`, `:raw:`, `:cmd:` lines
- Ze's decode tests (`test/decode/`) use a different `.ci` format with `stdin=`, `cmd=`, `expect=json:`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/exabgp-compat/encoding/conf-flow-redirect.ci` - renders IPv6 ext community as `attribute-0x19-0xC0`
- [ ] `test/exabgp-compat/encoding/conf-srv6-mup.ci` - has 4 `:json:` lines; ExaBGP has 10
- [ ] `test/exabgp-compat/encoding/conf-srv6-mup-v3.ci` - has 0 `:json:` lines; ExaBGP has 2
- [ ] `test/decode/bgp-evpn-1.ci` - EVPN label bytes end `01` vs ExaBGP's `00`

**Behavior to preserve:**
- Ze's own JSON output format in `test/decode/` (different schema than ExaBGP JSON)
- Ze's OPEN capability encoding (expected to differ from ExaBGP)
- All existing passing tests

**Behavior to change:**
- IPv6 extended community JSON rendering should use `extended-community-ipv6` key (not opaque fallback)
- Missing `:json:` expectations in SRv6 MUP tests should be added
- Missing encoding test cases should be ported from ExaBGP

## Findings Summary

### Category 1: Ze-side gaps (actionable in this repo)

| # | Gap | Type | Severity |
|---|-----|------|----------|
| Z-1 | IPv6 extended community rendered as opaque `attribute-0x19-0xC0` instead of `extended-community-ipv6` | Code bug | High |
| Z-2 | `conf-srv6-mup.ci` missing 6 `:json:` expectations (InterworkSegment, DirectSegment, Type1Session) | Test data | Medium |
| Z-3 | `conf-srv6-mup-v3.ci` missing 2 `:json:` expectations (v3 Type1Session ipv4/ipv6) | Test data | Medium |
| Z-4 | `conf-flow.ci` missing `rate-limit:1000:packets` test case | Test data | Medium |
| Z-5 | No `conf-sr-policy` encoding test (ExaBGP added Jun 2025) | Missing test | Medium |
| Z-6 | No `extended-nexthop` encoding test | Missing test | Medium |
| Z-7 | `conf-watchdog.ci` has stale `option:timeout:10.0` that ExaBGP removed | Test data | Low |
| Z-8 | EVPN wire data: label bytes end `01` in ze vs `00` in ExaBGP | Investigation | Medium |
| Z-9 | No `vpnv4-extended-nexthop-1` decoding test | Missing test | Low |

### Category 2: ExaBGP-side gaps (for user to port)

| # | Gap | Description |
|---|-----|-------------|
| E-1 | `conf-flow-bytes` / `conf-flow-packets` encoding tests | Ze split flowspec rate-limit into separate byte/packet tests |
| E-2 | `conf-paths-limit` encoding test | Paths-limit capability test |
| E-3 | 18 extra decode tests | Ze covers keepalive, labeled, BGP-LS-11, MUP, MVPN, notifications, GR, route-refresh, paths-limit, RTC, VPLS, VPN, LLGR, flowspec-plugin, nlri-plugin-infra |

### Category 3: Decisions needed

| # | Decision | Options |
|---|----------|---------|
| D-1 | `router-id` in ExaBGP JSON output | **Decided:** match ExaBGP format -- flat `"router-id":"X.X.X.X"` key under `neighbor`, between `asn` and `direction` |
| D-2 | EVPN label byte difference (`01` vs `00`) | **Decided:** Ze is correct. RFC 3032 + RFC 7432: MPLS label S-bit (bottom-of-stack) must be 1 for single label stack. Ze encodes `000001` (S=1), ExaBGP encodes `000000` (S=0). ExaBGP has the bug. |

### Not in scope

- ExaBGP `option=` vs `option:` syntax delimiter change (ze's compat runner handles both)
- OPEN wire byte differences (different implementations, expected)
- ExaBGP API test differences (different architecture, not directly comparable)
- `unknown-message` encoding test (unknown message type 255 - niche)

## Data Flow (MANDATORY)

### Entry Point (Z-1: IPv6 extended community)
- Config `redirect-to-nexthop IP` with IPv6 address enters via `routeattr_community.go`
- Built as IPv6 Extended Community (attribute type 25, 0x19) in `loader_routes.go`

### Transformation Path
1. Config parsing: `routeattr_community.go` parses `redirect-to-nexthop 2a02:...`
2. Route building: `loader_routes.go:281` builds IPv6 Extended Community (attr 25)
3. Wire encoding: attribute bytes on wire
4. ExaBGP bridge JSON: attribute rendered back to JSON for exabgp-compat output
   -> **Gap is here**: JSON falls back to opaque `attribute-0x19-0xC0` instead of proper key

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Wire | Community parsing + attribute building | [ ] |
| Wire -> JSON | Attribute JSON rendering | [ ] - this is the gap |

### Integration Points
- `internal/component/bgp/attribute/community.go:469` - `IPv6ExtendedCommunity` type and JSON rendering
- ExaBGP bridge JSON formatter - converts wire attributes to ExaBGP-compatible JSON

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Ze already supports SR-Policy SAFI 73 encoding | grep found `srpolicy` in family.go, nlri plugins, cmd | Need to implement SR-Policy first | `grep -r srpolicy internal/` | confirmed |
| A-2 | Ze already supports extended-nexthop capability | grep found it in capa/ plugin | Need to implement capability first | `grep -r extended.nexthop internal/component/bgp/plugins/capa/` | confirmed |
| A-3 | ExaBGP's `router-id` addition is intentional and permanent | Recent commit `df96e05ae` | Could be reverted | Check ExaBGP git log | confirmed |
| A-4 | Ze's exabgp-compat runner validates `:json:` lines against actual output | Reading `test/exabgp-compat/bin/functional` | Missing `:json:` lines may be silently skipped | Read runner code | broken |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | IPv6 ext community JSON fix may change output for other attribute types | Existing exabgp-compat tests fail | Run full exabgp-compat suite before and after |
| R-2 | EVPN label byte difference may indicate a real wire encoding bug in ze | Interop failure with EVPN peers | Investigate against RFC 7432 before changing either side |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ExaBGP config with IPv6 redirect-to-nexthop | -> | JSON attribute rendering | `test/exabgp-compat/encoding/conf-flow-redirect.ci` |
| ExaBGP config with SR-Policy routes | -> | SR-Policy encoding + JSON | `test/exabgp-compat/encoding/conf-sr-policy.ci` (to create) |
| ExaBGP config with extended-nexthop | -> | Extended NH encoding + JSON | `test/exabgp-compat/encoding/extended-nexthop.ci` (to create) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `conf-flow-redirect.ci` with IPv6 redirect-to-nexthop | JSON output uses `extended-community-ipv6` key, not `attribute-0x19-0xC0` |
| AC-2 | `conf-srv6-mup.ci` encoding test | All 10 `:json:` expectations present and passing (6 new) |
| AC-3 | `conf-srv6-mup-v3.ci` encoding test | All 2 `:json:` expectations present and passing (2 new) |
| AC-4 | `conf-flow.ci` encoding test | Includes `rate-limit:1000:packets` test case |
| AC-5 | SR-Policy encoding test exists | `conf-sr-policy.ci` ported from ExaBGP, passes |
| AC-6 | Extended-nexthop encoding test exists | `extended-nexthop.ci` ported from ExaBGP, passes |
| AC-7 | `conf-watchdog.ci` | `option:timeout:10.0` removed |
| AC-8 | EVPN label byte investigated | Difference documented with RFC reference; correct side identified |
| AC-9 | `router-id` added to exabgp-compat JSON | Flat `"router-id":"X.X.X.X"` key under `neighbor` in all exabgp-compat JSON output, matching ExaBGP format. All `:json:` expectations updated. |
| AC-10 | All existing exabgp-compat tests still pass | `make ze-exabgp-test` green |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Announces flowspec route with IPv6 redirect-to-nexthop via ExaBGP bridge | config -> community parse -> attr 25 build -> wire -> JSON with `extended-community-ipv6` | `conf-flow-redirect.ci` |
| 2 | Runs ExaBGP compat test for SR-Policy | ExaBGP config -> ze migrate -> encode -> compare wire + JSON | `conf-sr-policy.ci` |
| 3 | Runs ExaBGP compat test for extended-nexthop | ExaBGP config -> ze migrate -> encode -> compare wire + JSON | `extended-nexthop.ci` |

## TDD Test Plan

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `conf-flow-redirect` | `test/exabgp-compat/encoding/conf-flow-redirect.ci` | IPv6 ext community renders properly in JSON | existing, update expected |
| `conf-srv6-mup` | `test/exabgp-compat/encoding/conf-srv6-mup.ci` | All MUP route types have JSON expectations | existing, add lines |
| `conf-srv6-mup-v3` | `test/exabgp-compat/encoding/conf-srv6-mup-v3.ci` | v3 MUP types have JSON expectations | existing, add lines |
| `conf-flow` | `test/exabgp-compat/encoding/conf-flow.ci` | FlowSpec packets rate-limit case present | existing, add case |
| `conf-sr-policy` | `test/exabgp-compat/encoding/conf-sr-policy.ci` | SR-Policy encoding round-trip | new |
| `extended-nexthop` | `test/exabgp-compat/encoding/extended-nexthop.ci` | Extended NH encoding round-trip | new |
| `conf-watchdog` | `test/exabgp-compat/encoding/conf-watchdog.ci` | No stale timeout option | existing, remove line |

### Future (if deferring any tests)
- `vpnv4-extended-nexthop-1` decode test - lower priority, can follow in a separate change

## Files to Modify
- `internal/component/bgp/attribute/community.go` or JSON renderer - fix IPv6 ext community JSON key (AC-1)
- `test/exabgp-compat/encoding/conf-flow-redirect.ci` - update expected JSON (AC-1)
- `test/exabgp-compat/encoding/conf-srv6-mup.ci` - add 6 `:json:` lines (AC-2)
- `test/exabgp-compat/encoding/conf-srv6-mup-v3.ci` - add 2 `:json:` lines (AC-3)
- `test/exabgp-compat/encoding/conf-flow.ci` + `test/exabgp-compat/etc/conf-flow.conf` - add packets case (AC-4)
- `test/exabgp-compat/encoding/conf-watchdog.ci` - remove timeout line (AC-7)

### Files to Create
- `test/exabgp-compat/encoding/conf-sr-policy.ci` - ported from ExaBGP (AC-5)
- `test/exabgp-compat/etc/conf-sr-policy.conf` - ported from ExaBGP (AC-5)
- `test/exabgp-compat/encoding/extended-nexthop.ci` - ported from ExaBGP (AC-6)
- `test/exabgp-compat/etc/extended-nexthop.conf` - ported from ExaBGP (AC-6)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-exabgp-test` + `make ze-verify-changed` |
| 7-13. Review cycle | Standard |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: Investigate EVPN label bytes (AC-8)** - compare ze and ExaBGP wire data against RFC 7432, determine which is correct, document finding
   - Files: `test/decode/bgp-evpn-1.ci`, RFC 7432 label field definition
   - Verify: decision documented in this spec

2. **Phase: Fix IPv6 extended community JSON (AC-1)** - find where attribute 25 (0x19) is rendered to JSON, add proper `extended-community-ipv6` key
   - Tests: run `conf-flow-redirect` encoding test
   - Files: `internal/component/bgp/attribute/community.go` or relevant JSON renderer
   - Verify: `conf-flow-redirect.ci` passes with updated `:json:` expectation

3. **Phase: Add missing SRv6 MUP JSON expectations (AC-2, AC-3)** - port `:json:` lines from ExaBGP for MUP route types
   - Tests: run `conf-srv6-mup` and `conf-srv6-mup-v3` encoding tests
   - Files: `test/exabgp-compat/encoding/conf-srv6-mup.ci`, `conf-srv6-mup-v3.ci`
   - Verify: encoding tests pass with all JSON expectations

4. **Phase: Sync flow test data (AC-4, AC-7)** - add packets rate-limit case to `conf-flow`, remove stale watchdog timeout
   - Tests: run `conf-flow` and `conf-watchdog` encoding tests
   - Files: `conf-flow.ci`, `conf-flow.conf`, `conf-watchdog.ci`
   - Verify: encoding tests pass

5. **Phase: Port new encoding tests (AC-5, AC-6)** - create SR-Policy and extended-nexthop tests from ExaBGP data
   - Tests: run new encoding tests
   - Files: new `.ci` and `.conf` files
   - Verify: encoding tests pass

6. **Phase: Add router-id to exabgp-compat JSON (AC-9)** - add `router-id` field to neighbor JSON output in the exabgp bridge, update all 28+ `:json:` expectations across encoding tests
   - Tests: run full exabgp-compat encoding suite
   - Files: exabgp bridge JSON formatter + all `test/exabgp-compat/encoding/*.ci` files with `:json:` lines
   - Verify: all encoding tests pass with `router-id` in JSON

7. **Full verification (AC-10)** - `make ze-exabgp-test`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has passing test evidence |
| Correctness | IPv6 ext community JSON matches ExaBGP format exactly |
| Wire compat | No existing exabgp-compat tests broken |
| Data integrity | New test data matches ExaBGP source exactly (diff verified) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| IPv6 ext community JSON fix | `grep extended-community-ipv6 test/exabgp-compat/encoding/conf-flow-redirect.ci` |
| SR-Policy encoding test | `ls test/exabgp-compat/encoding/conf-sr-policy.ci` |
| Extended-nexthop encoding test | `ls test/exabgp-compat/encoding/extended-nexthop.ci` |
| All exabgp tests pass | `make ze-exabgp-test` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Test data only, no new input paths |
| Wire safety | No changes to wire parsing, only JSON output rendering |

### Failure Routing
| Failure | Route To |
|---------|----------|
| IPv6 ext community fix breaks other JSON output | Narrow the fix to only attribute type 25 |
| SR-Policy encoding test fails | Check if ze's SR-Policy wire encoding matches ExaBGP expectations |
| Extended-nexthop test fails | Check capability negotiation differences |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## ExaBGP-Side Recommendations (for user)

These items should be ported to the ExaBGP repo:

| Item | ExaBGP location | Ze source |
|------|----------------|-----------|
| `conf-flow-bytes.ci` + `.conf` | `qa/encoding/` | `test/exabgp-compat/encoding/conf-flow-bytes.ci` |
| `conf-flow-packets.ci` + `.conf` | `qa/encoding/` | `test/exabgp-compat/encoding/conf-flow-packets.ci` |
| `conf-paths-limit.ci` + `.conf` | `qa/encoding/` | `test/exabgp-compat/encoding/conf-paths-limit.ci` |
| 18 decode test binaries | `qa/decoding/` | `test/decode/*.ci` (need hex extraction) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-4: `:json:` lines validated by test runner | Mock bgp server skips `:json:` lines (line 1019-1020 of `bin/bgp`). They are documentation only. | Reading `test/exabgp-compat/bin/bgp` group_messages() | Low: `:json:` lines are reference data, not test assertions. Updates are still valuable for documentation accuracy. |

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
- ExaBGP API test differences not addressed (different architecture)
- `unknown-message` encoding test not ported (niche, low value)

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
| IPv6 ext community JSON fixed | functional test | `conf-flow-redirect.ci` passes |
| Missing JSON expectations added | functional test | `conf-srv6-mup.ci`, `conf-srv6-mup-v3.ci` pass |
| New encoding tests ported | functional test | `conf-sr-policy.ci`, `extended-nexthop.ci` pass |
| No regressions | full suite | `make ze-exabgp-test` green |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-exabgp-test` passes
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
