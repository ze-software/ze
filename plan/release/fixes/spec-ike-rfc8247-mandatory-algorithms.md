# Spec: ike-rfc8247-mandatory-algorithms

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | RFC 8247 is reached by a release profile, and four of its MUST-level algorithm requirements are absent from the IKE engine. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 8247 grades the IKEv2 algorithms. Ze meets every MUST and MUST NOT of
Sections 2 and 3 except four, each recorded as a `{gap}` row in
`rfc/short/rfc8247.md`:

| Row | RFC text | What Ze does |
|-----|----------|--------------|
| RFC8247-2.2-2 | PRF_HMAC_SHA1 is MUST- (Section 2.2), and MUST- means MUST today (Section 1.1) | `prfRegistry` (`internal/component/ike/crypto/transform.go`) holds SHA2-256, 384 and 512 only |
| RFC8247-2.3-2 | AUTH_HMAC_SHA1_96 is MUST- for the IKE SA (Section 2.3) | it sits in `espOnlyIntegrityRegistry`, so an IKE SA cannot offer or accept it |
| RFC8247-3.2-2 | RSASSA-PSS MUST be supported with the Digital Signature method and RSA (Section 3.2) | `hashFromAlgID` and `verifySignature` (`internal/component/ike/engine/auth.go`) know PKCS1-v1.5 only |
| RFC8247-3.1-1 | RSA Digital Signature (method 1) stays MUST (Section 3.1) | ze never sends method 1; `computeX509Auth` refuses to sign when the peer sent no SIGNATURE_HASH_ALGORITHMS; `verifyLegacyRSAAuth` checks a SHA-256 DigestInfo only, while RFC 7296 Section 3.8 has method 1 default to SHA-1 |

Each fix owes the interop proof against strongSwan
(`docs/architecture/testing/interop.md`), and each row loses its `{gap}`
for tagged tests with discrimination records once the behavior lands.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `mandatory-algorithm-absent.md` | 2026-10-26 | IKE PRF_HMAC_SHA1 (RFC8247-2.2-2) |
| `mandatory-algorithm-absent.md` | 2026-10-26 | IKE AUTH_HMAC_SHA1_96 (RFC8247-2.3-2) |
| `mandatory-algorithm-absent.md` | 2026-10-26 | RSASSA-PSS (RFC8247-3.2-2) |
| `mandatory-algorithm-absent.md` | 2026-10-26 | RSA AUTH method 1 (RFC8247-3.1-1) |

## Required Reading

| Read | Why |
|------|-----|
| `rfc/full/rfc8247.txt` Sections 1.1, 2.2, 2.3, 3.1 and 3.2 | The algorithm grades and what MUST- means |
| `rfc/full/rfc7296.txt` Sections 3.3.2 and 3.8 | The transform IDs, and method 1 defaulting to SHA-1 |
| `rfc/short/rfc8247.md` | The four `{gap}` rows this spec closes |
| `docs/architecture/ike/ipsec-6-ikev2-crypto.md` | The IKE transform registries and the AUTH computation |
| `docs/architecture/testing/interop.md` | The strongSwan scenario each fix owes |

Skeleton: the design pass reads how strongSwan negotiates each of the four
algorithms before the spec moves to `ready`.

## Current Behavior

Read at skeleton time to fill the Task table:

- [ ] `internal/component/ike/crypto/transform.go` -- `prfRegistry` holds the SHA-2 PRFs only; AUTH_HMAC_SHA1_96 is in `espOnlyIntegrityRegistry`
- [ ] `internal/component/ike/engine/auth.go` -- `hashFromAlgID`, `verifySignature`, `computeX509Auth` and `verifyLegacyRSAAuth` know PKCS1-v1.5 with SHA-2 only

## Data Flow

### Entry Point

The SA payload of IKE_SA_INIT (transform negotiation) and the AUTH payload of
IKE_AUTH (signature method and hash), from the peer and from the operator's
proposal config.

### Transformation Path

A configured proposal goes through the `crypto` transform registries into the
SA payload Ze sends; a received SA payload goes to `crypto` negotiation; an AUTH
payload goes to the signature verification and computation in
`engine/auth.go`.

### Boundaries Crossed

Wire to engine, and engine to the `crypto` registries. No plugin boundary.

### Integration Points

The transform registries in `internal/component/ike/crypto`, the AUTH method
handling in `internal/component/ike/engine/auth.go`, and the YANG enumerations
naming the algorithms an operator may configure.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| IKE_SA_INIT offering PRF_HMAC_SHA1 -> PRF negotiation | `prfRegistry` | named at design: an engine test and the strongSwan scenario |
| IKE_SA_INIT offering AUTH_HMAC_SHA1_96 for the IKE SA -> integrity negotiation | the IKE integrity registry | named at design |
| IKE_AUTH method 14 with RSASSA-PSS -> `verifySignature` | `engine/auth.go` | named at design |
| IKE_AUTH method 1 -> `verifyLegacyRSAAuth` and the send path | `engine/auth.go` | named at design |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| one tagged test per `{gap}` row, named at design | the row's requirement in both polarities, recorded with `./le rfc record` |

### Functional Tests

| Test | Asserts |
|------|---------|
| a strongSwan interop scenario, named at design | each algorithm negotiates and authenticates against strongSwan |

## Files to Modify

Skeleton, confirmed at design:

- `internal/component/ike/crypto/transform.go`
- `internal/component/ike/engine/auth.go`
- `rfc/short/rfc8247.md`

## Implementation Steps

Skeleton: written at design. The order follows the four Task rows, each
landing with its tagged tests and records.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Scoped unit tests of `internal/component/ike/crypto` and `internal/component/ike/engine`, then `./le verify changed`
- [ ] `./le rfc check` names no rfc8247 row
