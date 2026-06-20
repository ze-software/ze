# Spec: bugfix-bgp-srpolicy-encode

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-bgp-plugins.md` finding BPLUG-002
3. `internal/component/bgp/plugins/nlri/srpolicy/register.go`
4. `internal/component/bgp/plugins/nlri/srpolicy/config.go`
5. `internal/component/bgp/cli/encode.go`
6. `internal/component/plugin/registry/registry.go`
7. `rfc/short/rfc9830.md`

## Task

Fix BPLUG-002. SR-Policy is a registered NLRI family with decode and config parsing, but the canonical `ze bgp encode` route encoder path cannot encode it. Wire SR-Policy into the same in-process NLRI and route encoder chain used by other encodable NLRI families, with tests proving canonical CLI/registry encode works for IPv4 and IPv6 SR-Policy.

## Required Reading

### Source Finding
- [ ] `plan/review-bug-review-bgp-plugins.md` - BPLUG-002 evidence and regression plan
  -> Decision: this is a family-chain wiring bug.
  -> Constraint: the fix must use the owning SR-Policy package, not central special cases.

### Architecture and Rules
- [ ] `ai/patterns/bgp-family.md`
  -> Constraint: registered families expose decode, encode, route encode, and config links consistently when the feature is supported.
- [ ] `ai/rules/plugin-self-containment.md`
  -> Constraint: SR-Policy encoder logic belongs in `internal/component/bgp/plugins/nlri/srpolicy`.
- [ ] `ai/rules/testing.md`
  -> Constraint: tests prove the user-facing encode path, not only registration fields.

### RFC Summary
- [ ] `rfc/short/rfc9830.md`
  -> Constraint: SR-Policy NLRI uses SAFI 73 and the RFC 9830 key fields. Preserve segment list and tunnel attribute encoding rules used by existing config parser.

## Current Behavior

**Source files to read:**
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/register.go:28-37` - registers families, decoder, and config route parser, but no in-process NLRI or route encoder.
- [ ] `internal/component/bgp/plugins/nlri/srpolicy/config.go:40-157` - config parser can build SR-Policy route bytes from user route content.
- [ ] `internal/component/bgp/cli/encode.go:139-155` - non-unicast canonical encode uses `RouteEncoderByFamily` and returns `unsupported family` when missing.
- [ ] `internal/component/plugin/registry/registry.go:708-727` - nil encoder fields produce no-encoder or no-route-encoder errors.
- [ ] `test/exabgp-compat/encoding/conf-sr-policy.ci` - compatibility path has SR-Policy encoding coverage.
- [ ] `test/decode/bgp-srpolicy-1.ci` and `test/decode/bgp-srpolicy-2.ci` - decode-only user fixtures exist.

**Behavior to preserve:**
- Existing SR-Policy decode tests keep passing.
- Existing config route parser semantics and valid config bytes remain unchanged.
- ExaBGP compatibility encoding remains unchanged unless shared helpers are used.

**Behavior to change:**
- `registry.EncodeNLRIByFamily` for `ipv4/sr-policy` and `ipv6/sr-policy` returns encoded bytes instead of `no NLRI encoder`.
- `registry.RouteEncoderByFamily` for SR-Policy returns a non-nil route encoder.
- `ze bgp encode --family ipv4/sr-policy ...` and IPv6 equivalent use the canonical encode path successfully.

## Data Flow

### Entry Points
- Canonical `ze bgp encode` CLI.
- Registry route encoder lookup.
- Registry NLRI encoder lookup.

### Transformation Path
1. User selects SR-Policy family.
2. `parseEncodingFamily` resolves registered family.
3. Non-unicast encode path calls `registry.RouteEncoderByFamily`.
4. Fixed SR-Policy registration provides route encoder and optional NLRI-only encoder.
5. Encoder reuses config parser or shared encode helpers to build RFC 9830-compliant bytes.
6. CLI outputs UPDATE bytes through the normal encode flow.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI family selection -> registry | `encode.go` | [ ] functional `test/encode/bgp-srpolicy-*.ci` |
| registry -> SR-Policy owner package | `RouteEncoderByFamily` | [ ] unit test imports registration and sees non-nil encoder |
| config grammar -> route encoder | shared parser/helper | [ ] config and encode parity test |

### Architectural Verification
- [ ] No central special case for SR-Policy in `encode.go`.
- [ ] Registration lives in SR-Policy package.
- [ ] Route encoder and config parser share encode logic or have parity tests.
- [ ] IPv4 and IPv6 families are both covered.

## Risks and Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Existing config parser grammar is acceptable for canonical encode | source already builds SR-Policy route content | define explicit encode grammar and conversion | CLI tests | unvalidated |
| A-2 | NLRI-only encoder can share route parser key fields without tunnel attributes | registry has separate NLRI and route encoder concepts | register only route encoder if NLRI-only grammar is not meaningful | unit tests and review | unvalidated |
| A-3 | ExaBGP compatibility fixture bytes are suitable expected output references | existing fixture encodes SR-Policy | derive expected bytes from RFC 9830 fields | encode tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Canonical CLI syntax differs from config syntax | tests unclear or user-facing grammar conflict | document grammar in test names and reuse parser only if syntax matches |
| R-2 | NLRI encoder omits required attributes for full route | UPDATE bytes lack tunnel attribute | route encoder test asserts full UPDATE content |
| R-3 | IPv6 path gets IPv4-only assumptions | IPv6 test fails | explicit IPv4 and IPv6 fixtures |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `registry.RouteEncoderByFamily("ipv4/sr-policy")` | -> | SR-Policy route encoder | `TestSRPolicyRouteEncoderRegistered` |
| `registry.EncodeNLRIByFamily("ipv4/sr-policy", ...)` | -> | SR-Policy NLRI encoder if meaningful | `TestSRPolicyNLRIEncoderRegistered` |
| `ze bgp encode --family ipv4/sr-policy ...` | -> | canonical CLI route encoder | `test/encode/bgp-srpolicy-1.ci` |
| `ze bgp encode --family ipv6/sr-policy ...` | -> | canonical CLI route encoder | `test/encode/bgp-srpolicy-2.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Registry lookup for IPv4 and IPv6 SR-Policy route encoder | non-nil encoder returned |
| AC-2 | Canonical `ze bgp encode` SR-Policy IPv4 fixture | command succeeds and bytes match expected SR-Policy UPDATE |
| AC-3 | Canonical `ze bgp encode` SR-Policy IPv6 fixture | command succeeds and bytes match expected SR-Policy UPDATE |
| AC-4 | Invalid SR-Policy encode input | exact error naming missing or invalid field |
| AC-5 | Existing decode and ExaBGP compatibility fixtures | unchanged results |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSRPolicyRouteEncoderRegistered` | `internal/component/bgp/plugins/nlri/srpolicy/encode_test.go` | AC-1 | |
| `TestSRPolicyEncodeIPv4` | same | AC-2 core bytes | |
| `TestSRPolicyEncodeIPv6` | same | AC-3 core bytes | |
| `TestSRPolicyEncodeRejectsInvalidInput` | same | AC-4 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-srpolicy-1.ci` | `test/encode/` | user encodes IPv4 SR-Policy via canonical CLI | |
| `bgp-srpolicy-2.ci` | `test/encode/` | user encodes IPv6 SR-Policy via canonical CLI | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| optional SR-Policy encode/decode round trip | `test/interop/scenarios/` if available | GoBGP or FRR with SR-Policy support | peer accepts encoded route | not required for initial fix |

## Files to Modify

- `internal/component/bgp/plugins/nlri/srpolicy/register.go` - register in-process encoder fields.
- `internal/component/bgp/plugins/nlri/srpolicy/config.go` and/or new `encode.go` - share parser/encoder logic.
- `internal/component/bgp/plugins/nlri/srpolicy/encode_test.go` - unit coverage.
- `test/encode/bgp-srpolicy-1.ci` and `test/encode/bgp-srpolicy-2.ci` - functional CLI coverage.

## Files to Create

- `internal/component/bgp/plugins/nlri/srpolicy/encode.go` if no suitable encoder file exists.
- `internal/component/bgp/plugins/nlri/srpolicy/encode_test.go` if no suitable test file exists.
- `test/encode/bgp-srpolicy-1.ci`
- `test/encode/bgp-srpolicy-2.ci`

## Implementation Steps

1. Add failing registry and CLI tests showing SR-Policy encode currently reports unsupported family.
2. Factor SR-Policy route byte construction so config and encode paths share field validation.
3. Register `InProcessRouteEncoder` and, if grammar is meaningful for NLRI-only input, `InProcessNLRIEncoder`.
4. Add IPv4 and IPv6 canonical encode fixtures.
5. Run SR-Policy unit tests, encode functional tests, existing SR-Policy decode fixtures, ExaBGP compatibility fixture, and `make ze-lint-changed`.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Correctness | bytes match RFC 9830 and existing compatibility output |
| Wiring | registry and CLI use owner package encoder, not central special case |
| Errors | invalid input rejects exact-or-reject |
| Regression | decode and config paths unchanged |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| route encoder registration | `go test -run TestSRPolicyRouteEncoderRegistered ./internal/component/bgp/plugins/nlri/srpolicy` |
| IPv4 encode | `go test -run TestSRPolicyEncodeIPv4 ./internal/component/bgp/plugins/nlri/srpolicy` and `make ze-test-encode` target for fixture if available |
| IPv6 encode | `go test -run TestSRPolicyEncodeIPv6 ./internal/component/bgp/plugins/nlri/srpolicy` and encode fixture |
| lint | `make ze-lint-changed` |

## Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Input validation | unsupported or malformed SR-Policy fields fail before bytes emitted |
| Central coupling | no SR-Policy grammar in core CLI package |
| Wire correctness | IPv4/IPv6 family and SAFI 73 fields match RFC 9830 |

## Failure Routing

| Failure | Route To |
|---------|----------|
| NLRI-only encoder is not meaningful without attributes | document and register route encoder only, with tests proving registry route path |
| canonical CLI syntax is underspecified | create a grammar note in the test and reuse config parser syntax only after review |
| expected bytes differ from ExaBGP compatibility | compare against RFC 9830 and existing decoder output before changing fixture |

## Design Insights

- Registered family visibility is a contract. If a family is partially wired, users see inconsistent decode, config, and encode behavior.

## Core Insight

SR-Policy encode support belongs in the SR-Policy family chain, not in a CLI fallback.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Register owner package encoder | special-case SR-Policy in `encode.go` | matches BGP family registration pattern |
| Cover IPv4 and IPv6 | one-family smoke test | both families are registered and can drift separately |

## Known Limitations

- This spec does not implement the fix.
- Decode-only encode gaps for BGP-LS, RTC, and MVPN remain not promoted without product decision.

## Implementation Summary

### What Was Implemented
- Fix spec only. Production code is unchanged.

### Bugs Found/Fixed
- BPLUG-002 documented for implementation.

### Documentation Updates
- No user docs required unless SR-Policy encode syntax is added to CLI documentation during implementation.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for BPLUG-002 | Done | this spec | Includes registry and canonical CLI tests |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-5 | Planned | tests listed above | To be satisfied by implementation owner |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| SR-Policy registry and encode tests | Planned | owner package and `test/encode` | Not run by review program |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| SR-Policy register/config/encode files | Planned | implementation targets |

### Audit Summary
- Total items: 1 accepted finding converted to a fix spec.
- Done: fix spec created.
- Partial: implementation pending by design.
- Skipped: no production code changes in review program.
- Changed: new spec file.

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BPLUG-002 has implementable remediation path | spec artifact | this file names registration, CLI, registry, and fixture tests |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | generated from BPLUG-002 | `plan/review-bug-review-bgp-plugins.md` | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec covers canonical encode route, registry route, and owner package registration

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-bgp-srpolicy-encode.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| `ze bgp encode` SR-Policy | planned functional tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-3 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed yet | fix spec only, syntax docs deferred to implementation if syntax changes | yes |
