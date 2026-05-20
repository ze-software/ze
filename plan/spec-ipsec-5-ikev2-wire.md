# Spec: ipsec-5 -- IKEv2 Wire Format Codec

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
4. `rfc/short/rfc7296.md` -- IKEv2 message formats, payload types, encoding rules
5. `rfc/short/rfc7427.md` -- Digital Signature AUTH payload format
6. `ai/rules/buffer-first.md` -- buffer-first encoding pattern (WriteTo, skip-and-backfill)

## Task

Implement the IKEv2 wire format codec: encoding and decoding all IKEv2 message
types defined in RFC 7296 and RFC 7427. This is the foundational layer for Ze's
native IKEv2 engine. Pure encode/decode with no state machine, no network I/O,
and no cryptographic operations.

The codec lives at `internal/component/ike/wire/` and follows Ze's buffer-first
encoding convention: `WriteTo(buf []byte, off int) int` for encoding,
`ReadFrom(data []byte) error` for decoding. No `append()`, no `make([]byte)` in
encoding helpers. Skip-and-backfill for variable-length payloads with fixed-position
length fields.

Parallelizable with ipsec-6 (crypto). Both are consumed by ipsec-7 (engine).

### IKEv2 Message Structure (RFC 7296 Section 3)

An IKEv2 message is:

| Component | Size | Description |
|-----------|------|-------------|
| IKE Header | 28 bytes fixed | SPI pair (16), next payload (1), version (1), exchange type (1), flags (1), message ID (4), length (4) |
| Payload chain | variable | Each payload: generic header (4 bytes: next payload, critical bit, length) + type-specific data |

### Payload Types to Implement

| Type | Value | RFC Section | Key Sub-structures |
|------|-------|-------------|--------------------|
| SA | 33 | 3.3 | Proposal (header + SPI + transforms), Transform (type + ID + attributes) |
| KE | 34 | 3.4 | DH group number (2) + key exchange data |
| IDi | 35 | 3.5 | ID type (1) + identification data |
| IDr | 36 | 3.5 | Same as IDi |
| CERT | 37 | 3.6 | Cert encoding (1) + certificate data |
| CERTREQ | 38 | 3.7 | Cert encoding (1) + certification authority hashes (20 bytes each) |
| AUTH | 39 | 3.8 | Auth method (1) + authentication data. RFC 7427: method 14 adds ASN.1 AlgorithmIdentifier |
| Nonce | 40 | 3.9 | Nonce data (16-256 bytes) |
| Notify | 41 | 3.10 | Protocol ID (1) + SPI size (1) + notify type (2) + SPI + notification data |
| Delete | 42 | 3.11 | Protocol ID (1) + SPI size (1) + num SPIs (2) + SPIs |
| Vendor ID | 43 | 3.12 | Vendor ID data (opaque) |
| TSi | 44 | 3.13 | Number of TSs (1) + Traffic Selector sub-structures (type, protocol, port range, address range) |
| TSr | 45 | 3.13 | Same as TSi |
| SK (Encrypted) | 46 | 3.14 | IV + encrypted payloads + padding + integrity checksum (actual crypto in ipsec-6) |
| CP | 47 | 3.15 | CFG type (1) + Configuration Attributes (type + length + value) |
| EAP | 48 | 3.16 | EAP Code (1) + Identifier (1) + Length (2) + Type (1) + Type-Data |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` -- buffer-first encoding pattern
  -> Constraint: all encoding uses WriteTo(buf, off) int, no append, no make([]byte) in helpers
  -> Decision: skip-and-backfill for payload length fields
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: wire package is a library, not a component; no registration, no init()
- [ ] `spec-ipsec-0-umbrella.md` -- umbrella design decisions
  -> Decision: native IKEv2 in Go, buffer-first wire encoding

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2 message format, all payload types, encoding rules
  -> Constraint: header is 28 bytes fixed, payload generic header is 4 bytes, next-payload chaining
  -> Constraint: payload lengths include the 4-byte generic header
  -> Constraint: critical bit in generic header: if set and type unknown, must reject message
- [ ] `rfc/short/rfc7427.md` -- Digital Signature AUTH payload (method 14)
  -> Constraint: AUTH payload with method 14 has ASN.1 length + AlgorithmIdentifier + signature

**Key insights:**
- IKEv2 uses big-endian (network byte order) throughout
- Payload chain: each payload's "next payload" field points to the type of the following payload; last payload has next=0
- SA payload contains nested Proposal structures, each containing Transform structures with optional attributes
- Traffic Selectors contain sub-structures with address ranges (IPv4: 8 bytes per address; IPv6: 32 bytes)
- SK (Encrypted) payload wraps the real payload chain; decryption handled by crypto layer, not here
- The codec must tolerate unknown payload types (skip by length, check critical bit)
- No IKE state or session context needed for encoding/decoding

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/` -- does not exist yet; this spec creates it
  -> Decision: new package, no existing code to preserve
- [ ] `internal/plugins/bgp/wire/` -- Ze's BGP wire encoding for buffer-first pattern reference
  -> Constraint: follow same WriteTo/ReadFrom conventions; use binary.BigEndian

**Behavior to preserve:**
- No existing IKEv2 code; nothing to preserve

**Behavior to change:**
- New `internal/component/ike/wire/` package with complete IKEv2 codec

## Data Flow (MANDATORY)

### Entry Point
- Encoding: IKE engine (ipsec-7) constructs message struct, calls WriteTo to serialize into buffer
- Decoding: IKE transport (ipsec-7) receives UDP datagram, calls ReadFrom to parse into message struct

### Transformation Path
1. Encoding: Message struct with header + payload slice -> WriteTo writes header (28 bytes) -> iterates payloads, each WriteTo writes generic header + type-specific data -> skip-and-backfill for lengths
2. Decoding: Raw bytes -> ReadFrom parses 28-byte header -> reads next-payload type -> loops: parse generic header (4 bytes), dispatch by type, read type-specific data, advance to next payload

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go structs to wire bytes | WriteTo(buf, off) int | [ ] |
| Wire bytes to Go structs | ReadFrom(data []byte) error | [ ] |
| Codec to crypto (SK payload) | SK payload stores raw ciphertext; crypto layer decrypts externally | [ ] |

### Integration Points
- `internal/component/ike/crypto/` (ipsec-6) -- SK payload decryption/encryption
- `internal/component/ike/engine/` (ipsec-7) -- constructs and parses IKEv2 messages

### Architectural Verification
- [ ] No bypassed layers (codec is a pure library; engine uses it, never raw byte manipulation)
- [ ] No unintended coupling (no imports outside standard library + Ze common types)
- [ ] No duplicated functionality (new package, no overlap with BGP wire code)
- [ ] Zero-copy preserved where applicable (decode references input slice where safe; encode writes into caller buffer)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE engine constructs IKE_SA_INIT message | -> | Header + SA + KE + Nonce payloads encoded via WriteTo | Wired by ipsec-7; unit tests prove roundtrip here |
| IKE transport receives UDP datagram | -> | Header + payload chain decoded via ReadFrom | Wired by ipsec-7; unit tests prove roundtrip here |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKEv2 header struct with valid fields | WriteTo produces 28 bytes; ReadFrom recovers identical struct |
| AC-2 | SA payload with 2 proposals, each with 3 transforms | Encode/decode roundtrip preserves all proposal and transform fields |
| AC-3 | KE payload with DH group 14 and 256-byte key data | Roundtrip preserves group number and key data |
| AC-4 | IDi payload with type ID_FQDN and "vpn.example.com" | Roundtrip preserves ID type and data |
| AC-5 | AUTH payload with method 14 (RFC 7427) and ECDSA signature | Roundtrip preserves method, ASN.1 AlgorithmIdentifier, and signature |
| AC-6 | Notify payload with type NAT_DETECTION_SOURCE_IP and 20-byte data | Roundtrip preserves protocol ID, notify type, and notification data |
| AC-7 | TSi payload with 2 traffic selectors (IPv4 range + IPv6 range) | Roundtrip preserves TS type, protocol, ports, and address ranges |
| AC-8 | CP payload with INTERNAL_IP4_ADDRESS and INTERNAL_IP4_DNS attributes | Roundtrip preserves CFG type and all attribute type/value pairs |
| AC-9 | EAP payload with EAP-Response type 26 (MSCHAPv2) | Roundtrip preserves code, identifier, type, and type-data |
| AC-10 | Full message with header + 4-payload chain (SA, KE, Nonce, Notify) | WriteTo produces valid bytes; ReadFrom recovers all payloads via next-payload chaining |
| AC-11 | Truncated input (header says 100 bytes, only 50 received) | ReadFrom returns error, does not panic |
| AC-12 | Unknown payload type with critical bit clear | ReadFrom skips payload by length, continues chain |
| AC-13 | Unknown payload type with critical bit set | ReadFrom returns error indicating unsupported critical payload |
| AC-14 | CERT payload with X.509 certificate encoding type | Roundtrip preserves encoding type and certificate bytes |
| AC-15 | CERTREQ payload with 3 CA hashes (60 bytes total) | Roundtrip preserves encoding type and all 20-byte hash entries |
| AC-16 | Delete payload with 2 ESP SPIs | Roundtrip preserves protocol ID, SPI size, count, and SPI values |
| AC-17 | SK (Encrypted) payload with raw ciphertext | Roundtrip preserves IV and ciphertext bytes (no decryption in codec) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHeaderRoundtrip` | `internal/component/ike/wire/header_test.go` | 28-byte header encode/decode with all fields | |
| `TestPayloadSARoundtrip` | `internal/component/ike/wire/payload_sa_test.go` | SA with proposals and transforms | |
| `TestPayloadKERoundtrip` | `internal/component/ike/wire/payload_ke_test.go` | KE with DH group and key data | |
| `TestPayloadNonceRoundtrip` | `internal/component/ike/wire/payload_nonce_test.go` | Nonce data roundtrip | |
| `TestPayloadIDRoundtrip` | `internal/component/ike/wire/payload_id_test.go` | IDi and IDr with various ID types | |
| `TestPayloadAuthRoundtrip` | `internal/component/ike/wire/payload_auth_test.go` | AUTH method 1 (RSA), 14 (digital sig) | |
| `TestPayloadCertRoundtrip` | `internal/component/ike/wire/payload_cert_test.go` | CERT with X.509 encoding | |
| `TestPayloadCertReqRoundtrip` | `internal/component/ike/wire/payload_cert_test.go` | CERTREQ with CA hashes | |
| `TestPayloadNotifyRoundtrip` | `internal/component/ike/wire/payload_notify_test.go` | Notify with various types and data | |
| `TestPayloadDeleteRoundtrip` | `internal/component/ike/wire/payload_delete_test.go` | Delete with ESP SPIs | |
| `TestPayloadTSRoundtrip` | `internal/component/ike/wire/payload_ts_test.go` | TSi/TSr with IPv4 and IPv6 selectors | |
| `TestPayloadCPRoundtrip` | `internal/component/ike/wire/payload_cp_test.go` | CP with configuration attributes | |
| `TestPayloadEAPRoundtrip` | `internal/component/ike/wire/payload_eap_test.go` | EAP with various codes and types | |
| `TestPayloadVendorIDRoundtrip` | `internal/component/ike/wire/payload_vendor_test.go` | Vendor ID opaque data | |
| `TestPayloadSKRoundtrip` | `internal/component/ike/wire/payload_sk_test.go` | SK stores raw ciphertext | |
| `TestMessageChainRoundtrip` | `internal/component/ike/wire/message_test.go` | Full message with payload chain | |
| `TestDecodeTruncatedHeader` | `internal/component/ike/wire/header_test.go` | Truncated header returns error | |
| `TestDecodeTruncatedPayload` | `internal/component/ike/wire/payload_test.go` | Truncated payload returns error | |
| `TestDecodeUnknownPayloadSkip` | `internal/component/ike/wire/payload_test.go` | Unknown type, critical=false: skip | |
| `TestDecodeUnknownPayloadCritical` | `internal/component/ike/wire/payload_test.go` | Unknown type, critical=true: error | |
| `TestProposalTransformNested` | `internal/component/ike/wire/payload_sa_test.go` | Nested proposal/transform encode/decode | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Header length | 28 (header only) - 65535 | 65535 | 27 (too short for header) | N/A (uint32 field, but UDP limits) |
| Payload length | 4 (generic header only) - 65535 | 65535 | 3 (too short for generic header) | N/A |
| Nonce length | 16 - 256 | 256 | 15 | 257 |
| SPI size (SA proposal) | 0, 4, 8 | 8 | N/A | 9 |
| Number of proposals | 1 - 255 | 255 | 0 | N/A (uint8) |
| Traffic selector count | 1 - 255 | 255 | 0 | N/A (uint8) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Pure library; wired by ipsec-7 engine. Unit tests are the primary verification. | |

## Files to Modify
- None (new package)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (library, no config) |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A (unit tests only) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A (new wire format, not changing existing) |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7296.md` (wire format implemented) |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new ike component) |

## Files to Create
- `internal/component/ike/wire/header.go` -- IKEv2 header (28 bytes): encode/decode, exchange types, flags
- `internal/component/ike/wire/header_test.go` -- header roundtrip and boundary tests
- `internal/component/ike/wire/payload.go` -- generic payload header, payload type enum, payload chain, critical bit, Payload interface
- `internal/component/ike/wire/payload_test.go` -- unknown payload skip/critical tests, truncation tests
- `internal/component/ike/wire/payload_sa.go` -- SA payload with Proposal and Transform sub-structures
- `internal/component/ike/wire/payload_sa_test.go` -- SA/proposal/transform roundtrip tests
- `internal/component/ike/wire/payload_ke.go` -- Key Exchange payload
- `internal/component/ike/wire/payload_ke_test.go` -- KE roundtrip tests
- `internal/component/ike/wire/payload_nonce.go` -- Nonce payload
- `internal/component/ike/wire/payload_nonce_test.go` -- Nonce roundtrip and boundary tests
- `internal/component/ike/wire/payload_id.go` -- IDi and IDr payloads
- `internal/component/ike/wire/payload_id_test.go` -- ID roundtrip tests
- `internal/component/ike/wire/payload_auth.go` -- AUTH payload (methods 1, 2, 9, 14)
- `internal/component/ike/wire/payload_auth_test.go` -- AUTH roundtrip tests including RFC 7427 method 14
- `internal/component/ike/wire/payload_cert.go` -- CERT and CERTREQ payloads
- `internal/component/ike/wire/payload_cert_test.go` -- CERT/CERTREQ roundtrip tests
- `internal/component/ike/wire/payload_notify.go` -- Notify payload with notify type constants
- `internal/component/ike/wire/payload_notify_test.go` -- Notify roundtrip tests
- `internal/component/ike/wire/payload_delete.go` -- Delete payload with SPI list
- `internal/component/ike/wire/payload_delete_test.go` -- Delete roundtrip tests
- `internal/component/ike/wire/payload_ts.go` -- TSi and TSr payloads with traffic selector sub-structures
- `internal/component/ike/wire/payload_ts_test.go` -- TS roundtrip tests with IPv4/IPv6
- `internal/component/ike/wire/payload_cp.go` -- Configuration payload with attribute sub-structures
- `internal/component/ike/wire/payload_cp_test.go` -- CP roundtrip tests
- `internal/component/ike/wire/payload_eap.go` -- EAP payload
- `internal/component/ike/wire/payload_eap_test.go` -- EAP roundtrip tests
- `internal/component/ike/wire/payload_vendor.go` -- Vendor ID payload
- `internal/component/ike/wire/payload_vendor_test.go` -- Vendor ID roundtrip tests
- `internal/component/ike/wire/payload_sk.go` -- SK (Encrypted) payload (stores raw ciphertext, no crypto)
- `internal/component/ike/wire/payload_sk_test.go` -- SK roundtrip tests
- `internal/component/ike/wire/message.go` -- full IKEv2 message: header + payload chain WriteTo/ReadFrom
- `internal/component/ike/wire/message_test.go` -- full message roundtrip tests

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

1. **Phase: Wiring (MANDATORY FIRST)** -- create package, define Payload interface, header type
   - Tests: `TestHeaderRoundtrip`
   - Files: `header.go`, `payload.go`
   - Verify: header encode/decode roundtrip passes; Payload interface defined

2. **Phase: Core Payloads** -- SA (with proposals/transforms), KE, Nonce, IDi/IDr
   - Tests: `TestPayloadSARoundtrip`, `TestPayloadKERoundtrip`, `TestPayloadNonceRoundtrip`, `TestPayloadIDRoundtrip`, `TestProposalTransformNested`
   - Files: `payload_sa.go`, `payload_ke.go`, `payload_nonce.go`, `payload_id.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Auth and Cert Payloads** -- AUTH (including RFC 7427 method 14), CERT, CERTREQ
   - Tests: `TestPayloadAuthRoundtrip`, `TestPayloadCertRoundtrip`, `TestPayloadCertReqRoundtrip`
   - Files: `payload_auth.go`, `payload_cert.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Control Payloads** -- Notify, Delete, Vendor ID, SK
   - Tests: `TestPayloadNotifyRoundtrip`, `TestPayloadDeleteRoundtrip`, `TestPayloadVendorIDRoundtrip`, `TestPayloadSKRoundtrip`
   - Files: `payload_notify.go`, `payload_delete.go`, `payload_vendor.go`, `payload_sk.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Extension Payloads** -- TSi/TSr, CP, EAP
   - Tests: `TestPayloadTSRoundtrip`, `TestPayloadCPRoundtrip`, `TestPayloadEAPRoundtrip`
   - Files: `payload_ts.go`, `payload_cp.go`, `payload_eap.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Message and Error Handling** -- full message chain, truncation, unknown payloads
   - Tests: `TestMessageChainRoundtrip`, `TestDecodeTruncatedHeader`, `TestDecodeTruncatedPayload`, `TestDecodeUnknownPayloadSkip`, `TestDecodeUnknownPayloadCritical`
   - Files: `message.go`, `payload.go` (dispatch logic)
   - Verify: full chain roundtrip; error cases handled without panic

7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-17 has implementation with file:line |
| Correctness | Payload lengths include 4-byte generic header; big-endian byte order; skip-and-backfill for lengths |
| Naming | Payload type constants match RFC 7296 names; Go types use PascalCase |
| Data flow | Codec is a pure library; no imports of engine, crypto, or transport packages |
| Rule: buffer-first | All encoding uses WriteTo(buf, off); no append(); no make([]byte) in encoding helpers |
| Rule: no-layering | SK payload stores raw ciphertext; does not import crypto package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Header type exists | `grep -rn 'type Header struct' internal/component/ike/wire/` |
| Payload interface defined | `grep -rn 'type Payload interface' internal/component/ike/wire/` |
| All 15 payload types implemented | `grep -rn 'func.*WriteTo' internal/component/ike/wire/payload_*.go \| wc -l` |
| Message type with chain support | `grep -rn 'type Message struct' internal/component/ike/wire/` |
| All unit tests pass | `go test ./internal/component/ike/wire/...` |
| No append() in encoding | `grep -rn 'append(' internal/component/ike/wire/*.go \| grep -v _test.go \| grep -v '// ok:'` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All length fields validated before slice access; no panics on malformed input |
| Buffer bounds | WriteTo checks buffer capacity before writing; ReadFrom checks data length before reading |
| Integer overflow | Payload length fields are uint16; no overflow on addition when computing total message size |
| DoS resilience | Nested proposal/transform parsing has depth limit; payload chain has max count to prevent infinite loops |
| No crypto in codec | SK payload must NOT attempt decryption; stores raw bytes only |

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
MUST document: payload type values, length field semantics, critical bit handling, nonce length constraints, SPI size constraints.

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
- [ ] AC-1..AC-17 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-ipsec-5-ikev2-wire.md`
- [ ] Summary included in commit
