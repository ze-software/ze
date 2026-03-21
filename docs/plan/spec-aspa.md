# Spec: ASPA Path Verification

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-rpki-0-umbrella |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-rpki-0-umbrella.md` - RPKI umbrella spec (D5 defers ASPA here)
4. `internal/component/bgp/plugins/rpki/rpki.go` - existing RPKI plugin structure
5. `internal/component/bgp/plugins/rpki/validate.go` - existing ROA validation algorithm
6. `internal/component/bgp/plugins/rpki/rtr_pdu.go` - existing RTR PDU types

## Task

Add ASPA (Autonomous System Provider Authorization) path verification to Ze. ASPA allows an AS to declare its authorized upstream providers via RPKI. When a route is received, the AS_PATH is verified against ASPA records: each hop in the path is checked to confirm the customer-provider relationship is authorized. This is the "upstream verification" algorithm from draft-ietf-sidrops-aspa-verification-24 (Internet-Draft, no RFC number assigned as of March 2026).

ASPA complements ROA origin validation (RFC 6811) by verifying not just the origin but the entire AS_PATH. This spec extends the existing `bgp-rpki` plugin.

### Relationship to RPKI Umbrella

This spec fulfills the deferral "D5: ASPA deferred" from `spec-rpki-0-umbrella.md`, which states: "ASPA validation (draft-ietf-sidrops-aspa-verification) is a separate concern with its own RTR PDU type and validation algorithm."

### Scope

| In scope | Out of scope |
|----------|-------------|
| ASPA record storage (customer-AS to authorized-provider set) | ROA validation changes (already in rpki-0) |
| RTR PDU type for ASPA (receive from cache server) | RTR v2 protocol changes beyond ASPA PDU |
| Upstream path verification algorithm | Downstream path verification |
| ASPA validation states (Valid, Invalid, Unknown) | Policy actions based on ASPA state (separate concern) |
| Per-route ASPA validation integrated into RPKI plugin | ASPA-aware best-path selection in RIB |
| Config for enabling ASPA validation | ASPA record creation/signing (CA-side) |
| JSON event output with ASPA validation state | |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RPKI plugin integration pattern
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/wire/attributes.md` - AS_PATH attribute format (needed for verification)
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/api/architecture.md` - plugin event format
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6811.md` - ROA validation (existing, for pattern reference)
  -> Constraint:
- [ ] `rfc/short/rfc8210.md` - RTR v1 protocol (PDU types, session lifecycle)
  -> Constraint:
- [ ] RFC summary for ASPA verification (draft-ietf-sidrops-aspa-verification-24, no RFC number yet) - MUST CREATE
  -> Constraint:

**Key insights:** (summary of all checkpoint lines -- minimal context to resume after compaction)
- [to be filled during design phase]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - RPKI plugin entry point, ROA validation gate
- [ ] `internal/component/bgp/plugins/rpki/validate.go` - RFC 6811 origin validation: Validate(prefix, originAS) returns state
- [ ] `internal/component/bgp/plugins/rpki/roa_cache.go` - ROA cache VRP storage, covering-prefix lookup
- [ ] `internal/component/bgp/plugins/rpki/rtr_pdu.go` - RTR PDU types (v1), no ASPA PDU yet
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - RTR session lifecycle
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - RPKI config parsing
- [ ] `internal/component/bgp/plugins/rpki/emit.go` - RPKI event JSON building

**Behavior to preserve:** (unless user explicitly said to change)
- Existing ROA validation algorithm and states
- RTR session lifecycle and reconnection logic
- RPKI plugin event flow (hold route, validate, accept/reject)
- Existing PDU parsing for ROA prefixes
- Config structure for cache server

**Behavior to change:** (only if user explicitly requested)
- Add ASPA record cache alongside ROA cache
- Add ASPA RTR PDU parsing
- Add upstream path verification algorithm
- Add ASPA validation state to event output
- Extend config to enable/disable ASPA

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- RTR cache server sends ASPA PDUs (customer-AS, provider-AS set) during sync
- BGP UPDATE received with AS_PATH attribute triggers ASPA verification

### Transformation Path
1. RTR session receives ASPA PDU -> parse customer-AS and provider-AS-set
2. ASPA cache stores customer-AS -> set of authorized providers (with AFI)
3. Route received -> extract AS_PATH from UPDATE
4. Upstream verification: walk AS_PATH, check each hop's provider authorization
5. Result: Valid (all hops authorized), Invalid (unauthorized hop found), Unknown (no ASPA records)
6. Validation state included in event to RIB plugin

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RTR cache -> ASPA cache | ASPA PDU parsing -> cache storage | [ ] |
| Engine -> RPKI plugin | JSON event with AS_PATH and peer info | [ ] |
| RPKI plugin -> RIB plugin | accept/reject command with ASPA state | [ ] |

### Integration Points
- `rtr_pdu.go` - add ASPA PDU type constant and parser
- `rtr_session.go` - handle ASPA PDU in session receive loop
- `rpki.go` - call ASPA verification after ROA validation
- `validate.go` or new `aspa_verify.go` - upstream verification algorithm
- New `aspa_cache.go` - ASPA record storage

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RTR session with ASPA PDU | -> | ASPA cache populated | [to be determined] |
| UPDATE with AS_PATH + ASPA enabled | -> | upstream verification runs | [to be determined] |
| Config with ASPA enabled | -> | ASPA validation active in plugin | [to be determined] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RTR ASPA PDU received | ASPA record stored in cache (customer-AS -> provider set) |
| AC-2 | Route with AS_PATH where all hops have authorized providers | ASPA state = Valid |
| AC-3 | Route with AS_PATH containing unauthorized provider hop | ASPA state = Invalid |
| AC-4 | Route with AS_PATH where some hops have no ASPA records | ASPA state = Unknown |
| AC-5 | ASPA cache update (new/withdrawn records) | Affected routes re-validated |
| AC-6 | JSON event output includes ASPA validation state | `"aspa-state"` field in event JSON |
| AC-7 | ASPA disabled in config | No ASPA verification performed, ROA-only |
| AC-8 | Malformed ASPA PDU from RTR | Error logged, PDU skipped, session continues |
| AC-9 | AS_PATH with AS_SET segments | ASPA verification result = Unknown (cannot verify sets) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASPACacheAdd` | `rpki/aspa_cache_test.go` | Add ASPA record to cache | |
| `TestASPACacheRemove` | `rpki/aspa_cache_test.go` | Withdraw ASPA record | |
| `TestASPACacheLookup` | `rpki/aspa_cache_test.go` | Lookup providers for customer AS | |
| `TestASPAVerifyValid` | `rpki/aspa_verify_test.go` | All hops authorized -> Valid | |
| `TestASPAVerifyInvalid` | `rpki/aspa_verify_test.go` | Unauthorized hop -> Invalid | |
| `TestASPAVerifyUnknown` | `rpki/aspa_verify_test.go` | Missing ASPA records -> Unknown | |
| `TestASPAVerifyASSet` | `rpki/aspa_verify_test.go` | AS_SET in path -> Unknown | |
| `TestASPAVerifySingleHop` | `rpki/aspa_verify_test.go` | Single-hop AS_PATH edge case | |
| `TestASPAVerifyEmptyPath` | `rpki/aspa_verify_test.go` | Empty AS_PATH edge case | |
| `TestParseASPAPDU` | `rpki/rtr_pdu_test.go` | Parse ASPA PDU from wire bytes | |
| `TestParseASPAPDUMalformed` | `rpki/rtr_pdu_test.go` | Malformed ASPA PDU handling | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Customer AS | 1 - 2^32-1 | 0xFFFFFFFE | 0 (reserved) | N/A (uint32) |
| Provider AS | 1 - 2^32-1 | 0xFFFFFFFE | 0 (reserved) | N/A (uint32) |
| Provider count per ASPA | 1+ | 1 (minimum) | 0 (empty set) | implementation limit |
| AS_PATH length for verification | 0 - 1000 | 1000 (MaxASPathTotalLength) | N/A | 1001 (already rejected) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-aspa-valid` | `test/plugin/*.ci` | Route with fully authorized path -> accepted | |
| `rpki-aspa-invalid` | `test/plugin/*.ci` | Route with unauthorized hop -> rejected | |
| `rpki-aspa-unknown` | `test/plugin/*.ci` | Route with no ASPA coverage -> accepted (policy) | |
| `rpki-aspa-disabled` | `test/plugin/*.ci` | ASPA disabled, only ROA validation runs | |

### Future (if deferring any tests)
- Downstream path verification (separate algorithm, separate spec)
- ASPA-aware best-path selection (RIB plugin concern)
- Policy actions based on ASPA state (accept/reject/local-pref adjustment)

## Files to Modify
- `internal/component/bgp/plugins/rpki/rpki.go` - integrate ASPA verification into validation flow
- `internal/component/bgp/plugins/rpki/rtr_pdu.go` - add ASPA PDU type and parser
- `internal/component/bgp/plugins/rpki/rtr_session.go` - handle ASPA PDU in receive loop
- `internal/component/bgp/plugins/rpki/rpki_config.go` - ASPA enable/disable config
- `internal/component/bgp/plugins/rpki/emit.go` - ASPA state in event JSON
- `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` - ASPA config schema

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `rpki/schema/ze-rpki.yang` (ASPA config) |
| RPC count in architecture docs | [ ] | N/A (no new RPCs, extends existing) |
| CLI commands/flags | [ ] | possibly `rpki show aspa-cache` |
| CLI usage/help text | [ ] | if CLI command added |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/plugin/*.ci` |

## Files to Create
- `internal/component/bgp/plugins/rpki/aspa_cache.go` - ASPA record storage (customer-AS -> provider set)
- `internal/component/bgp/plugins/rpki/aspa_cache_test.go` - cache unit tests
- `internal/component/bgp/plugins/rpki/aspa_verify.go` - upstream path verification algorithm
- `internal/component/bgp/plugins/rpki/aspa_verify_test.go` - verification unit tests
- `test/plugin/rpki-aspa-*.ci` - functional tests

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: RFC summary** -- Create RFC summary for ASPA verification (confirm RFC number)
   - Verify: summary exists, covers verification algorithm, PDU format, validation states
2. **Phase: ASPA cache** -- Customer-AS to provider-set storage
   - Tests: `TestASPACacheAdd`, `TestASPACacheRemove`, `TestASPACacheLookup`
   - Files: `aspa_cache.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: RTR ASPA PDU** -- Parse ASPA PDU from RTR cache server
   - Tests: `TestParseASPAPDU`, `TestParseASPAPDUMalformed`
   - Files: `rtr_pdu.go`, `rtr_session.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Upstream verification** -- Walk AS_PATH, check provider authorization per hop
   - Tests: `TestASPAVerifyValid`, `TestASPAVerifyInvalid`, `TestASPAVerifyUnknown`, `TestASPAVerifyASSet`, `TestASPAVerifySingleHop`, `TestASPAVerifyEmptyPath`
   - Files: `aspa_verify.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Plugin integration** -- Wire ASPA verification into RPKI plugin event flow
   - Tests: integration-level tests
   - Files: `rpki.go`, `emit.go`, `rpki_config.go`
   - Verify: tests fail -> implement -> tests pass
6. **Phase: Config and YANG** -- ASPA enable/disable config
   - Tests: config parsing tests
   - Files: `rpki_config.go`, `schema/ze-rpki.yang`
   - Verify: tests fail -> implement -> tests pass
7. **Functional tests** -- Create .ci tests for ASPA validation scenarios
8. **RFC refs** -- Add `// RFC NNNN Section X.Y` comments
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Upstream verification algorithm matches RFC exactly (hop-by-hop check) |
| Naming | JSON key is `"aspa-state"` (kebab-case), cache types follow rpki naming |
| Data flow | ASPA verification runs after ROA validation, before accept/reject decision |
| Rule: no-layering | ASPA extends existing RPKI plugin, does not create parallel validation |
| Rule: single-responsibility | aspa_cache.go and aspa_verify.go are separate concerns from ROA |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `aspa_cache.go` exists | `ls internal/component/bgp/plugins/rpki/aspa_cache.go` |
| `aspa_verify.go` exists | `ls internal/component/bgp/plugins/rpki/aspa_verify.go` |
| ASPA PDU type in rtr_pdu.go | `grep -n ASPA internal/component/bgp/plugins/rpki/rtr_pdu.go` |
| ASPA verification called from rpki.go | `grep -n aspa internal/component/bgp/plugins/rpki/rpki.go` |
| Unit tests pass | `go test -race ./internal/component/bgp/plugins/rpki/... -run ASPA -v` |
| Functional tests exist | `ls test/plugin/rpki-aspa-*.ci` |
| RFC summary exists | `ls rfc/short/rfc*.md` for ASPA RFC |
| YANG schema updated | `grep -n aspa internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ASPA PDU length validation, provider-AS count bounds |
| Resource exhaustion | ASPA cache size limits (large number of ASPA records from cache server) |
| Path verification DoS | Verification time bounded by AS_PATH length (already capped at 1000) |
| Cache poisoning | RTR session auth ensures trusted cache server only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation states, upstream verification steps, ASPA PDU format, AS_SET handling.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
