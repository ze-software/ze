# Spec: bugfix-bgp-nlri-strictness

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-plugins.md` finding BPLUG-001
3. `internal/component/bgp/plugins/nlri/labeled/encode.go`
4. `internal/component/bgp/plugins/nlri/mup/config.go`
5. `internal/component/bgp/plugins/nlri/mup/encode.go`
6. `internal/component/bgp/plugins/nlri/mvpn/config.go`
7. `internal/component/bgp/plugins/nlri/vpls/encode.go`
8. `internal/component/plugin/registry/registry.go`
9. `internal/component/bgp/plugins/cmd/update/update_text_nlri.go`

## Task

Fix BPLUG-001. BGP NLRI family encode and config parsers must reject unknown keys and dangling tokens instead of silently ignoring operator input. The fix covers labeled unicast, MUP, MVPN, and VPLS parser paths identified by the review.

## Required Reading

### Source Finding
- [ ] `plan/review-bug-review-bgp-plugins.md` - BPLUG-001 evidence and regression plan
  -> Decision: this is an input validation bug across CLI, RPC, and config paths.
  -> Constraint: exact-or-reject applies to user-provided family-specific tokens.

### Architecture and Rules
- [ ] `ai/rules/plugin-self-containment.md`
  -> Constraint: parsers stay in owning NLRI family packages.
- [ ] `ai/patterns/bgp-family.md`
  -> Constraint: family registration and parser behavior should be consistent across encode/config paths.
- [ ] `ai/rules/testing.md`
  -> Constraint: tests must exercise behavior through real parser entry points, not only helper strings.

## Current Behavior

**Source files to read:**
- [ ] `internal/component/bgp/plugins/nlri/labeled/encode.go:120-153` - unknown encode tokens have no default error.
- [ ] `internal/component/bgp/plugins/nlri/mup/encode.go:63-123` - unknown encode tokens and dangling tokens can be skipped.
- [ ] `internal/component/bgp/plugins/nlri/mup/config.go:108-115` - config parser copies known keys and drops unknown or dangling input.
- [ ] `internal/component/bgp/plugins/nlri/mvpn/config.go:59-75` - config parser handles known keys only.
- [ ] `internal/component/bgp/plugins/nlri/vpls/encode.go:43-97` - in-process encoder lacks the strictness present in route-command parser code.
- [ ] `internal/component/plugin/registry/registry.go:708-715` - registry dispatches to registered in-process NLRI encoders.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text_nlri.go:341-350` - update text path reaches registry encode.

**Behavior to preserve:**
- Valid labeled, MUP, MVPN, and VPLS inputs keep identical encoded bytes.
- Existing required-field errors remain at least as specific as today.
- Neighboring strict parsers, for example SR-Policy and VPLS route-command parsing, remain unchanged except for shared helpers if introduced.

**Behavior to change:**
- Unknown keys return errors naming the bad token.
- A key without a required value returns an error naming the dangling token.
- Config route parsers and in-process NLRI encoders share strict token-pair semantics where their grammars are key/value based.

## Data Flow

### Entry Points
- `ze-plugin-engine:encode-nlri` RPC/API calls.
- `ze bgp peer ... update text ... nlri <family> ...` CLI path.
- BGP route config parsers for affected plugin families.

### Transformation Path
1. User supplies family-specific tokens.
2. Registry or config parser dispatches to the owning NLRI plugin.
3. Fixed parser validates the token stream before building bytes.
4. Unknown or dangling tokens return an error before any route is encoded or applied.
5. Valid tokens build the same NLRI bytes as before.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/update text -> registry encoder | `encodeViaRegistry` | [ ] labeled unknown-token functional or command parser test |
| RPC/API -> registry encoder | `registry.EncodeNLRIByFamily` | [ ] per-family unit tests |
| config tree -> config route parser | family config parser | [ ] MUP and MVPN config unknown-token tests |

### Architectural Verification
- [ ] Parser changes live in owner packages.
- [ ] Errors carry enough context for operators to fix input.
- [ ] No central BGP parser hardcodes family-specific keys.
- [ ] Existing valid fixtures continue to pass.

## Risks and Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Token grammars are key/value pairs for all affected paths | source loops advance by pairs | handle family-specific positional grammar separately | owner-package valid and invalid parser tests | confirmed |
| A-2 | Unknown tokens should always be errors, not extension passthrough | Ze exact-or-reject rules | document an explicit extension namespace | review and strictness tests | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Existing tests rely on ignored tokens | regression fails | update tests only if they were asserting broken behavior, not defaults |
| R-2 | Error messages differ across families | usability review flags inconsistency | add shared helper local to NLRI packages or small internal helper |
| R-3 | Config parser and encode parser drift remains | one path strict, another silent | add tests for every path in finding |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `EncodeNLRIByFamily("ipv4/mpls-label", ...)` | -> | labeled encoder | `TestEncodeLabeledRejectsUnknownToken` |
| `EncodeNLRIByFamily("ipv4/mup", ...)` | -> | MUP encoder | `TestEncodeMUPRejectsUnknownAndDanglingToken` |
| BGP MUP route config parser | -> | MUP config parser | `TestMUPConfigRejectsUnknownAndDanglingToken` |
| BGP MVPN route config parser | -> | MVPN config parser | `TestMVPNConfigRejectsUnknownAndDanglingToken` |
| `EncodeNLRIByFamily("l2vpn/vpls", ...)` | -> | VPLS encoder | `TestEncodeVPLSRejectsUnknownAndDanglingToken` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | labeled unicast encode args include valid `prefix`, `label`, plus `bogus value` | encode returns error naming `bogus` |
| AC-2 | MUP encode args include an unknown token or dangling final key | encode returns an error before producing bytes |
| AC-3 | MUP config route includes unknown or dangling token | config parse fails and names offending token |
| AC-4 | MVPN config route includes unknown or dangling token | config parse fails and names offending token |
| AC-5 | VPLS encode args include unknown or dangling token | encode returns an error before producing bytes |
| AC-6 | existing valid fixtures for affected families | encoded bytes unchanged |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeLabeledRejectsUnknownToken` | `internal/component/bgp/plugins/nlri/labeled/encode_label_test.go` | AC-1 | |
| `TestEncodeMUPRejectsUnknownAndDanglingToken` | `internal/component/bgp/plugins/nlri/mup/config_test.go` or new encode test | AC-2 | |
| `TestMUPConfigRejectsUnknownAndDanglingToken` | `internal/component/bgp/plugins/nlri/mup/config_test.go` | AC-3 | |
| `TestMVPNConfigRejectsUnknownAndDanglingToken` | `internal/component/bgp/plugins/nlri/mvpn/config_test.go` | AC-4 | |
| `TestEncodeVPLSRejectsUnknownAndDanglingToken` | `internal/component/bgp/plugins/nlri/vpls/config_test.go` or encode test | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| update text rejects labeled unknown token | `test/encode/` or `test/bgp/` existing update-text suite | operator typo fails before route is sent | |
| config parse rejects MUP or MVPN typo | existing BGP config functional suite if present | config load fails on misspelled route key | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| not required | - | - | local input validation, no on-wire behavior for invalid input | |

## Files to Modify

- `internal/component/bgp/plugins/nlri/labeled/encode.go`
- `internal/component/bgp/plugins/nlri/mup/config.go`
- `internal/component/bgp/plugins/nlri/mup/encode.go`
- `internal/component/bgp/plugins/nlri/mvpn/config.go`
- `internal/component/bgp/plugins/nlri/vpls/encode.go`
- Existing tests in the same owner packages.
- Possibly a small shared token helper under an owner-appropriate BGP plugin internal package if duplication becomes error-prone.

## Files to Create

- New `*_test.go` files only if existing package tests cannot hold the new cases.

## Implementation Steps

1. Add failing unknown-token and dangling-token tests for every affected parser path.
2. Add a small strict token-pair helper if it reduces duplicated bugs without centralizing family policy.
3. Update each parser to return an error on unknown keys and missing values.
4. Re-run valid fixture tests to prove output bytes are unchanged.
5. Run targeted package tests, affected functional encode/config tests, and `make ze-lint-changed`.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Correctness | invalid input fails before bytes are produced |
| User experience | error names the token and family path where possible |
| Ownership | family logic remains in owning NLRI packages |
| Regression | valid fixtures unchanged |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| labeled strictness | `go test -run TestEncodeLabeledRejectsUnknownToken ./internal/component/bgp/plugins/nlri/labeled` |
| MUP strictness | `go test -run 'Test.*MUP.*Rejects' ./internal/component/bgp/plugins/nlri/mup` |
| MVPN strictness | `go test -run 'Test.*MVPN.*Rejects' ./internal/component/bgp/plugins/nlri/mvpn` |
| VPLS strictness | `go test -run 'Test.*VPLS.*Rejects' ./internal/component/bgp/plugins/nlri/vpls` |
| lint | `make ze-lint-changed` |

## Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Silent config loss | unknown fields cannot be dropped |
| CLI/API validation | bad tokens fail at command time |
| Wire safety | invalid input does not emit partial route bytes |

## Failure Routing

| Failure | Route To |
|---------|----------|
| one family has positional grammar | add family-specific validation, do not force key/value helper |
| functional harness lacks negative config support | keep unit tests and add closest parser-level functional test |

## Design Insights

- Silent parser fall-through is worse than an unsupported feature because the command appears to work while dropping operator intent.

## Core Insight

Every runtime route token is operator data. Unknown operator data must be rejected at the owning parser boundary.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Make owner parsers strict | central post-parse validation | owner packages understand legal family keys |
| Test every affected entry point | one helper test | review finding spans CLI/RPC and config paths |

## Known Limitations

- NLRI strict token validation is implemented for the affected labeled, MUP, MVPN, and VPLS parser paths.
- Decode-only family encode completeness is tracked separately and not included here.

## Implementation Summary

### What Was Implemented
- Labeled, MUP, MVPN, and VPLS owner parsers now reject unknown and dangling tokens with errors naming the offending token.

### Bugs Found/Fixed
- BPLUG-001 documented for implementation.

### Documentation Updates
- No user docs required unless error text or CLI grammar documentation exists for these families.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BPLUG-001 | Done | this spec | Covers every parser named by review |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-6 | Done | owner-package strictness tests | Invalid input rejects; valid fixtures unchanged |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| affected family strictness tests | Done | owner packages | `go test` owner packages passed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| affected NLRI parser files | Done | implementation targets updated |

### Audit Summary
- Total items: 1 accepted finding converted to a fix spec.
- Done: NLRI strictness implementation and tests.
- Partial: none for listed parser paths.
- Skipped: no approved scope reduction.
- Changed: labeled, MUP, MVPN, and VPLS owner parsers and tests.

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BPLUG-001 has complete remediation plan | spec artifact | this file lists affected parsers, entry points, ACs, and tests |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | generated from BPLUG-001 | `plan/review-bug-review-bgp-plugins.md` | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec covers every affected parser path named by BPLUG-001

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-nlri-strictness.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| encode/config parser entry points | planned tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | validation fix only unless CLI docs exist | yes |
