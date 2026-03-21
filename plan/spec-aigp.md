# Spec: AIGP (Accumulated IGP Metric)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/wire/attributes.md` - attribute wire format
4. `internal/component/bgp/attribute/attribute.go` - attribute constants and names
5. `internal/component/bgp/attribute/wire.go` - attribute parsing dispatch

## Task

Add full AIGP (RFC 7311) support to Ze: wire parsing, encoding, JSON formatting, API command syntax, capability negotiation, and ExaBGP migration. AIGP carries the accumulated IGP metric across AS boundaries so BGP best-path selection can consider end-to-end IGP cost.

### Scope

| In scope | Out of scope |
|----------|-------------|
| Wire parsing of AIGP TLV (type 1 = metric) | AIGP-aware best-path selection in RIB plugin |
| Wire encoding (WriteTo buffer-first) | IGP metric injection from IGP protocols |
| JSON decode/encode (kebab-case) | Route redistribution policies using AIGP |
| API command syntax (`aigp set <value>`) | Per-peer AIGP policy configuration |
| Pool dedup for AIGP attribute | |
| AIGP capability negotiation (if defined) | |
| ExaBGP config migration for AIGP | |
| Unknown TLV types preserved as opaque | |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` - attribute wire format, flags, parsing dispatch
  → Decision:
  → Constraint:
- [ ] `docs/architecture/pool-architecture.md` - per-attribute-type pool dedup
  → Decision:
  → Constraint:
- [ ] `docs/architecture/api/commands.md` - API command syntax for attributes
  → Decision:
  → Constraint:
- [ ] `docs/architecture/config/syntax.md` - config syntax for attributes
  → Decision:
  → Constraint:

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7311.md` - AIGP attribute definition, TLV format, when to attach/propagate
  → Constraint:

**Key insights:** (summary of all checkpoint lines -- minimal context to resume after compaction)
- [to be filled during design phase]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/attribute/attribute.go` - AttrAIGP = 26, name "AIGP" registered
- [ ] `internal/component/bgp/attribute/wire.go` - AIGP listed as "known code without parser", treated as opaque
- [ ] `internal/component/bgp/message/` - UPDATE handling, attribute iteration
- [ ] `docs/architecture/config/syntax.md` - mentions `aigp <value>` in API command syntax
- [ ] `docs/architecture/api/commands.md` - mentions `aigp <value>` command

**Behavior to preserve:** (unless user explicitly said to change)
- Attribute code 26 constant and name mapping
- Opaque passthrough of AIGP bytes when parser is not active
- Existing attribute parsing dispatch pattern in wire.go

**Behavior to change:** (only if user explicitly requested)
- Add real parser for AIGP TLV structure (currently opaque)
- Add encoder using buffer-first pattern
- Add JSON formatting
- Add API command support

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes in BGP UPDATE path attributes (type code 26, optional transitive)
- API command: `aigp set <value>`

### Transformation Path
1. Wire parsing: attribute bytes -> AIGP TLV parsing (type 1 = 64-bit metric, others = opaque)
2. Pool dedup: parsed AIGP -> pool storage with refcounted handle
3. JSON encode: AIGP struct -> kebab-case JSON (`"aigp"` key)
4. Wire encode: AIGP struct -> WriteTo(buf, off) int
5. API parse: command string -> AIGP struct -> wire bytes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Storage | attribute iterator -> pool dedup | [ ] |
| Engine -> Plugin | JSON event with AIGP field | [ ] |
| API -> Wire | command parse -> attribute build | [ ] |

### Integration Points
- Attribute dispatch table in `wire.go` (register parser for code 26)
- JSON marshaling in attribute JSON output
- API command parser for `aigp` keyword
- ExaBGP migration for AIGP config syntax

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Wire bytes with AIGP attribute | -> | AIGP parser in wire.go | [to be determined] |
| API command `aigp set 100` | -> | AIGP encoder | [to be determined] |
| Config with AIGP route | -> | end-to-end parse + JSON | [to be determined] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE with AIGP attribute (type 1 TLV, metric=100) | Parsed correctly, metric value extracted |
| AC-2 | AIGP attribute with unknown TLV types | Unknown TLVs preserved as opaque bytes |
| AC-3 | JSON output of UPDATE with AIGP | `"aigp"` key with metric value in kebab-case JSON |
| AC-4 | API command `aigp set <value>` | AIGP attribute encoded in UPDATE |
| AC-5 | AIGP wire encoding | Uses buffer-first WriteTo(buf, off) pattern |
| AC-6 | AIGP attribute pool dedup | Same AIGP values share pool entries |
| AC-7 | Malformed AIGP (truncated TLV) | Error returned, attribute treated as malformed |
| AC-8 | ExaBGP config with AIGP | Migrated to ze syntax |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAIGP` | `internal/.../attribute/aigp_test.go` | Parse AIGP TLV from wire bytes | |
| `TestParseAIGPMultipleTLVs` | `internal/.../attribute/aigp_test.go` | Multiple TLVs including unknown types | |
| `TestParseAIGPMalformed` | `internal/.../attribute/aigp_test.go` | Truncated/invalid TLV handling | |
| `TestAIGPWriteTo` | `internal/.../attribute/aigp_test.go` | Buffer-first encoding | |
| `TestAIGPJSON` | `internal/.../attribute/aigp_test.go` | JSON marshal/unmarshal | |
| `TestAIGPLen` | `internal/.../attribute/aigp_test.go` | Correct length calculation | |
| `TestAIGPPoolDedup` | `internal/.../attribute/aigp_test.go` | Pool dedup for identical values | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AIGP metric | 0 - 2^64-1 | 0xFFFFFFFFFFFFFFFF | N/A (unsigned) | N/A (unsigned) |
| TLV length | 11 (type 1) | 11 | 10 (truncated) | N/A (variable) |
| TLV type | 1 (known) | 1 | 0 (reserved?) | 2+ (unknown, opaque) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `decode-aigp` | `test/decode/*.ci` | Hex input with AIGP -> JSON output with metric | |
| `encode-aigp` | `test/encode/*.ci` | Config with AIGP route -> hex output with AIGP | |
| `api-aigp` | `test/plugin/*.ci` | API command `aigp set 100` -> wire bytes | |

### Future (if deferring any tests)
- AIGP-aware best-path selection tests (out of scope, RIB plugin concern)
- Per-peer AIGP policy tests (out of scope)

## Files to Modify
- `internal/component/bgp/attribute/wire.go` - register AIGP parser in dispatch table
- `internal/component/bgp/attribute/attribute.go` - if any constant additions needed
- `docs/architecture/wire/attributes.md` - document AIGP parsing support

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A (attribute, not RPC) |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | if `aigp` API command needs wiring |
| CLI usage/help text | [ ] | if `aigp` API command needs wiring |
| API commands doc | [x] | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/decode/*.ci`, `test/encode/*.ci` |

## Files to Create
- `internal/component/bgp/attribute/aigp.go` - AIGP type, parser, WriteTo, JSON, Len
- `internal/component/bgp/attribute/aigp_test.go` - unit tests
- `test/decode/aigp-*.ci` - decode functional tests
- `test/encode/aigp-*.ci` - encode functional tests

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

1. **Phase: RFC summary** -- Create `rfc/short/rfc7311.md` with AIGP spec summary
   - Verify: summary exists and covers TLV format, propagation rules, flags
2. **Phase: Wire parsing** -- Parse AIGP TLV structure from wire bytes
   - Tests: `TestParseAIGP`, `TestParseAIGPMultipleTLVs`, `TestParseAIGPMalformed`
   - Files: `aigp.go`, `wire.go` (register parser)
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Wire encoding** -- Buffer-first WriteTo encoding
   - Tests: `TestAIGPWriteTo`, `TestAIGPLen`
   - Files: `aigp.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: JSON formatting** -- Kebab-case JSON marshal/unmarshal
   - Tests: `TestAIGPJSON`
   - Files: `aigp.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Pool dedup** -- Register AIGP in per-attribute-type pool
   - Tests: `TestAIGPPoolDedup`
   - Files: pool registration
   - Verify: tests fail -> implement -> tests pass
6. **Phase: API command** -- Parse `aigp set <value>` command syntax
   - Tests: API-level tests
   - Files: command parser
   - Verify: tests fail -> implement -> tests pass
7. **Functional tests** -- Create .ci tests for decode, encode, API
8. **RFC refs** -- Add `// RFC 7311 Section X.Y` comments
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | TLV parsing handles all edge cases (truncated, zero-length, unknown types) |
| Naming | JSON key is `"aigp"` (kebab-case), Go type follows attribute naming pattern |
| Data flow | Parser registered in wire.go dispatch, encoder uses buffer-first |
| Rule: buffer-first | WriteTo(buf, off) int, no append, no make([]byte) in encoding |
| Rule: lazy-first | No eager parsing of unknown TLV types |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `aigp.go` exists with parser + encoder | `ls internal/component/bgp/attribute/aigp.go` |
| Parser registered in wire.go | `grep -n AIGP internal/component/bgp/attribute/wire.go` |
| Unit tests pass | `go test -race ./internal/component/bgp/attribute/... -run AIGP -v` |
| Functional tests exist | `ls test/decode/aigp-*.ci test/encode/aigp-*.ci` |
| RFC summary exists | `ls rfc/short/rfc7311.md` |
| JSON uses kebab-case | `grep aigp internal/component/bgp/attribute/aigp.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | TLV length validation to prevent read past end of attribute bytes |
| Resource exhaustion | Bounded TLV iteration (cannot loop forever on malformed data) |
| Integer overflow | 64-bit metric value handled correctly, TLV length arithmetic checked |

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

Add `// RFC 7311 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, TLV format constraints, propagation rules.

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
- [ ] AC-1..AC-8 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
