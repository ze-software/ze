# Spec: ipsec-6 -- IKEv2 Crypto Primitives

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions (native IKEv2)
4. `rfc/short/rfc7296.md` -- IKEv2 key derivation (Sections 2.13-2.14, 3.3)
5. `rfc/short/rfc6071.md` -- IPsec/IKE algorithm roadmap
6. `internal/component/ike/crypto/` -- implementation location

## Task

Implement the IKEv2 cryptographic primitives layer for Ze's native IKEv2 engine.
This is a pure computation package with no state machine, no network I/O, and no
wire encoding. It provides:

- **Diffie-Hellman groups:** DH key pair generation and shared secret computation
  for groups 14 (MODP 2048), 19 (ECP-256), and 20 (ECP-384).
- **PRF functions:** prf-hmac-sha256, prf-hmac-sha384, prf-hmac-sha512.
  Used for SKEYSEED derivation and prf+ key expansion.
- **Encryption algorithms:** ENCR_AES_GCM_16 (128/256 bit keys) and
  ENCR_AES_CBC (128/256 bit keys). AES-GCM is AEAD (no separate integrity);
  AES-CBC requires a separate integrity transform.
- **Integrity algorithms:** AUTH_HMAC_SHA2_256_128, AUTH_HMAC_SHA2_384_192,
  AUTH_HMAC_SHA2_512_256. Used only with non-AEAD ciphers (AES-CBC).
- **SKEYSEED derivation:** RFC 7296 Section 2.14.
  `SKEYSEED = prf(Ni | Nr, g^ir)` for initial IKE SA.
  `SKEYSEED = prf(SK_d_old, g^ir_new | Ni | Nr)` for rekeyed IKE SA.
- **SK_* key hierarchy:** `prf+(SKEYSEED, Ni | Nr | SPIi | SPIr)` producing
  SK_d, SK_ai, SK_ar, SK_ei, SK_er, SK_pi, SK_pr.
- **Child SA key derivation:** `KEYMAT = prf+(SK_d, Ni | Nr)` split into
  ESP encryption and integrity keys for initiator and responder.
- **Proposal negotiation:** Walk remote IKE/ESP proposals, match against local
  policy (first acceptable match wins), reject unsupported algorithms.
- **Transform type registry:** Encryption (1), PRF (2), Integrity (3),
  DH Group (4), ESN (5) with their IANA-assigned algorithm IDs.

This spec is **parallelizable with ipsec-5** (wire format). Neither depends on
the other. The wire codec defines payload structures; this package defines the
crypto operations those payloads parameterize.

Go standard library provides all needed primitives: `crypto/ecdh` for ECP groups,
`math/big` for MODP, `crypto/aes` + `crypto/cipher` for encryption, `crypto/hmac`
+ `crypto/sha256` / `crypto/sha512` for PRF and integrity.

## Required Reading

### Architecture Docs
- [ ] `spec-ipsec-0-umbrella.md` -- native IKEv2 design decisions, crypto engine layer
  -> Decision: Ze implements all crypto natively in Go using standard library
  -> Constraint: no CGo dependencies for crypto; pure Go only
- [ ] `docs/architecture/core-design.md` -- registration pattern
  -> Constraint: transform registry uses registration pattern for algorithm lookup

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- Sections 2.13 (DH), 2.14 (SKEYSEED + SK_*), 3.3 (SA payload, proposal/transform)
  -> Constraint: SKEYSEED = prf(Ni | Nr, g^ir); SK_* = prf+(SKEYSEED, Ni | Nr | SPIi | SPIr)
  -> Constraint: proposal selection: first match wins (responder picks from initiator's list)
- [ ] `rfc/short/rfc6071.md` -- IPsec algorithm requirements, MUST/SHOULD/MAY algorithms
  -> Constraint: MUST implement AES-GCM-16 (128) and HMAC-SHA256. SHOULD implement AES-CBC-128

**Key insights:**
- DH group 14 (MODP 2048) uses math/big; groups 19/20 (ECP) use crypto/ecdh (Go 1.20+)
- prf+ (Section 2.13) is iterated HMAC: T1 = prf(K, S | 0x01), T2 = prf(K, T1 | S | 0x02), ...
- AEAD ciphers (AES-GCM) set integrity algorithm to NONE; non-AEAD (AES-CBC) require explicit integrity
- Proposal negotiation is ordered: initiator lists proposals in preference order, responder picks first acceptable
- Transform IDs are IANA-assigned: ENCR_AES_GCM_16 = 20, ENCR_AES_CBC = 12, PRF_HMAC_SHA2_256 = 5, etc.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/` -- directory does not exist yet (to be created)
  -> Constraint: no existing code; greenfield package
- [ ] `internal/component/ipsec/types.go` -- IKEGroup.Proposals contains algorithm names from config
  -> Constraint: crypto package must map config algorithm names to IANA transform IDs

**Behavior to preserve:**
- No existing crypto code to preserve (greenfield)
- IPsec data model (ipsec-3) algorithm name strings unchanged

**Behavior to change:**
- New `internal/component/ike/crypto/` package with DH, PRF, cipher, key derivation, proposals
- Config algorithm name strings mapped to IANA transform IDs and Go crypto implementations

## Data Flow (MANDATORY)

### Entry Point
- IKE engine (ipsec-7) calls crypto functions during IKE_SA_INIT and IKE_AUTH exchanges
- Config parser (ipsec-3) provides algorithm names; crypto package maps them to implementations

### Transformation Path
1. Config algorithm name string (e.g., "aes128gcm") looked up in transform registry
2. Transform registry returns IANA ID + crypto implementation factory
3. IKE_SA_INIT: DH key pair generated, public value sent; remote public value received, shared secret computed
4. SKEYSEED derived from nonces and DH shared secret via selected PRF
5. SK_* key hierarchy expanded from SKEYSEED via prf+
6. IKE_AUTH: SK_ei/SK_er used to encrypt/decrypt, SK_ai/SK_ar for integrity (if non-AEAD)
7. CREATE_CHILD_SA: KEYMAT derived from SK_d + nonces, split into ESP keys

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config algorithm names to IANA IDs | Transform registry lookup | [ ] |
| IKE engine to crypto | Function calls (no IPC, same process) | [ ] |
| Crypto to Go stdlib | crypto/ecdh, crypto/aes, crypto/hmac, math/big | [ ] |

### Integration Points
- `internal/component/ipsec/types.go` -- algorithm name strings from config
- `internal/component/ike/wire/` (ipsec-5) -- proposal/transform payload structures
- `internal/component/ike/engine/` (ipsec-7) -- consumes DH, key derivation, cipher at runtime

### Architectural Verification
- [ ] No bypassed layers (crypto accessed via registry, not direct construction)
- [ ] No unintended coupling (pure computation, no I/O, no state)
- [ ] No duplicated functionality (Go stdlib provides primitives; this package composes them for IKEv2)
- [ ] Zero-copy preserved where applicable (key material in byte slices, no unnecessary copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config algorithm name "aes128gcm" | -> | Transform registry returns ENCR_AES_GCM_16 with key length 128 | `internal/component/ike/crypto/transform_test.go:TestTransformRegistryLookup` |
| DH group 14 key pair + remote public | -> | Shared secret computed via MODP 2048 | `internal/component/ike/crypto/dh_test.go:TestDHGroup14SharedSecret` |
| Nonces + DH shared secret | -> | SKEYSEED + SK_* derived correctly | `internal/component/ike/crypto/keys_test.go:TestSKEYSEEDDerivation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DH group 14 (MODP 2048) key exchange | Both peers compute identical shared secret from their private key and peer's public value |
| AC-2 | DH group 19 (ECP-256) key exchange | Both peers compute identical shared secret using crypto/ecdh P-256 |
| AC-3 | DH group 20 (ECP-384) key exchange | Both peers compute identical shared secret using crypto/ecdh P-384 |
| AC-4 | Nonces Ni, Nr and DH shared secret g^ir | SKEYSEED = prf(Ni \| Nr, g^ir) matches RFC 7296 Section 2.14 |
| AC-5 | SKEYSEED, nonces, and SPIs | prf+ expansion produces correct SK_d, SK_ai, SK_ar, SK_ei, SK_er, SK_pi, SK_pr with correct lengths for selected algorithms |
| AC-6 | SK_d and nonces (no PFS) | Child SA KEYMAT = prf+(SK_d, Ni \| Nr) split into ESP keys for initiator and responder |
| AC-7 | AES-GCM-16 cipher with key and nonce | Encrypt/decrypt roundtrip produces original plaintext; authentication tag verified |
| AC-8 | AES-CBC cipher with key and IV | Encrypt/decrypt roundtrip with PKCS7 padding produces original plaintext |
| AC-9 | HMAC-SHA256-128 integrity | MAC computed over data; truncated to 128 bits; verification succeeds on valid, fails on tampered |
| AC-10 | Initiator proposal list with 3 IKE proposals | Negotiation selects first proposal acceptable to responder policy |
| AC-11 | Remote proposal with unsupported algorithm | Negotiation rejects with NO_PROPOSAL_CHOSEN |
| AC-12 | Config algorithm name "aes256gcm" | Transform registry returns ENCR_AES_GCM_16 (ID 20) with key length 256 |
| AC-13 | Unknown config algorithm name "chacha20" | Transform registry returns error (unsupported) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHGroup14SharedSecret` | `internal/component/ike/crypto/dh_test.go` | MODP 2048 key exchange produces matching shared secret | |
| `TestDHGroup19SharedSecret` | `internal/component/ike/crypto/dh_test.go` | ECP-256 key exchange produces matching shared secret | |
| `TestDHGroup20SharedSecret` | `internal/component/ike/crypto/dh_test.go` | ECP-384 key exchange produces matching shared secret | |
| `TestPRFHMACSHA256` | `internal/component/ike/crypto/prf_test.go` | PRF output matches expected for known inputs | |
| `TestPRFPlus` | `internal/component/ike/crypto/prf_test.go` | prf+ expansion produces correct length output | |
| `TestSKEYSEEDDerivation` | `internal/component/ike/crypto/keys_test.go` | SKEYSEED from nonces + DH matches expected | |
| `TestSKHierarchy` | `internal/component/ike/crypto/keys_test.go` | SK_d/SK_ai/SK_ar/SK_ei/SK_er/SK_pi/SK_pr correct lengths | |
| `TestChildSAKeymat` | `internal/component/ike/crypto/keys_test.go` | Child SA KEYMAT split into ESP keys correctly | |
| `TestAESGCMEncryptDecrypt` | `internal/component/ike/crypto/cipher_test.go` | AES-GCM roundtrip, tag verification | |
| `TestAESCBCEncryptDecrypt` | `internal/component/ike/crypto/cipher_test.go` | AES-CBC roundtrip with padding | |
| `TestHMACSHA256Integrity` | `internal/component/ike/crypto/cipher_test.go` | MAC compute + verify, truncation correct | |
| `TestProposalNegotiationFirstMatch` | `internal/component/ike/crypto/proposal_test.go` | First acceptable proposal selected | |
| `TestProposalNegotiationNoMatch` | `internal/component/ike/crypto/proposal_test.go` | No match returns NO_PROPOSAL_CHOSEN | |
| `TestTransformRegistryLookup` | `internal/component/ike/crypto/transform_test.go` | Config name to IANA ID mapping | |
| `TestTransformRegistryUnknown` | `internal/component/ike/crypto/transform_test.go` | Unknown algorithm returns error | |
| `TestRekeyedSKEYSEED` | `internal/component/ike/crypto/keys_test.go` | Rekeyed SKEYSEED = prf(SK_d_old, g^ir \| Ni \| Nr) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DH group ID | {14, 19, 20} | 20 | 13 (unsupported) | 21 (unsupported) |
| ENCR transform ID | {12, 20} | 20 (AES-GCM-16) | 11 (unsupported) | 21 (unsupported) |
| PRF transform ID | {5, 6, 7} | 7 (HMAC-SHA512) | 4 (unsupported) | 8 (unsupported) |
| AES key length (bits) | {128, 256} | 256 | 64 (too short) | 512 (unsupported) |
| Nonce length (bytes) | 16-256 | 256 | 15 (too short) | 257 (too long) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-crypto-key-derivation` | `test/ipsec/ike-crypto-key-derivation.ci` | Full SKEYSEED + SK_* derivation from test vectors | |

## Files to Modify
- `internal/component/ipsec/types.go` -- reference: algorithm name strings that crypto package maps

## Files to Create
- `internal/component/ike/crypto/dh.go` -- DH groups 14, 19, 20: key pair generation, shared secret
- `internal/component/ike/crypto/dh_test.go` -- DH group unit tests
- `internal/component/ike/crypto/prf.go` -- PRF functions (HMAC-SHA256/384/512) and prf+ expansion
- `internal/component/ike/crypto/prf_test.go` -- PRF unit tests
- `internal/component/ike/crypto/cipher.go` -- AEAD (AES-GCM) and non-AEAD (AES-CBC) + integrity (HMAC)
- `internal/component/ike/crypto/cipher_test.go` -- cipher unit tests
- `internal/component/ike/crypto/keys.go` -- SKEYSEED derivation, SK_* hierarchy, Child SA KEYMAT
- `internal/component/ike/crypto/keys_test.go` -- key derivation unit tests
- `internal/component/ike/crypto/proposal.go` -- IKE/ESP proposal negotiation (first match wins)
- `internal/component/ike/crypto/proposal_test.go` -- proposal negotiation unit tests
- `internal/component/ike/crypto/transform.go` -- transform type registry: config name to IANA ID + factory
- `internal/component/ike/crypto/transform_test.go` -- transform registry unit tests
- `test/ipsec/ike-crypto-key-derivation.ci` -- functional test for key derivation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] N/A | No config changes in this spec |
| CLI commands/flags | [ ] N/A | No CLI in this spec |
| Editor autocomplete | [ ] N/A | N/A |
| Functional test for new RPC/API | [ ] | `test/ipsec/ike-crypto-key-derivation.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] N/A | Crypto is internal |
| 2 | Config syntax changed? | [ ] N/A | No config changes |
| 3 | CLI command added/changed? | [ ] N/A | No CLI |
| 4 | API/RPC added/changed? | [ ] N/A | N/A |
| 5 | Plugin added/changed? | [ ] N/A | N/A |
| 6 | Has a user guide page? | [ ] N/A | N/A |
| 7 | Wire format changed? | [ ] N/A | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] N/A | N/A |
| 9 | RFC behavior implemented? | [ ] Yes | `rfc/short/rfc7296.md` (Sections 2.13-2.14 key derivation) |
| 10 | Test infrastructure changed? | [ ] N/A | N/A |
| 11 | Affects daemon comparison? | [ ] N/A | Not directly |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/core-design.md` (new ike/crypto package) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- create package, transform registry, failing wiring tests
   - Tests: `TestTransformRegistryLookup`
   - Files: `transform.go` (registry skeleton)
   - Verify: registry exists; lookup returns error for all algorithms (not yet registered)

2. **Phase: DH Groups** -- key pair generation and shared secret computation
   - Tests: `TestDHGroup14SharedSecret`, `TestDHGroup19SharedSecret`, `TestDHGroup20SharedSecret`
   - Files: `dh.go`, `dh_test.go`
   - Verify: two peers compute identical shared secrets for each group

3. **Phase: PRF and prf+** -- HMAC-based PRF functions and key expansion
   - Tests: `TestPRFHMACSHA256`, `TestPRFPlus`
   - Files: `prf.go`, `prf_test.go`
   - Verify: PRF output matches expected; prf+ produces correct length

4. **Phase: Encryption and Integrity** -- AEAD and non-AEAD ciphers, MAC functions
   - Tests: `TestAESGCMEncryptDecrypt`, `TestAESCBCEncryptDecrypt`, `TestHMACSHA256Integrity`
   - Files: `cipher.go`, `cipher_test.go`
   - Verify: encrypt/decrypt roundtrip; integrity verify passes on valid, fails on tampered

5. **Phase: Key Derivation** -- SKEYSEED, SK_* hierarchy, Child SA KEYMAT
   - Tests: `TestSKEYSEEDDerivation`, `TestSKHierarchy`, `TestChildSAKeymat`, `TestRekeyedSKEYSEED`
   - Files: `keys.go`, `keys_test.go`
   - Verify: derived keys have correct lengths for selected algorithms

6. **Phase: Proposal Negotiation** -- match local policy against remote proposals
   - Tests: `TestProposalNegotiationFirstMatch`, `TestProposalNegotiationNoMatch`
   - Files: `proposal.go`, `proposal_test.go`
   - Verify: first acceptable proposal selected; unsupported returns error

7. **Functional tests** -- create after feature works
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-13 has implementation with file:line |
| Correctness | SKEYSEED derivation matches RFC 7296 Section 2.14 exactly; prf+ iteration correct |
| Naming | Transform IDs match IANA assignments; config name mapping consistent with ipsec-3 |
| Data flow | Crypto functions are pure (no I/O, no state mutation); key material in byte slices |
| Rule: buffer-first | Key material uses append-based construction, not fmt.Sprintf |
| Rule: no-sprintf-alloc | No fmt.Sprintf on any path; errors use static strings or append |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| DH groups 14, 19, 20 implemented | `go test -run TestDHGroup` passes |
| PRF and prf+ implemented | `go test -run TestPRF` passes |
| AES-GCM and AES-CBC ciphers | `go test -run TestAES` passes |
| SKEYSEED + SK_* derivation | `go test -run TestSKEYSEED` passes |
| Proposal negotiation | `go test -run TestProposal` passes |
| Transform registry | `go test -run TestTransform` passes |
| Functional test exists | `ls test/ipsec/ike-crypto-key-derivation.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Key material handling | DH private keys and SK_* zeroed after use (overwrite with zeros); never logged |
| Constant-time comparison | HMAC verification uses crypto/subtle.ConstantTimeCompare, not bytes.Equal |
| Nonce uniqueness | AES-GCM nonces must never repeat for the same key; verify nonce generation |
| Side channels | No branching on secret data; use constant-time operations from crypto/subtle |
| Random number generation | All randomness from crypto/rand, never math/rand |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
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

## RFC Documentation

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: SKEYSEED formula, prf+ iteration, SK_* key lengths, proposal selection order, DH group parameters.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-6-ikev2-crypto.md`
- [ ] Summary included in commit
