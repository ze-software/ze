# Spec: home-cpe-5-ike-eddsa

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P2 |
| Release reason | The home CPE authenticates with an Ed25519 certificate. Ze's PKI store accepts Ed25519 keys and the IPsec guide implies they sign, but IKE refuses them (`errUnsupportedKey`). RFC 8420 is listed in no release profile, so by the release timing rule this could land after the first release; it landed before it, and the guide now states what Ze does. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

RFC gaps: none known in code. RFC 8420 has no `rfc/short/` summary yet: Thomas decided O-1 on 2026-11-08, and `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` (commit `0f367857c2`) lists it as optional for vpn-concentrator and writes, enrols and tags the summary. The metadata cell takes only listed requirement ids or summary stems, so it carries `-`.

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
   responder with a P-256 certificate (the CPE case), and the reverse direction;
   Ze initiator with an Ed448 certificate; Ze responder receiving an Ed448 leaf,
   which proves certificate parsing and trust-chain validation on the receive
   path, from a trusted CA and from an untrusted one.

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
- [ ] `docs/guide/ipsec.md` "Configuration" (the PKI example) and "IKEv2 responder role" (the paragraph that opens "The signature method follows the peer")
  → Decision: the guide is wrong in two places and is corrected with this spec: the example names `certificate-file`, `private-key-file` and `ca-certificate`, while `ze-pki-conf.yang` declares `pki ca <name> certificate`, `pki certificate <name> certificate`, `intermediate` and a `private` container; and "Any other key, Ed25519 for example, cannot authenticate to such a peer" implies Ed25519 authenticates to an RFC 7427 peer, which today it does not.
- [ ] `docs/contributing/patching-the-standard-library.md` - how Ze carries a patched GOROOT
  → Decision: not taken. `crypto/x509` already parses an Ed448 leaf without error (A-2), so the Ed448 key is read from the leaf's raw SubjectPublicKeyInfo and no `crypto/x509` patch is needed.

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
- [ ] `internal/component/ike/engine/auth.go` `computeX509Auth` - legacy method when `sa.RemoteHashAlgos` is nil; otherwise `selectSignatureAlgorithm`, hashes the signed octets, `signDigest`, then writes `len(algID) | algID | signature` as method 14.
- [ ] `auth.go` `selectSignatureAlgorithm` - RSA picks SHA2-512, 384 or 256 from the peer's list; ECDSA P-384 needs SHA2-384, other curves SHA2-256; any other key returns `errUnsupportedKey`.
- [ ] `auth.go` `signDigest` and `verifySignature` - RSA PKCS#1 v1.5 and ECDSA ASN.1 only; anything else `errUnsupportedKey`.
- [ ] `auth.go` `hashFromAlgID` - accepts a raw OID or a full AlgorithmIdentifier SEQUENCE; knows the RSA and ECDSA SHA-2 OIDs only, so an Ed25519 signature from a peer is refused with "unsupported algorithm identifier".
- [ ] `auth.go` `algorithmIdentifierEC` - encodes SEQUENCE { OID } with no parameters, which is the RFC 8410 shape.
- [ ] `internal/component/ike/engine/cert_payload.go` `getRemoteCert` - verifies the peer's chain with `x509.Certificate.Verify` against the configured CA; a failure fails `verifyRemoteAuth`, and the responder answers AUTHENTICATION_FAILED (`responder.go`, `sendAuthFailed`).
- [ ] `internal/component/ike/engine/initiator.go` `buildSignatureHashAlgosNotify` - announces SHA2-256, SHA2-384, SHA2-512; the responder (`responder.go` `buildSAInitResponse`) sends the same builder. Constants `hashAlgoSHA2256..512` in `auth.go`.
- [ ] `internal/component/ike/engine/auth_legacy.go` `computeLegacySignatureAuth`, `legacyKeyName` - names an Ed25519 key in the refusal when the peer sent no notify.
- [ ] Go 1.27.3 `crypto/x509` (the toolchain `go env GOROOT` names): `parseCertificate` (`parser.go`) calls `parsePublicKey` only when `PublicKeyAlgorithm` is known, so an Ed448 SubjectPublicKeyInfo parses without error and leaves `PublicKey` nil; `checkSignature` (`x509.go`) finds no `signatureAlgorithmDetails` entry for an Ed448 signature and returns `ErrUnsupportedAlgorithm`.
- [ ] `internal/component/pki/config.go` `parsePrivateKey` - PKCS#8, SEC1 EC, PKCS#1; `verifyKeyMatchesCert` accepts ECDSA, RSA and Ed25519 key-certificate pairs; Ed448 falls to `errPKIKeyUnsupported`.
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RFC 7427 method 14 signing | `internal/component/ike/engine/auth.go` `computeX509Auth`, `selectSignatureAlgorithm`, `signDigest`; `docs/architecture/ike/ipsec-7-ikev2-engine.md` "The method Ze signs with follows the peer's SIGNATURE_HASH_ALGORITHMS notify" | Picks an algorithm from the peer's list for RSA and ECDSA keys, hashes, signs; any other key is `errUnsupportedKey` | extends: an EdDSA branch picks Identity and signs the octets whole |
| RFC 7427 method 14 verification | `auth.go` `verifyX509Auth`, `hashFromAlgID`, `verifySignature` | Recognises the RSA and ECDSA SHA-2 OIDs, raw or as a SEQUENCE; any other key is `errUnsupportedKey` | extends: recognises id-Ed25519 and id-Ed448 and verifies with no prehash |
| AlgorithmIdentifier encoding | `auth.go` `algorithmIdentifierEC` | Encodes SEQUENCE { OID } with no parameters | extends: reused for the RFC 8410 identifiers, which have the same shape |
| SIGNATURE_HASH_ALGORITHMS notify | `internal/component/ike/engine/initiator.go` `buildSignatureHashAlgosNotify`, shared by the responder | Announces SHA2-256, SHA2-384, SHA2-512 | extends: adds Identity (5) |
| Legacy signature fallback | `internal/component/ike/engine/auth_legacy.go` `computeLegacySignatureAuth`, `legacyKeyName` | Signs with method 1, 9, 10 or 11 when the peer sent no notify; refuses an Ed25519 key with a named message | extends: `legacyKeyName` also names an Ed448 key; the refusal stays |
| PKI key and certificate load | `internal/component/pki/config.go` `parsePrivateKey`, `verifyKeyMatchesCert` | Parses PKCS#8, SEC1 EC and PKCS#1; matches ECDSA, RSA and Ed25519 pairs | extends: Ed448 parse and match |
| Third-party module patches | `internal/le/vendorpatch/patches/` | Ze's diffs against upstream modules, applied at build time | does not apply: this spec names no patch to a module; the Ed448 module lands as `go.mod`, `go.sum` and `vendor/` in its own commit (Files to Modify) |

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
| Registration over hardcoding, outbound | Yes, after the redesign | Today the method 14 algorithms are four separate switches in `auth.go` (`selectSignatureAlgorithm`, `signDigest`, `hashFromAlgID`, `verifySignature`) plus the hash list in `buildSignatureHashAlgosNotify` and the key names in `auth_legacy.go` `legacyKeyName`. Adding EdDSA to each would be the central-enumeration edit `ai/META-DECISIONS.md` forbids ("redone as a registration before it is built"). The redesign: one signature-scheme table in `ike/engine`, filled from `init()` in one file per scheme family (`auth_scheme_rsa.go`, `auth_scheme_ecdsa.go`, `auth_scheme_eddsa.go`). Each entry declares the key type it serves (a predicate over the parsed public key, or the SPKI OID for Ed448, which `crypto/x509` leaves unparsed), its AlgorithmIdentifier bytes, its hash rule (one of the peer's SHA-2 values, or Identity with no prehash), and its sign and verify functions. Selection, signing, algID recognition, verification, the notify's hash list and the legacy refusal's key name are all derived from the table. It follows the IKE pattern already in use for transforms (`ike/crypto/transform.go`, a lookup table keyed by identifier) and adds the `init()` registration the transforms do not need, because a scheme family lives in its own file. The RSA and ECDSA behaviour moves into it unchanged (Behavior to preserve) |
| Registration over hardcoding, inbound | Yes, after the redesign | `hashFromAlgID` and `verifySignature` read the same table, so a received algID is recognised by lookup, never by a switch |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan 6.1.0 announces Identity and verifies an Ed25519 AUTH | Announcing: source read (A-4). Verifying: the lab image builds the openssl and curve25519 plugins (`test/interop-ipsec/Dockerfile.strongswan`), and strongSwan's openssl plugin carries `openssl_ed_public_key.c` | Scenario configuration changes | Scenario `x509-ed25519-ze-initiator` | confirmed (Assumptions Resolved) |
| A-2 | `crypto/x509` parses a certificate whose subject key is Ed448 without error, leaving `PublicKey` unset | Go 1.27.3 `crypto/x509` `parseCertificate` skips `parsePublicKey` for `UnknownPublicKeyAlgorithm` (read 2026-11-02) | Ed448 leaf cannot even be stored; Ed448 needs a patched GOROOT | `TestPKIAcceptsEd448KeyAndCertificate` | confirmed (Assumptions Resolved) |
| A-3 | A P-256 CA can sign an Ed448 leaf and Go verifies that chain, because the chain check reads only the issuer's key; an Ed448 CA (or intermediate) gives `ErrUnsupportedAlgorithm` | Go 1.27.3 `crypto/x509` `checkSignature`: the signature algorithm is looked up in `signatureAlgorithmDetails`, which holds no Ed448 entry, so an Ed448 signature returns `ErrUnsupportedAlgorithm`; an ECDSA-with-SHA256 signature is checked with the issuer's P-256 key and never reads the leaf key (read 2026-11-02). The lab CA `test/interop-ipsec/pki/ca.pem` is P-256 | Ed448 leaves need chain handling changes | `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA`, `TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA`, scenario `x509-ed448-ze-responder` | confirmed (Assumptions Resolved) |
| A-4 | Announcing Identity always matches the peer's behaviour and is conformant, since Ze always supports EdDSA verification once this lands | strongSwan 6.1.0 `src/libcharon/sa/ikev2/tasks/ike_init.c` `send_supported_hash_algorithms` (read 2026-11-02): with no IKE signature scheme configured it announces every registered hasher `hasher_algorithm_for_ikev2` accepts, and the openssl and curve25519 plugins each register a pro forma `HASH_IDENTITY` hasher, so strongSwan announces Identity whenever those plugins load, whatever its own certificate key; RFC 8420 Section 3 "supported and configured" | Announce only when an EdDSA certificate is configured | Source read; the scenarios' pcap shows both notifies carrying 5 | confirmed (Assumptions Resolved) |
| A-5 | `cloudflare/circl` `sign/ed448` implements RFC 8032 Ed448 with empty context | circl documentation | Choose another implementation | Phase 1 RFC 8032 Section 7.4 test vectors | confirmed (Assumptions Resolved) |

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
| strongSwan IKE_AUTH with an Ed448 leaf under the lab P-256 CA | → | `verifyRemoteAuth` → `getRemoteCert` chain check → EdDSA verify from the raw SubjectPublicKeyInfo | `x509-ed448-ze-responder` |

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
| AC-13 | `docs/guide/ipsec.md` | PKI example uses the YANG leaves that exist; the signature paragraph states Ed25519 and Ed448 sign under RFC 7427 with a peer that announces Identity, and cannot sign for a peer without the notify. Where it stands: met by `docs/guide/ipsec.md` now that O-2 is decided: an Ed25519 or Ed448 key signs under RFC 7427 only, pure, for a peer that announces Identity, and an Ed448 certificate must come from a CA that does not sign with Ed448 |
| AC-14 | strongSwan initiator with an Ed448 leaf issued by the lab P-256 CA, Ze responder trusting that CA | Ze verifies the chain and the Ed448 AUTH (AlgorithmIdentifier 30 05 06 03 2B 65 71); IKE SA and Child SA established |
| AC-15 | strongSwan initiator with an Ed448 leaf issued by a CA Ze does not trust | Refused: Ze answers AUTHENTICATION_FAILED, no Child SA is installed, and the log names the peer and the chain failure |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Loads an Ed25519 CPE certificate and brings up the management tunnel | config → PKI → IKE AUTH → strongSwan | `x509-ed25519-ze-initiator` |
| 2 | Terminates a peer that holds an Ed25519 certificate | strongSwan AUTH → Ze verify | `x509-ed25519-ze-responder` |
| 3 | Terminates a peer that holds an Ed448 certificate from a trusted CA, and refuses one from an untrusted CA | strongSwan AUTH → chain check → Ze verify | `x509-ed448-ze-responder` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSignatureHashNotifyAnnouncesIdentity` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-1 | |
| `TestComputeX509AuthSignsWithEd25519UnderIdentity` | same | AC-2, RFC 8032 Section 7.1 vector on fixed octets | |
| `TestComputeX509AuthSignsWithEd448UnderIdentity` | `internal/component/ike/engine/auth_ed448_test.go` | AC-3 | passes |
| `TestSignMatchesRFC8032Ed448Vectors` | `internal/core/ed448/ed448_test.go` | AC-3, RFC 8032 Section 7.4 "Blank" and "1 octet" vectors byte for byte, through the PKCS #8 and SubjectPublicKeyInfo parsers (A-5) | passes |
| `TestVerifyRefusesAlteredInput`, `TestParseRefusesOtherAlgorithmsAndMalformedKeys` | `internal/core/ed448/ed448_test.go` | Ed448 verify refusals and length boundaries; id-Ed448 with parameters refused (RFC 8410 Section 3) | passes |
| `TestEdDSARefusedWithoutIdentity` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-4 | |
| `TestEd448RefusedWithoutIdentity` | `internal/component/ike/engine/auth_ed448_test.go` | AC-4 for Ed448 | passes |
| `TestVerifyX509AuthAcceptsEd25519` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-6 both polarities | |
| `TestVerifyX509AuthAcceptsEd448` | `internal/component/ike/engine/auth_ed448_test.go` | AC-6 both polarities for Ed448; 113 and 115 octets refused | passes |
| `TestVerifyRefusesEdDSAAlgIDWithMismatchedKey` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-7 | |
| `TestVerifyRefusesEd448AlgIDWithMismatchedKey` | `internal/component/ike/engine/auth_ed448_test.go` | AC-7 for Ed448: id-Ed448 from an Ed25519 certificate, id-Ed25519 and ecdsa-with-SHA256 from an Ed448 certificate. Its first run found the id-Ed25519 case accepted, because both families share the Identity hash; fixed by `requireKeyFamilyNamesAlgorithm` in `verifyX509Auth` | passes after the fix |
| `TestVerifyRefusesEdDSAAlgIDWithParameters` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-8 | |
| `TestVerifyRefusesEd448AlgIDWithParameters` | `internal/component/ike/engine/auth_ed448_test.go` | AC-8 for Ed448 | passes |
| `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA` | `internal/component/ike/engine/auth_ed448_test.go` | AC-14, A-2, A-3: the openssl-minted Ed448 leaf issued by the P-256 lab CA verifies through `getRemoteCert` and the Ed448 AUTH | passes |
| `TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA` | same | AC-15, A-3: an Ed448 leaf from a CA the store does not hold fails `verifyRemoteAuth` with `x509.UnknownAuthorityError` naming the peer | passes |
| `TestGetRemoteCertNamesEd448SignedChain` | same | Known Limitations: a leaf an Ed448 CA signed is refused naming the peer, the issuer and the fix | passes |
| `TestPKIAcceptsEd448KeyAndCertificate` | `internal/component/pki/config_ed448_test.go` | AC-9, A-2 | passes |
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
| `pki-ed448-certificate-accepted` | `test/parse/pki-ed448-certificate-accepted.ci`, run by `./le test bgp parse --pattern pki-ed448-certificate-accepted` | an Ed448 entry validates; a mismatched pair is refused with the fix in the message | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `x509-ed25519-ze-initiator` | `test/interop-ipsec/scenarios/x509-ed25519-ze-initiator/`, checker `checkEd25519ZeSigns` (`internal/le/interoplab/ipsec/checkers.go`) | strongSwan 6.1.0, P-256 server cert | AC-10, the CPE case | passes; red walk recorded (Goal Validation) |
| `x509-ed25519-ze-responder` | `test/interop-ipsec/scenarios/x509-ed25519-ze-responder/`, checker `checkEd25519ZeVerifies` | strongSwan with Ed25519 client cert | AC-11 | passes; red walk recorded (Goal Validation) |
| `x509-ed448-ze-initiator` | `test/interop-ipsec/scenarios/x509-ed448-ze-initiator/`, checker `checkEd448ZeSigns` (`internal/le/interoplab/ipsec/eddsa.go`) | strongSwan 6.1.0 (openssl plugin: `PRIVKEY_SIGN`/`PUBKEY_VERIFY SIGN_ED448`, `HASHER HASH_IDENTITY`) | AC-12 | passes; red walk recorded (Goal Validation) |
| `x509-ed448-ze-responder` | `test/interop-ipsec/scenarios/x509-ed448-ze-responder/`, checker `checkEd448ZeVerifies` | strongSwan 6.1.0 initiator, two connections: `ze` with an Ed448 leaf issued by the lab P-256 CA, `untrusted` with an Ed448 leaf from `untrusted-ca.example.net` (`server-untrusted.pem`, mounted by `internal/le/interoplab/ipsec/ipsec.go`) | AC-14 (the `ze` connection establishes and traffic crosses it), AC-15 (charon logs `received AUTHENTICATION_FAILED notify error`, Ze logs the refusal with `peer=swan` and the unknown authority, no new ESP state) | passes; red walk recorded (Goal Validation) |

Each scenario runs alone with `IPSEC_INTEROP_SCENARIO=<name> ./le test integration interop-ipsec` (`docs/functional-tests.md`, the IPsec interop row), for example `IPSEC_INTEROP_SCENARIO=x509-ed448-ze-responder ./le test integration interop-ipsec`.

## Files to Modify
- `internal/component/ike/engine/auth.go` - the signature-scheme table (Architectural Verification); `selectSignatureAlgorithm`, `signDigest`, `hashFromAlgID` and `verifySignature` become lookups in it; Identity constant; the selection result says whether to prehash
- `internal/component/ike/engine/initiator.go` - the notify's hash list is derived from the table, so it announces Identity (responder uses the same builder)
- `internal/component/ike/engine/auth_legacy.go` - `legacyKeyName` reads the key name from the table
- `internal/component/ike/engine/cert_payload.go` `getRemoteCert` - an Ed448 leaf's key is read from its raw SubjectPublicKeyInfo; an Ed448-signed chain is refused with the message in Known Limitations
- `internal/component/pki/config.go` - Ed448 PKCS#8 parse and key-certificate match
- `go.mod`, `go.sum`, `vendor/` - Ed448 module, in its own commit
- `docs/guide/ipsec.md` - AC-13
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - signature algorithm table
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by `initiator.go`. "The method Ze signs with follows the peer's SIGNATURE_HASH_ALGORITHMS notify" says method 14 uses an algorithm from the peer's list and that a key other than RSA or ECDSA is refused; after this change Ze announces Identity and an Ed25519 or Ed448 key signs under method 14 with Identity, unhashed, so that paragraph names the EdDSA case and its `source:` anchors gain the EdDSA functions

## Files to Create
- `internal/component/ike/engine/auth_signature_scheme.go` - the table and its lookups
- `internal/component/ike/engine/register_signature_rsa.go`, `register_signature_ecdsa.go`, `register_signature_eddsa.go` - one scheme family each, registered into the table from `init()`
- `internal/component/ike/engine/auth_eddsa_test.go` also holds `TestEd25519SignsPureRFC8032Vector` (RFC 8032 Section 7.1 TEST 1 through `signDigest`, and Ed25519ph refused)
- `internal/component/ike/engine/auth_eddsa_test.go`
- `test/interop-ipsec/scenarios/x509-ed25519-ze-initiator/`, `x509-ed25519-ze-responder/`, `x509-ed448-ze-initiator/`, `x509-ed448-ze-responder/`
- `test/interop-ipsec/pki/` Ed25519 and Ed448 leaf certificates and keys under the existing lab CA, and for `x509-ed448-ze-responder` a second CA and an Ed448 leaf it issued (invented names, `example.net`)
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
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7427.md` notes Identity (closure); the RFC 8420 summary is `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md`'s work (O-1) |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists IKE signature algorithms |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-5-ike-eddsa.md` |
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
   - Tests: AC-3, AC-9, AC-12, AC-14, AC-15; `x509-ed448-ze-initiator`, `x509-ed448-ze-responder`
   - Files: `auth_scheme_eddsa.go`, `cert_payload.go`, `pki/config.go`
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
| Four scenarios | `ls test/interop-ipsec/scenarios/x509-ed*` |

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
| A-2 broken on a later toolchain | Report it; the raw SubjectPublicKeyInfo reader then needs the certificate parsed without `crypto/x509` |
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
| Always announce Identity | Announce only when an EdDSA certificate is configured | Ze always supports and is configured to verify EdDSA once this lands, and the notify is sent before the peer's certificate is known; RFC 8420 Section 3 then requires it. strongSwan does the same (A-4) |
| One signature-scheme table in `ike/engine`, filled from `init()` in `register_signature_{rsa,ecdsa,eddsa}.go`, from which selection, signing, algID recognition, verification, the notify's hash list and the legacy key name are derived | Add an EdDSA case to each of the four switches, the notify list and `legacyKeyName` | The switches are a central enumeration a new scheme would have to edit (`ai/META-DECISIONS.md`, "redone as a registration before it is built"); the table is the transform-registry shape IKE already uses. Detail in Architectural Verification. **Taken while the owner was away, pending review** (2026-11-07): the RFC and the code settle it, so it was built under the standing away rule. The files are named `register_signature_*.go`, not `auth_scheme_*.go`, because `writeGoPatterns` refuses an `init()` that registers outside a `register*.go` file; the table and its lookups live in `auth_signature_scheme.go`. `selectSignatureAlgorithm`, `signDigest`, `hashFromAlgID` and `verifySignature` keep their signatures, because RFC-tagged tests call them, and became table lookups. Identity is `crypto.Hash(0)` on the sign side (the value `crypto.SignerOpts` gives an unhashed message) and an `identityHash` `hash.Hash` on the verify side, so `verifyX509Auth`'s hash-then-verify pipeline is unchanged. The certificate key's family refuses a hash it does not sign with, which is the AC-7 check |
| Keep RSA and ECDSA hashes in the notify | Send Identity alone | RFC 8420 Section 3's SHOULD NOT applies only when every supported algorithm needs no hash; Ze verifies RSA and ECDSA too |
| Ed448 through `cloudflare/circl` `sign/ed448` | Implement Ed448 in tree; drop Ed448 | RFC 8420 covers both curves and full coverage of an implemented RFC is owed; an audited library over in-tree curve arithmetic. Lands in its own commit |
| Ed448 leaf key read from the raw SubjectPublicKeyInfo | Patch `crypto/x509` | Smaller and local; A-2 holds on Go 1.27.3 |

## Open decisions for the owner

| ID | Decision | Options |
|----|----------|---------|
| O-1 | RFC 8420 in a release profile | Decided 2026-11-08: RFC 8420 gets its own spec in the vpn-concentrator work, `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` (commit `0f367857c2`), which lists it as optional for vpn-concentrator and writes and enrols the summary. Nothing is left open here |
| O-2 | `github.com/cloudflare/circl` (`sign/ed448`) as a new third-party module, for Ed448 | Decided: approved for `internal/core/ed448` alone. The module landed alone in `bfc6dfe868` (`go.mod`, `go.sum`, `vendor/`), and the approval is the `dependency-ledger.txt` row `github.com/cloudflare/circl/sign/ed448	internal/core/ed448` added by `a5f0fbcd06`. Ed448 is built as its own family, `register_signature_ed448.go`, over `internal/core/ed448` (`091e347f4b`) |

## Known Limitations
- A chain in which a CA or intermediate signs with Ed448 is refused: `crypto/x509` `checkSignature` answers `ErrUnsupportedAlgorithm` (A-3). Ze refuses it with a message that names the issuer, says its Ed448 signature cannot be checked, and gives the fix (issue the Ed448 leaf from an RSA, ECDSA or Ed25519 CA). RFC 8420 covers the IKE AUTH signature, not certificate chains.
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
- [ ] The targeted tests for what changed pass: the scoped package tests for `internal/component/ike/engine` and `internal/component/pki` through `./le job run label unit-pkg command go test -tags ...`, `./le test bgp parse --pattern pki-ed448-certificate-accepted`, `IPSEC_INTEROP_SCENARIO=<name> ./le test integration interop-ipsec` once for each of `x509-ed25519-ze-initiator`, `x509-ed25519-ze-responder`, `x509-ed448-ze-initiator` and `x509-ed448-ze-responder`, the existing `x509-legacy-*` scenarios the same way (R-1), and `./le verify changed` over the closing commits
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

## Implementation Summary

### What Was Implemented
- `987f0e052b`: the Digital Signature algorithms became one table, `signatureSchemes` in `internal/component/ike/engine/auth_signature_scheme.go`, filled from `init()` by `register_signature_rsa.go`, `register_signature_ecdsa.go` and `register_signature_eddsa.go`. `selectSignatureAlgorithm`, `signDigest`, `hashFromAlgID`, `verifySignature`, `announcedHashAlgos` (read by `buildSignatureHashAlgosNotify`) and `legacyKeyName` are lookups in it. Ed25519 signs pure under Identity (5) for a peer that announced it, and verifies.
- `bfc6dfe868`: `github.com/cloudflare/circl` v1.6.5 vendored alone.
- `091e347f4b`: `internal/core/ed448` wraps circl `sign/ed448` (pure Ed448, empty context, PKCS #8 and SubjectPublicKeyInfo readers); `register_signature_ed448.go` adds the Ed448 family with `ed448CertificateKey`; `certificatePublicKey` reads a key crypto/x509 leaves nil; `requireKeyFamilyNamesAlgorithm` refuses an AlgorithmIdentifier the certificate key's family does not own; `getRemoteCert` names an Ed448-signed chain; `pki/config.go` `parsePrivateKey` and `verifyKeyMatchesCert` accept Ed448; four strongSwan scenarios and `test/parse/pki-ed448-certificate-accepted.ci`.

### Bugs Found/Fixed
- An Ed448 signature labelled id-Ed25519 from an Ed448 certificate verified, because both families share the Identity hash. Fixed by `requireKeyFamilyNamesAlgorithm` (`auth_signature_scheme.go`), covered by `TestVerifyRefusesEd448AlgIDWithMismatchedKey`.

### Documentation Updates
- `docs/guide/ipsec.md` (signature paragraph, PKI example) and `docs/architecture/ike/ipsec-7-ikev2-engine.md` (signature-family paragraph, anchors to `register_signature_eddsa.go`, `register_signature_ed448.go`, `internal/core/ed448/ed448.go`): in `987f0e052b` and `091e347f4b`.
- Closure: `rfc/short/rfc7427.md` notes Identity (5) beside its hash table, with anchors to `hashAlgoIdentity`, `announcedHashAlgos`, `chooseEd25519Signature`, `chooseEd448Signature`; `features/ikev2-engine.md` Interop names the four `ipsec/x509-ed*` scenarios.
- `./le doc check all`: the anchor stage reports "checked 3043 anchors across 23 digests, all resolve"; the run exits 1 on the `clear debug` pipe-operator availability check, which no file of this spec touches.

### Deviations from Plan
- The scheme files are `register_signature_*.go`, not `auth_scheme_*.go` (`writeGoPatterns` refuses an `init()` registration outside a `register*.go` file), and Ed448 is its own family file, `register_signature_ed448.go`, not part of `register_signature_eddsa.go`.
- Ed448 arithmetic lives in a new core package, `internal/core/ed448`, so circl has one importer (the ledger row approves that directory alone).
- The checkers live in `internal/le/interoplab/ipsec/eddsa.go`, registered in `checkers.go`; the lab keys live in each scenario's own `pki/` directory, not in `test/interop-ipsec/pki/`.
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` was not edited: it holds no signature algorithm table (grep for signature, ECDSA and RSA finds none); the signature families are documented in `ipsec-7-ikev2-engine.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Algorithm confusion was planned as a hash check (R-2: the family refuses a hash it does not sign with) | Two families share the Identity hash, so the hash cannot tell Ed25519 from Ed448 | `TestVerifyRefusesEd448AlgIDWithMismatchedKey` red on its first run | `requireKeyFamilyNamesAlgorithm` matches the AlgorithmIdentifier against the key's family |
| approach | The implementation commits left `features/ikev2-engine.md` Interop and `rfc/short/rfc7427.md` silent on the new scenarios and Identity | Documentation rows 1 and 9 said both were owed | Closure step 4 | Repaired in commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Announce Identity (5) | Done | `auth_signature_scheme.go` `announcedHashAlgos`; `register_signature_eddsa.go` and `register_signature_ed448.go` `announce` | both roles use `buildSignatureHashAlgosNotify` |
| 2. Sign pure EdDSA only for a peer that announced Identity | Done | `chooseEd25519Signature`, `signEd25519`, `chooseEd448Signature`, `signEd448`; `auth.go` `computeX509Auth` skips the hash for `hashIdentity` | |
| 3. Verify Ed25519 and Ed448 AUTH | Done | `verifyEd25519`, `verifyEd448`, `certificatePublicKey`, `requireKeyFamilyNamesAlgorithm` | |
| 4. Ed448 keys and leaves in the PKI store | Done | `pki/config.go` `parsePrivateKey`, `verifyKeyMatchesCert`; `pki/certkey.go` | |
| 5. strongSwan proof, four directions, trusted and untrusted CA | Done | `internal/le/interoplab/ipsec/eddsa.go` | Goal Validation |
| Guide correction | Done | `docs/guide/ipsec.md` | AC-13 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSignatureHashNotifyAnnouncesIdentity` | |
| AC-2 | Done | `TestComputeX509AuthSignsWithEd25519UnderIdentity`, `TestEd25519SignsPureRFC8032Vector` | |
| AC-3 | Done | `TestComputeX509AuthSignsWithEd448UnderIdentity`, `TestSignMatchesRFC8032Ed448Vectors` | |
| AC-4 | Done | `TestEdDSARefusedWithoutIdentity`, `TestEd448RefusedWithoutIdentity` | message names Identity, the peer's list and the fix |
| AC-5 | Done | `computeLegacySignatureAuth` unchanged apart from `legacyKeyName` | |
| AC-6 | Done | `TestVerifyX509AuthAcceptsEd25519`, `TestVerifyX509AuthAcceptsEd448` | |
| AC-7 | Done | `TestVerifyRefusesEdDSAAlgIDWithMismatchedKey`, `TestVerifyRefusesEd448AlgIDWithMismatchedKey` | |
| AC-8 | Done | `TestVerifyRefusesEdDSAAlgIDWithParameters`, `TestVerifyRefusesEd448AlgIDWithParameters` | |
| AC-9 | Done | `TestPKIAcceptsEd448KeyAndCertificate`, `pki-ed448-certificate-accepted.ci` | |
| AC-10 | Done | `x509-ed25519-ze-initiator` | |
| AC-11 | Done | `x509-ed25519-ze-responder` | |
| AC-12 | Done | `x509-ed448-ze-initiator` | |
| AC-13 | Done | `docs/guide/ipsec.md` lines 447-456 | |
| AC-14 | Done | `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA`, `x509-ed448-ze-responder` | |
| AC-15 | Done | `TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA`, `x509-ed448-ze-responder` (`untrusted` connection) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every unit test of the TDD table | Done | `auth_eddsa_test.go`, `auth_ed448_test.go`, `internal/core/ed448/ed448_test.go`, `pki/config_ed448_test.go` | all twenty names found with `git grep "func <name>("` |
| `pki-ed448-certificate-accepted` | Done | `test/parse/` | |
| four interop scenarios | Done | `test/interop-ipsec/scenarios/x509-ed*` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `auth.go`, `initiator.go`, `auth_legacy.go`, `cert_payload.go`, `pki/config.go` | Done | |
| `auth_signature_scheme.go`, `register_signature_{rsa,ecdsa,eddsa}.go` | Done | plus `register_signature_ed448.go` |
| `go.mod`, `go.sum`, `vendor/` | Done | `bfc6dfe868` |
| `docs/guide/ipsec.md`, `ipsec-7-ikev2-engine.md` | Done | |
| `ipsec-6-ikev2-crypto.md` | Changed | Deviations |
| `rfc/full/rfc8420.txt`, scenarios, `.ci` | Done | |

### Audit Summary
- **Total items:** 6 requirements, 15 ACs, 3 test groups, 6 file groups
- **Done:** all but one file row
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`ipsec-6-ikev2-crypto.md`, Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze signs with Ed25519 for strongSwan (the CPE case) | interop, red walk | `x509-ed25519-ze-initiator` passed at 12:19 on 2026-11-08 (`tmp/session/2026-11-07-104251/scratch/rwinv/out-c2-12-interop/rerun-ed25519-init.log`); with `sign: signEd448` in the Ed25519 family (overlay `rwinv/c5-ed25519-signs-ed448`) Ze logged `build IKE_AUTH failed ... unsupported key type` and the run ended `FAIL 0 passed, 1 failed` (`c5-ed25519-init-broken.log`, 12:08) |
| Ze verifies strongSwan's Ed25519 | interop, red walk | `rerun-ed25519-resp.log` pass; overlay `c5-ed25519-verifies-ed448`: Ze logged `initiator AUTH verification failed`, charon `received AUTHENTICATION_FAILED notify error`, FAIL (`c5-ed25519-resp-broken.log`, 12:10) |
| Ze signs with Ed448 | interop, red walk | `rerun-ed448-init.log` pass; overlay `c5-ed448-signs-ed25519`: `unsupported key type`, FAIL (`c5-ed448-init-broken.log`, 12:12) |
| Ze verifies an Ed448 leaf from a trusted CA and refuses one from an untrusted CA | interop, red walk | `rerun-ed448-resp.log` pass; overlay `c5-ed448-verifies-ed25519`: `initiator AUTH verification failed`, FAIL (`c5-ed448-resp-broken.log`, 12:13) |
| PKI accepts Ed448 | functional, red walk | `rwinv/c2-12-host.log`: `pki-ed448-certificate-accepted` CLEAN pass, BROKEN (overlay `c5-pki-ed448-unparsed`, the Ed448 PKCS #8 branch disabled) FAIL |

The overlays replace only `register_signature_eddsa.go`, `register_signature_ed448.go` or `pki/config.go`, none changed since `091e347f4b` (2026-11-07 23:57).

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC 8420 summary, enrolment, tags and discrimination records | O-1: Thomas gave RFC 8420 its own spec | `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-5-ike-eddsa-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (17 files) |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | logic and wiring of the scheme table, algorithm confusion, input bounds (AlgorithmIdentifier, key and signature lengths), stale comments, docs against source |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER or ISSUE. NOTE: `buildSignatureHashAlgosNotify`'s comment names Identity for Ed25519 only, though Ed448 needs it too (incomplete, not false). NOTE: `computeX509Auth` calls `hashFunc.New()`, which panics for an unlinked hash; every registered hash is SHA-2, linked by the families | `initiator.go`, `auth.go` | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `auth_signature_scheme.go`, `register_signature_{rsa,ecdsa,eddsa,ed448}.go`, `auth_eddsa_test.go`, `auth_ed448_test.go` | Yes | `ls internal/component/ike/engine` 2026-11-08 |
| `internal/core/ed448/ed448.go`, `ed448_test.go` | Yes | `ls` |
| `rfc/full/rfc8420.txt`, `test/parse/pki-ed448-certificate-accepted.ci` | Yes | `ls` |
| four `test/interop-ipsec/scenarios/x509-ed*` | Yes | `ls test/interop-ipsec/scenarios` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-9, AC-14, AC-15 (units) | the package tests pass | `ike/engine`: `all5.log` 2026-11-08 11:51, the only failures `TestMobikeParallelAuth*` (`bind: can't assign requested address` for 127.0.0.2, macOS loopback, not this spec); `pki` and `core/ed448`: `ok` in `u15.log` 23:45, no file changed since |
| AC-10 to AC-12, AC-14, AC-15 (interop) | the four scenarios pass | `rerun-ed*.log`, `PASS 1 scenario(s)` each |
| AC-13 | guide states the behaviour | `docs/guide/ipsec.md` 447-456 read at closure |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Ed448 `pki certificate` entry | `test/parse/pki-ed448-certificate-accepted.ci` | pass, and red with the Ed448 branch disabled |
| strongSwan exchange, both roles, both curves | the four interop scenarios | `eddsa.go` read: the checkers read charon's own `authentication of ... with ED25519/ED448 successful` lines |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `x509-ed25519-ze-initiator` passes: charon logs the Ed25519 authentication of Ze as successful, and Ze signed only because charon announced 5 |
| A-2 | confirmed | `TestPKIAcceptsEd448KeyAndCertificate` and `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA` parse the Ed448 leaf through `x509.ParseCertificate` |
| A-3 | confirmed | `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA`, `TestGetRemoteCertNamesEd448SignedChain`, `x509-ed448-ze-responder` |
| A-4 | confirmed | both initiator scenarios pass, so strongSwan announced Identity; both responder scenarios pass, so Ze's announcement reached charon's signer |
| A-5 | confirmed | `TestSignMatchesRFC8032Ed448Vectors` matches RFC 8032 Section 7.4 byte for byte |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1 feature: `features/ikev2-engine.md` Interop lists the four scenarios | `checkers.go` registry rows for `x509-ed*` | Yes |
| 6, 17 guide: `docs/guide/ipsec.md` 447-456 | `chooseEd25519Signature`, `chooseEd448Signature`, `getRemoteCert` Ed448 refusal | Yes |
| 7 wire: `ipsec-7-ikev2-engine.md` 401-431 | AlgorithmIdentifiers in `register_signature_eddsa.go` and `register_signature_ed448.go` | Yes |
| 9 RFC: `rfc/short/rfc7427.md` Identity note | `hashAlgoIdentity`, RFC 8420 Section 2 "This document defines a new value called "Identity" (5)" | Yes |
| 11 comparison: No | `grep -i ecdsa\|ed25519\|rfc 7427 docs/comparison.md` finds no IKE signature list | Yes |
| 16 anchors | `./le doc check all`: all anchors resolve | Yes |
