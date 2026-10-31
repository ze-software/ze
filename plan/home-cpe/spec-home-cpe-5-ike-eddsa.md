# Spec: home-cpe-5-ike-eddsa

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P2 |
| Release reason | The home CPE authenticates with an Ed25519 certificate. Ze's PKI store accepts Ed25519 keys and the IPsec guide implies they sign, but IKE refuses them (`errUnsupportedKey`). RFC 8420 is listed in no release profile, so by the release timing rule this may land after the first release; the false guide sentence is corrected now. |
| RFC gaps | RFC 8420 Sections 2 and 3 (all MUST and MUST NOT), not enrolled: no profile lists it |
| Depends | - |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement RFC 8420 in Ze's IKEv2 RFC 7427 Digital Signature authentication, for
Ed25519 and Ed448, signing and verifying, in both roles:

1. Announce the "Identity" hash (5) in SIGNATURE_HASH_ALGORITHMS.
2. Sign the AUTH octets with an Ed25519 or Ed448 private key, with no prehash,
   only when the peer announced Identity, carrying the RFC 8410
   AlgorithmIdentifier with absent parameters.
3. Verify an Ed25519 or Ed448 Digital Signature AUTH from a peer whose
   certificate holds such a key.
4. Accept Ed448 private keys and Ed448 leaf certificates in the PKI store
   (Ed25519 is already accepted).
5. Prove against strongSwan: Ze initiator with an Ed25519 certificate, strongSwan
   responder with a P-256 certificate (the CPE case), and the reverse direction.

Correct `docs/guide/ipsec.md` in the same work: its PKI example uses leaf names
that do not exist, and its signature paragraph implies Ed25519 already works
with RFC 7427.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door; `pki` is always on
  → Constraint: `ike/crypto` is the transform registry and stays ungated; signature selection lives in `ike/engine/auth.go`, not in the transform registry.
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - crypto layer
- [ ] `docs/architecture/pki/pki-store.md` - configured store versus local CA
  → Constraint: the store parses `pki certificate <name>` entries; key type support is decided in `pki/config.go` `parsePrivateKey` and `verifyKeyMatchesCert`.
- [ ] `docs/guide/ipsec.md` lines 57-65 (PKI example) and 414-421 (signature method)
  → Decision: the guide is wrong in two places and is corrected with this spec: the example names `certificate-file`, `private-key-file` and `ca-certificate`, while `ze-pki-conf.yang` declares `pki ca <name> certificate`, `pki certificate <name> certificate`, `intermediate` and a `private` container; and "Any other key, Ed25519 for example, cannot authenticate to such a peer" implies Ed25519 authenticates to an RFC 7427 peer, which today it does not.
- [ ] `docs/contributing/patching-the-standard-library.md` - how Ze carries a patched GOROOT, in case Ed448 X.509 parsing is taken that way (see Key Design Decisions)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7427.md` - present, enrolled, profiles vpn-concentrator and vpn-endpoint
  → Constraint: RFC 7427 Section 4: "When calculating the digital signature, a peer MUST pick one algorithm sent by the other peer." (`selectSignatureAlgorithm` already enforces this; Identity joins the same rule.)
  → Constraint: RFC 7427 Section 4: "This notification also implicitly indicates support of the new "Digital Signature" algorithm method, as well as the list of hash functions supported by the sending peer."
- [ ] `rfc/short/rfc8420.md` - ABSENT. RFC 8420 appears in no profile (`rfc/short/*.md` "Use cases" rows, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` searched 2026-10-31). Per `ai/META-DECISIONS.md` ("no summary is written or enrolled for an RFC that no profile lists") no summary is written by this spec. A summary is owed once the owner lists RFC 8420 in a profile (see Open decisions). The text itself may be added: `rfc/full/rfc8420.txt` is fetched in Phase 1 (adding an RFC text under `rfc/full/` is permitted).
  → Constraint: RFC 8420 Section 2: "Ed25519 and Ed448 are only defined with the "Identity" hash and MUST NOT be sent to a receiver that has not indicated support for the "Identity" hash."
  → Constraint: RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
  → Constraint: RFC 8420 Section 3: "It MUST NOT be used if none of the supported and configured algorithms have this property." and "implementations MUST have the "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA is supported and configured." and "Implementations SHOULD NOT have other hash algorithms in the notification if all supported and configured signature algorithms have this property."
  → Constraint: RFC 8420 Appendix A: id-Ed25519 1.3.101.112 and id-Ed448 1.3.101.113, "Parameters are absent. Length is 7 bytes."
- [ ] `rfc/short/rfc8247.md` - present; its Section 3.1 legacy methods have no EdDSA method
  → Decision: with no SIGNATURE_HASH_ALGORITHMS from the peer, an EdDSA key still cannot sign, and the existing refusal (`computeLegacySignatureAuth`) stays.

**Key insights:**
- The hash-then-sign pipeline in `computeX509Auth` and `verifyX509Auth` assumes a prehash; Identity means the signed octets go to the signer whole.
- Go's standard library has Ed25519 and no Ed448, in `crypto` and in `crypto/x509`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/auth.go` `computeX509Auth` (about line 360) - legacy method when `sa.RemoteHashAlgos` is nil; otherwise `selectSignatureAlgorithm`, hashes the signed octets, `signDigest`, then writes `len(algID) | algID | signature` as method 14.
- [ ] `auth.go` `selectSignatureAlgorithm` (line 878) - RSA picks SHA2-512, 384 or 256 from the peer's list; ECDSA P-384 needs SHA2-384, other curves SHA2-256; any other key returns `errUnsupportedKey`.
- [ ] `auth.go` `signDigest` (line 926) and `verifySignature` (line 963) - RSA PKCS#1 v1.5 and ECDSA ASN.1 only; anything else `errUnsupportedKey`.
- [ ] `auth.go` `hashFromAlgID` - accepts a raw OID or a full AlgorithmIdentifier SEQUENCE; knows the RSA and ECDSA SHA-2 OIDs only, so an Ed25519 signature from a peer is refused with "unsupported algorithm identifier".
- [ ] `auth.go` `algorithmIdentifierEC` (line 846) - encodes SEQUENCE { OID } with no parameters, which is the RFC 8410 shape.
- [ ] `internal/component/ike/engine/initiator.go` `buildSignatureHashAlgosNotify` (line 611) - announces SHA2-256, SHA2-384, SHA2-512; the responder (`responder.go` about line 355) sends the same builder. Constants `hashAlgoSHA2256..512` in `auth.go` (about line 860).
- [ ] `internal/component/ike/engine/auth_legacy.go` `computeLegacySignatureAuth`, `legacyKeyName` - names an Ed25519 key in the refusal when the peer sent no notify.
- [ ] `internal/component/pki/config.go` `parsePrivateKey` (about line 400) - PKCS#8, SEC1 EC, PKCS#1; `verifyKeyMatchesCert` (line 427) accepts ECDSA, RSA and Ed25519 key-certificate pairs; Ed448 falls to `errPKIKeyUnsupported`.
- [ ] `go.mod` - `golang.org/x/crypto` v0.58.0; no `cloudflare/circl`. No Ed448 implementation is vendored.
- [ ] `test/interop-ipsec/Dockerfile.strongswan` - strongSwan 6.1.0 built with `--enable-openssl` and `--enable-curve25519`, both of which provide Ed25519 (and OpenSSL Ed448).

**Behavior to preserve:**
- RSA and ECDSA method 14 selection and the legacy methods 1, 9, 10, 11.
- An Ed25519 key facing a peer that sent no notify is refused with the existing message.
- The algID encoding for RSA (with NULL) and ECDSA (without).

**Behavior to change:**
- Notify announces Identity (5) alongside the SHA-2 values.
- Ed25519 and Ed448 sign and verify under method 14 with Identity.
- The PKI store accepts Ed448 keys and certificates.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `pki certificate cpe { certificate <PEM>; private { key <PEM> } }` with an Ed25519 or Ed448 key; the IKE peer's `authentication` names it.
- Wire: the peer's SIGNATURE_HASH_ALGORITHMS notify in IKE_SA_INIT; the peer's AUTH payload in IKE_AUTH.

### Transformation Path
1. PKI parse: key and certificate parsed and matched (`pki/config.go`).
2. IKE_SA_INIT: Ze sends the notify with Identity; records the peer's list in `sa.RemoteHashAlgos`.
3. IKE_AUTH send: `selectSignatureAlgorithm` returns the EdDSA AlgorithmIdentifier and "no prehash"; the signer receives the whole signed octets.
4. IKE_AUTH receive: the AlgorithmIdentifier maps to EdDSA with no prehash; the peer certificate's public key verifies the whole signed octets.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PKI store ↔ IKE engine | `pki.GetCertificate` returns key and chain | No |
| Ze ↔ strongSwan | AUTH method 14 payload, notify 16431 | No |
| Ze ↔ third-party module | Ed448 implementation (new dependency) | No |

### Integration Points
- `selectSignatureAlgorithm`, `signDigest`, `verifySignature`, `hashFromAlgID`, `buildSignatureHashAlgosNotify`, `parsePrivateKey`, `verifyKeyMatchesCert`, `getRemoteCert`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Same AUTH path for every key type |
| No unintended coupling | Yes | PKI stays the only key parser |
| No duplicated functionality | Yes | Extends the existing selection and verify switches |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | No | The signature algorithms are a closed switch in `auth.go`; this spec extends it. A registry is not added: the algorithm set is fixed by RFCs, not by plugins |
| Registration over hardcoding, inbound | No | Same switch; recorded so a reviewer judges it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan 6.1.0 announces Identity and verifies an Ed25519 AUTH | strongSwan builds openssl and curve25519 plugins; source not present locally | Scenario configuration changes | Scenario `x509-ed25519-ze-initiator` | unvalidated |
| A-2 | `crypto/x509` parses a certificate whose subject key is Ed448 without error, leaving `PublicKey` unset | Go has no Ed448 type | Ed448 leaf cannot even be stored; Ed448 needs a patched GOROOT or a raw SPKI parser | Unit test parsing an Ed448 leaf | unvalidated |
| A-3 | A P-256 or Ed25519 CA can sign an Ed448 leaf and Go verifies that chain (the CA's signature, not the leaf's key, is checked) | X.509 chain verification reads the issuer key | Ed448 leaves need chain handling changes | Unit test with lab PKI | unvalidated |
| A-4 | Announcing Identity always is conformant, since Ze always supports EdDSA verification once this lands | RFC 8420 Section 3 "supported and configured" | Announce only when an EdDSA certificate is configured | Reading of strongSwan's notify builder (open question O-2) | unvalidated |
| A-5 | `cloudflare/circl` `sign/ed448` implements RFC 8032 Ed448 with empty context | circl documentation | Choose another implementation | Phase 1 RFC 8032 Section 7.4 test vectors | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding Identity to the notify changes what an RSA/ECDSA peer sees | Existing x509 interop scenarios go red | Identity is an extra value; peers ignore unknown values (RFC 7427 Section 4 list semantics). Run the existing x509 scenarios |
| R-2 | An EdDSA signature is accepted from a peer whose certificate key is not EdDSA (algorithm confusion) | Unit test with mismatched key and algID | Verify only when algID and certificate key type agree; refuse otherwise |
| R-3 | Ed25519ph or Ed448ph slips in | Signature with a prehash or context | Use only the pure variants; test vectors |
| R-4 | New module brings a large vendor tree | `vendor/` diff | Module lands alone in its own commit (META); vendor only the Ed448 package's import closure |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | X.509 IKE authentication for every key type (shared switch and notify) |
| How is it reverted? | Single commit revert of the engine change; the module commit can stay |
| Who else touches this path? | `rfc7427` and `rfc4754` tagged tests; the x509-legacy-* interop scenarios |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pki certificate` with an Ed25519 key, IKE peer names it, peer announced Identity | → | `computeX509Auth` → `selectSignatureAlgorithm` → EdDSA signer | `TestComputeX509AuthSignsWithEd25519UnderIdentity` |
| Peer AUTH with id-Ed25519 AlgorithmIdentifier | → | `verifyX509Auth` → EdDSA verify | `TestVerifyX509AuthAcceptsEd25519` |
| Config with an Ed448 key | → | `pki/config.go` parse and match | `TestPKIAcceptsEd448KeyAndCertificate` |
| Full exchange with strongSwan | → | all of the above | `x509-ed25519-ze-initiator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any IKE_SA_INIT Ze sends (initiator or responder) | SIGNATURE_HASH_ALGORITHMS carries 2, 3, 4 and 5 |
| AC-2 | Ed25519 key, peer list contains 5 | AUTH method 14, AlgorithmIdentifier bytes 30 05 06 03 2B 65 70, a 64-octet signature over the whole signed octets that `ed25519.Verify` accepts |
| AC-3 | Ed448 key, peer list contains 5 | AlgorithmIdentifier 30 05 06 03 2B 65 71, 114-octet signature over the whole signed octets, verified with an empty context |
| AC-4 | EdDSA key, peer list lacks 5 | No AUTH sent; error names the missing Identity hash and the peer; message says how to fix it (enable RFC 8420 on the peer, or use an ECDSA certificate) |
| AC-5 | EdDSA key, peer sent no notify | Existing legacy refusal, unchanged |
| AC-6 | Peer AUTH with id-Ed25519 or id-Ed448 and a matching certificate key | Verified; an altered octet refuses with AUTHENTICATION_FAILED |
| AC-7 | Peer AUTH with id-Ed25519 but an ECDSA certificate key (or the reverse) | Refused |
| AC-8 | Peer AUTH with id-Ed25519 AlgorithmIdentifier carrying parameters | Refused (RFC 8420 Appendix A: parameters are absent) |
| AC-9 | PKI entry with an Ed448 key and its certificate | Accepted; a mismatched pair refused with the existing mismatch error |
| AC-10 | Ze initiator, Ed25519 leaf under a P-256 CA, strongSwan responder with P-256 cert | IKE SA and Child SA established; strongSwan logs an Ed25519 signature verified |
| AC-11 | strongSwan initiator with an Ed25519 cert, Ze responder | Established; Ze verified EdDSA |
| AC-12 | Ed448 variant of AC-10 | Established |
| AC-13 | `docs/guide/ipsec.md` | PKI example uses the YANG leaves that exist; the signature paragraph states Ed25519 and Ed448 sign under RFC 7427 with a peer that announces Identity, and cannot sign for a peer without the notify |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Loads an Ed25519 CPE certificate and brings up the management tunnel | config → PKI → IKE AUTH → strongSwan | `x509-ed25519-ze-initiator` |
| 2 | Terminates a peer that holds an Ed25519 certificate | strongSwan AUTH → Ze verify | `x509-ed25519-ze-responder` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSignatureHashNotifyAnnouncesIdentity` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-1 | |
| `TestComputeX509AuthSignsWithEd25519UnderIdentity` | same | AC-2, RFC 8032 Section 7.1 vector on fixed octets | |
| `TestComputeX509AuthSignsWithEd448UnderIdentity` | same | AC-3, RFC 8032 Section 7.4 vector | |
| `TestEdDSARefusedWithoutIdentity` | same | AC-4 | |
| `TestVerifyX509AuthAcceptsEd25519`, `...Ed448` | same | AC-6 both polarities | |
| `TestVerifyRefusesEdDSAAlgIDWithMismatchedKey` | same | AC-7 | |
| `TestVerifyRefusesEdDSAAlgIDWithParameters` | same | AC-8 | |
| `TestPKIAcceptsEd448KeyAndCertificate` | `internal/component/pki/config_test.go` | AC-9, A-2 | |
| existing `TestRFC7427SignatureAlgorithmIsOneThePeerSent` | `internal/component/ike/engine/` | unchanged, still green | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AlgorithmIdentifier length octet | 0-255 | 7 for EdDSA | 6 (truncated OID) refused | 9 (with parameters) refused |
| Ed25519 signature length | 64 | 64 | 63 refused | 65 refused |
| Ed448 signature length | 114 | 114 | 113 refused | 115 refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-ed448-certificate-accepted` | `test/parse/` | an Ed448 entry validates; a mismatched pair is refused with the fix in the message | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `x509-ed25519-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0, P-256 server cert | AC-10, the CPE case | |
| `x509-ed25519-ze-responder` | same | strongSwan with Ed25519 client cert | AC-11 | |
| `x509-ed448-ze-initiator` | same | strongSwan 6.1.0 (OpenSSL Ed448) | AC-12 | |

## Files to Modify
- `internal/component/ike/engine/auth.go` - Identity constant; EdDSA branch in selection, sign, verify; algID recognition; the selection result says whether to prehash
- `internal/component/ike/engine/initiator.go` - notify announces Identity (responder uses the same builder)
- `internal/component/ike/engine/auth_legacy.go` - `legacyKeyName` names an Ed448 key too
- `internal/component/pki/config.go` - Ed448 PKCS#8 parse and key-certificate match
- `go.mod`, `go.sum`, `vendor/` - Ed448 module, in its own commit
- `docs/guide/ipsec.md` - AC-13
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - signature algorithm table

## Files to Create
- `internal/component/ike/engine/auth_eddsa_test.go`
- `test/interop-ipsec/scenarios/x509-ed25519-ze-initiator/`, `x509-ed25519-ze-responder/`, `x509-ed448-ze-initiator/`
- `test/interop-ipsec/pki/` Ed25519 and Ed448 leaf certificates and keys under the existing lab CA (invented names, `example.net`)
- `rfc/full/rfc8420.txt` - the RFC text
- `test/parse/pki-ed448-certificate-accepted.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | existing PKI leaves carry the PEM |
| YANG validation constraints | No | |
| YANG custom validators | No | key type checked in `pki/config.go` |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | no runtime dependency |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope changed? | Yes | the IPsec feature file under `features/` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md` |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7427.md` notes Identity; RFC 8420 summary waits for the owner's profile decision |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists IKE signature algorithms |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-5-ike-eddsa.md` |
| 17 | Existing examples for this area? | Yes | the PKI example in `docs/guide/ipsec.md` (AC-13) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)**
   - Tests: `TestComputeX509AuthSignsWithEd25519UnderIdentity`, `TestVerifyX509AuthAcceptsEd25519`
   - Files: test file; fetch `rfc/full/rfc8420.txt`; correct the false guide sentence at once (a false public claim is corrected without waiting, `ai/rules/rfc-compliance.md`)
   - Verify: tests red on `errUnsupportedKey` and "unsupported algorithm identifier"
2. **Phase: Ed25519**
   - Tests: AC-1, AC-2, AC-4 to AC-8 unit tests; `x509-ed25519-ze-initiator`, `x509-ed25519-ze-responder`
   - Files: `auth.go`, `initiator.go`, `auth_legacy.go`
   - Verify: interop green; revert the sign branch and confirm the scenario red; existing x509-legacy-* and RFC 7427 tests green
3. **Phase: Ed448 module** - one commit holding only `go.mod`, `go.sum` and the vendor tree (META)
4. **Phase: Ed448**
   - Tests: AC-3, AC-9, AC-12
   - Files: `auth.go`, `pki/config.go`
   - Verify: RFC 8032 Section 7.4 vectors; interop green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No prehash on the EdDSA path; empty context; key type and algID agree |
| Rule: RFC quotes | Each new function quotes RFC 8420 Section 2 or 3 above the statement |
| Rule: user-facing errors | AC-4 message says what, why, and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Notify carries 5 | unit test and pcap in the scenario |
| Three scenarios | `ls test/interop-ipsec/scenarios/x509-ed*` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Algorithm confusion | Verify path refuses an algID that does not match the certificate key type |
| Malleability | Pure EdDSA verify from the standard library (Ed25519) and the chosen module (Ed448), no custom arithmetic |
| Input validation | AlgorithmIdentifier length and parameters checked before use |

### Failure Routing
| Failure | Route To |
|---------|----------|
| strongSwan refuses Ze's Ed25519 AUTH | Capture both logs; check algID bytes and Identity presence |
| A-2 broken | Choose between a GOROOT patch and a raw SPKI reader; report the choice |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

The Digital Signature method was written as "hash, then sign". RFC 8420 adds an
algorithm whose hash is the identity, so the selection result has to say whether
a hash runs at all, rather than which one.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | No row exists (grep of `plan/journal/` for Ed25519, EdDSA and 8420 on 2026-10-31 found none) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Always announce Identity | Announce only when an EdDSA certificate is configured | Ze always supports and is configured to verify EdDSA once this lands, and the notify is sent before the peer's certificate is known; RFC 8420 Section 3 then requires it. Open question O-2 checks strongSwan |
| Keep RSA and ECDSA hashes in the notify | Send Identity alone | RFC 8420 Section 3's SHOULD NOT applies only when every supported algorithm needs no hash; Ze verifies RSA and ECDSA too |
| Ed448 through `cloudflare/circl` `sign/ed448` | Implement Ed448 in tree; drop Ed448 | RFC 8420 covers both curves and full coverage of an implemented RFC is owed; an audited library over in-tree curve arithmetic. Lands in its own commit |
| Ed448 leaf key read from the raw SubjectPublicKeyInfo | Patch `crypto/x509` | Smaller and local, if A-2 holds |

## Open decisions for the owner

| ID | Decision | Options |
|----|----------|---------|
| O-1 | RFC 8420 in a release profile | List it as optional for vpn-endpoint and vpn-concentrator (then a summary is written and enrolled), or leave it unlisted (code ships, no ledger entry) |
| O-2 | Not an owner question: strongSwan's notify behaviour is read from the 6.1.0 tarball the lab builds (`libcharon/sa/ikev2/tasks/ike_init.c`, `send_supported_hash_algorithms`) before Phase 2 | - |

## Known Limitations
- Ed448 signatures by a CA over a certificate are not verified by `crypto/x509`; only Ed448 leaf keys are in scope. RFC 8420 does not cover certificate chains.
- No EdDSA legacy method exists; a peer without RFC 7427 cannot verify an EdDSA key.

## RFC Documentation (Scope: protocol)

Above the sign-side Identity check: `// RFC 8420 Section 2: "Ed25519 and Ed448 are
only defined with the "Identity" hash and MUST NOT be sent to a receiver that has
not indicated support for the "Identity" hash."` Above the notify value: the
Section 3 MUST quoted in Required Reading.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (RFC 8420: absent by META rule, recorded above)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
