# Spec: ike-post-quantum

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/ike/crypto/transform.go` - DH group registry
4. `internal/component/ike/crypto/dh.go` - DH exchange implementation

## Task

Ze's native IKEv2 implementation has a closed DH group registry with only 3 groups
(MODP-2048/group 14, ECP-256/group 19, ECP-384/group 20). There is no post-quantum
key exchange support (ML-KEM/Kyber, hybrid key exchange per RFC 9370). ESN constants
are defined on the wire but hardcoded off in negotiation.

This is a tracking spec for future PQ readiness. Ze's native IKEv2 requires its own
implementation.

## Required Reading

### Architecture Docs
- [ ] `internal/component/ike/` - IKE component structure
- [ ] `internal/component/ike/crypto/` - cryptographic primitives

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2
- [ ] `rfc/short/rfc9370.md` - Multiple Key Exchanges in IKEv2
- [ ] `rfc/short/rfc4304.md` - Extended Sequence Numbers (ESN) for IPsec

**Key insights:**
- `crypto/transform.go`: DH registry has 3 entries (groups 14, 19, 20)
- `crypto/dh.go`: `NewDHExchange()` returns `ErrUnsupportedGroup` for unknown groups
- `engine/initiator.go`: explicitly comments "ESN not used for IKE proposals", hardcodes ID 0
- `TransformTypeESN` constant exists in `transform.go` and `wire/payload_sa.go`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/crypto/transform.go` - transform/DH group definitions
- [ ] `internal/component/ike/crypto/dh.go` - DH exchange implementation
- [ ] `internal/component/ike/engine/initiator.go` - IKE SA proposal construction

**Behavior to preserve:**
- Existing DH groups (14, 19, 20) continue to work
- ESN wire parsing/round-tripping
- All existing IKE negotiation

**Behavior to change:**
- Add ML-KEM key exchange to DH registry
- Add hybrid key exchange support (RFC 9370)
- Enable ESN negotiation when peer supports it
- Expose PQ and ESN options in YANG config

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE SA negotiation (initiator and responder)
- YANG config for DH group / PQ algorithm selection

### Transformation Path
1. Config selects DH groups / PQ algorithms
2. IKE SA proposal includes selected transforms
3. `NewDHExchange()` instantiates the selected algorithm
4. Key exchange completes with PQ or hybrid method

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> IKE engine | YANG tree resolution | [ ] |
| IKE engine -> crypto | `NewDHExchange()` dispatch | [ ] |

### Integration Points
- `crypto/dh.go` DH group registry
- `engine/initiator.go` proposal construction
- YANG config for algorithm selection

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Go has mature ML-KEM libraries available | Go 1.23+ has `crypto/mlkem` | Would need vendored implementation | Check Go version and stdlib availability | unvalidated |
| A-2 | RFC 9370 hybrid approach is the right path | Industry consensus for IKEv2 PQ migration | Alternative approaches may emerge | Monitor IETF drafts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PQ algorithm landscape still evolving | NIST finalizes new algorithms | Design registry to be extensible |
| R-2 | Performance impact of PQ key exchange | Benchmarks show unacceptable latency | Offer hybrid as optional, not default |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE SA negotiation with PQ group | -> | ML-KEM key exchange completes | (fill during design) |
| ESN enabled in config | -> | ESN transform negotiated | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ML-KEM group configured | IKE SA uses ML-KEM key exchange |
| AC-2 | Hybrid PQ configured (RFC 9370) | IKE SA uses hybrid classical+PQ exchange |
| AC-3 | ESN enabled | IKE SA negotiates ESN with capable peers |
| AC-4 | PQ not configured | Existing classical DH groups used (backward compatible) |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables PQ key exchange in IKE config | YANG -> engine -> ML-KEM exchange -> SA established | (fill during design) |
| 2 | Enables ESN for IPsec SA | YANG -> engine -> ESN negotiated -> extended counters used | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHRegistryMLKEM` | `internal/component/ike/crypto/dh_test.go` | ML-KEM group registered; `NewDHExchange()` returns a working exchange for it | |
| `TestHybridKeyExchange` | `internal/component/ike/crypto/dh_test.go` | RFC 9370 hybrid exchange combines classical and PQ shared secrets | |
| `TestClassicalGroupsUnchanged` | `internal/component/ike/crypto/dh_test.go` | Groups 14, 19, 20 continue to work when PQ is not configured (AC-4) | |
| `TestProposalIncludesESN` | `internal/component/ike/engine/initiator_test.go` | ESN transform included in the SA proposal when enabled in config | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-pq-mlkem` | `test/ipsec/ike-pq-mlkem.ci` | ML-KEM group configured; IKE SA established using PQ key exchange (AC-1, AC-2) | |
| `ike-esn` | `test/ipsec/ike-esn.ci` | ESN enabled; SA negotiates ESN with a capable peer (AC-3) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ike-pq-strongswan` | `test/ipsec-interop/scenarios/` | strongSwan | Hybrid/PQ key exchange interoperates with another IKEv2 implementation | |

## Files to Modify
- `internal/component/ike/crypto/transform.go` - add ML-KEM / hybrid entries to the DH group registry
- `internal/component/ike/crypto/dh.go` - ML-KEM and hybrid key exchange implementations behind `NewDHExchange()`
- `internal/component/ike/engine/initiator.go` - include configured PQ groups and ESN transform in SA proposals
- `internal/component/ike/yang/` - expose PQ algorithm and ESN selection in config

## Files to Create
- `test/ipsec/ike-pq-mlkem.ci` - functional test for PQ key exchange
- `test/ipsec/ike-esn.ci` - functional test for ESN negotiation

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - register ML-KEM in the DH group registry and the YANG config leaf; failing wiring test
   - Tests: `TestDHRegistryMLKEM` (initially failing)
   - Files: `transform.go`, `dh.go`, YANG schema
   - Verify: configured group reaches `NewDHExchange()`; exchange is a stub that fails
2. **Phase: ML-KEM key exchange** - implement the PQ exchange
   - Tests: `TestDHRegistryMLKEM`, `TestClassicalGroupsUnchanged`
   - Files: `dh.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: Hybrid key exchange (RFC 9370)** - multiple key exchanges in one negotiation
   - Tests: `TestHybridKeyExchange`
   - Files: `dh.go`, `initiator.go`
   - Verify: hybrid SA established in unit tests
4. **Phase: ESN negotiation** - enable the existing ESN transform in proposals when configured
   - Tests: `TestProposalIncludesESN`, `ike-esn.ci`
   - Files: `initiator.go`, YANG schema
   - Verify: ESN negotiated with capable peers
5. **Functional + interop tests** - `ike-pq-mlkem.ci`, `ike-esn.ci`, strongSwan interop scenario
6. **Full verification** - `make ze-verify`

## Known Limitations
- Large scope: requires cryptographic implementation work
- PQ algorithm landscape still evolving (NIST ML-KEM finalized, but IKEv2 integration drafts in progress)
- ESN is simpler and could be split into a separate spec

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Interop test with strongSwan passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
