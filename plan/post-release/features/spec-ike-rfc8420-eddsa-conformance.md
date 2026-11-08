# Spec: ike-rfc8420-eddsa-conformance

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | - (RFC 8420 is listed only in the vpn-concentrator profile, which is not a release profile; the code it measures already ships, so this spec adds proof and a ledger entry, not behaviour) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Related: the other vpn-concentrator specs, `plan/post-release/features/spec-ike-eap-only-authentication.md` (RFC 5998) and `plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md` (RFC 4739). No vpn-concentrator umbrella spec exists (searched `plan/` on 2026-11-08); see Open decisions O-2.

Implementation came from `plan/release/fixes/spec-home-cpe-5-ike-eddsa.md` (commits `987f0e052b` Ed25519, `bfc6dfe868` circl vendor, `091e347f4b` Ed448). This spec answers that spec's open decision O-1.

RFC gaps: no RFC 8420 requirement is known unmet in code (Current Behavior). The cell carries `-` because no `rfc/short/rfc8420.md` exists yet, and the cell takes only listed requirement ids or summary stems. The testing gaps this spec closes are in the TDD Test Plan.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas decided that RFC 8420 (EdDSA, Ed25519 and Ed448, in IKEv2) gets its own
spec, done as part of the vpn-concentrator work. Ze already implements it. This
spec brings it onto the RFC ledger:

1. List RFC 8420 in the vpn-concentrator profile, required or optional per the
   profile rules in `ai/META-DECISIONS.md`. The vpn-endpoint profile stays as
   Thomas left it: this spec decides nothing there and lists nothing there.
2. Write `rfc/short/rfc8420.md` with verbatim row quotes, walk the RFC text, and
   record the extraction sign-off `rfc/extraction/rfc8420.json`.
3. Map each MUST, MUST NOT, SHOULD and SHOULD NOT to the existing tests with
   `// RFC requirement:` tags, write the tests that are missing, and record each
   tagged unit's discrimination record with `./le rfc record`.
4. Record any requirement Ze does not meet as a `{gap}` row. The research below
   found none in code; it found four testing gaps, closed here.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` (IKE), `ai/doors/core.md` (`internal/core/ed448`), `ai/doors/le-tooling.md` (interop scenarios, `.ci`)
  → Constraint: the tests here stay inside `ike/engine`, `internal/core/ed448` and `pki`, with no new import across a component.
  → Constraint: `internal/core/ed448` is the one package that imports `github.com/cloudflare/circl`, and no circl type leaves it (`internal/core/ed448/ed448.go` package comment). An engine test that needs an Ed448ph signature MUST NOT import circl; the Ed448ph refusal is tested in `internal/core/ed448`.
  → Constraint (core door trap): a test that mutates the package-level `signatureSchemes` table snapshots it before the first mutation and restores it in `t.Cleanup`, or it poisons the next test.
- [ ] `ai/META-DECISIONS.md`, "Goal and scope"
  → Constraint: "An RFC for an optional transport or feature is listed for the profile as optional, and it counts toward the profile's percentage only once Thomas brings it into scope. Every RFC a profile lists, required or optional, has a valid summary."
  → Constraint: "RFC validation scope does not grow: no summary is written or enrolled for an RFC that no profile lists". Listing RFC 8420 in vpn-concentrator (step 1) is what licenses the summary, so step 1 and the summary land in one commit.
  → Constraint: "A requirement counts as covered only when a recorded break proved its test goes red."
- [ ] `docs/contributing/rfc-conformance-gates.md`, "The artifacts", "Profiles and aggregates", "The row quote", "The extraction sign-off", "Producing a record: the two proof routes"
  → Constraint: a use-case profile lists no member; the summary's `Use cases` and `Optional use cases` Meta rows are the declaration (`readUseCases`, `checkUseCases`). Step 1 is a Meta row in `rfc/short/rfc8420.md`, and `rfc/profiles/vpn-concentrator.md` is not edited.
  → Constraint: a row quote is a verbatim span of the cited section, at least 24 characters, matched case-sensitively after whitespace collapse (`checkRowQuotes`). The five quotes in AC-3 were copied from `rfc/full/rfc8420.txt` Sections 2 and 3.
  → Constraint: the extraction skeleton comes from `./le rfc extraction create stem rfc8420`, is classified in scratch, and is moved into `rfc/extraction/` only when every site and section carries a disposition. A sign-off is a precondition of enrolment (`checkEnrolment`).
  → Constraint: `./le rfc record` is the only writer of a discrimination record, it must SEE the tagged unit's own `--- FAIL:` line, and records land in the same commit as the tag. `./le rfc check` refuses a tagged unit added against `HEAD^` with no record.
  → Constraint: derived files (`rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`, `rfc/requirements/`, `ai/RFC-REQUIREMENTS.md`, `rfc/graph.tsv`) are untracked; `./le commit create` refuses one.
- [ ] `rfc/profiles/vpn-concentrator.md`, `rfc/profiles/vpn-endpoint.md`
  → Decision: vpn-concentrator is `Release: no`; vpn-endpoint is `Release: yes`. Listing RFC 8420 in vpn-concentrator therefore changes no release figure.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md`, "The method Ze signs with follows the peer's SIGNATURE_HASH_ALGORITHMS notify"
  → Constraint: the page already says the notify carries SHA2-256, SHA2-384, SHA2-512 and Identity; this spec changes no behaviour, so the page is not edited.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc8420.txt` - present (added by `987f0e052b`). Sections 1, 1.1, 2, 3, 4 (IANA), 5 (references), Appendix A, A.1, A.2.
  → Constraint: RFC 8420 Section 2: "Ed25519 and Ed448 are only defined with the "Identity" hash and MUST NOT be sent to a receiver that has not indicated support for the "Identity" hash."
  → Constraint: RFC 8420 Section 2: "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE."
  → Constraint: RFC 8420 Section 3: "It MUST NOT be used if none of the supported and configured algorithms have this property."
  → Constraint: RFC 8420 Section 3: "For this reason, implementations MUST have the "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA is supported and configured."
  → Constraint: RFC 8420 Section 3: "Implementations SHOULD NOT have other hash algorithms in the notification if all supported and configured signature algorithms have this property."
  → Decision: Appendix A carries no keyword; it repeats RFC 8410's objects for convenience ("[RFC8410] is the normative reference for the ASN.1 objects for Ed25519 and Ed448."). Its "Parameters are absent." is RFC 8410's obligation, so it is not a row here, and the existing parameter tests (`TestVerifyRefusesEdDSAAlgIDWithParameters`, `TestVerifyRefusesEd448AlgIDWithParameters`) stay untagged. RFC 8410 has no summary and no profile lists it.
- [ ] `rfc/short/rfc7427.md` - enrolled, `Use cases: vpn-concentrator, vpn-endpoint`, `Aggregate: ikev2-ipsec`, `Implementation: ze`
  → Decision: RFC 8420 joins the same aggregate (`ikev2-ipsec`) with its own `Support area`, and declares `Implementation: ze`, as RFC 7427 does. Every RFC 8420 obligation is IKE logic in Ze's Go; the EdDSA arithmetic (Go's `crypto/ed25519`, circl through `internal/core/ed448`) is RFC 8032's, which is not a row here.
  → Constraint: `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` already carries `RFC requirement: RFC7427-3-4 positive`. A second tag on it for RFC8420-3-2 leaves the RFC 7427 tag and its record untouched.
- [ ] `rfc/short/rfc7634.md`, `rfc/short/rfc8031.md`, `rfc/short/rfc7383.md` - IKE algorithm and extension RFCs already listed for vpn-concentrator
  → Decision: each is an optional algorithm or extension and is listed under `Optional use cases`, never `Use cases`. RFC 7634 is implemented and so is `Enrolment: enrolled` while optional; RFC 8031 and 7383 are unimplemented and `out-of-scope`. RFC 8420 is the RFC 7634 case: optional and enrolled (Key Design Decisions, O-1).

**Key insights:**
- Every RFC 8420 obligation is met by Ze's own Go in `ike/engine` today; what is missing is the ledger (summary, sign-off, tags, records) and four tests.
- Two families (Ed25519 and Ed448) each announce Identity, so a single break of one family's `announce` cannot turn the RFC8420-3-2 test red: its record needs a break in the shared `announcedHashAlgos` or a paired break.
- RFC 8420 Section 3's MUST NOT and SHOULD NOT are conditional on a table Ze never builds (no EdDSA family, or only EdDSA families). The derivation rule is still Ze's code, and it is tested by swapping the table.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/auth_signature_scheme.go` - `signatureScheme` table filled from `init()`; `announcedHashAlgos` derives the notify list from every family's `announce`, deduplicated and sorted; `selectSignatureAlgorithm`, `signDigest`, `hashFromAlgID`, `verifySignature` are lookups; `hashIdentity` (`crypto.Hash(0)`) and `hashAlgoIdentity` (5); `requireKeyFamilyNamesAlgorithm` refuses an AlgorithmIdentifier whose family does not own the certificate key. Header line 8 says "(no summary: no release profile lists RFC 8420)".
- [ ] `internal/component/ike/engine/register_signature_eddsa.go` - Ed25519 family: `announce` is Identity only; `chooseEd25519Signature` refuses without Identity in the peer's list (Section 2 MUST NOT quoted above it); `signEd25519` refuses any h other than Identity, then pure `ed25519.Sign`; `verifyEd25519` refuses any h other than Identity, checks key and signature length, then pure `ed25519.Verify`. Header line 4 carries the same note.
- [ ] `internal/component/ike/engine/register_signature_ed448.go` - Ed448 family, same shape over `internal/core/ed448`: `chooseEd448Signature`, `signEd448`, `verifyEd448`, and `ed448CertificateKey` reading the raw SubjectPublicKeyInfo. Header line 6 carries the note.
- [ ] `internal/component/ike/engine/auth.go` - `computeX509Auth` signs the octets whole under Identity; header line 8 carries the note.
- [ ] `internal/core/ed448/ed448.go` - `Sign` is pure Ed448 with an empty context (Section 2 MUST NOT quoted above it); no prehash entry point exists. Header line 5 carries the note.
- [ ] `internal/component/ike/engine/auth_eddsa_test.go` - `TestSignatureHashNotifyAnnouncesIdentity`, `TestEd25519SignsPureRFC8032Vector` (sign half of the no-prehash rule, Ed25519 only), `TestComputeX509AuthSignsWithEd25519UnderIdentity`, `TestEdDSARefusedWithoutIdentity`, `TestVerifyX509AuthAcceptsEd25519`, `TestVerifyRefusesEdDSAAlgIDWithMismatchedKey`, `TestVerifyRefusesEdDSAAlgIDWithParameters`. Header line 6: "(not enrolled: no release profile lists RFC 8420, so these tests carry no requirement tag)".
- [ ] `internal/component/ike/engine/auth_ed448_test.go` - `TestComputeX509AuthSignsWithEd448UnderIdentity`, `TestEd448RefusedWithoutIdentity`, `TestVerifyX509AuthAcceptsEd448`, `TestVerifyX509AuthAcceptsEd448LeafUnderP256CA`, `TestVerifyX509AuthRefusesEd448LeafFromUntrustedCA`, `TestVerifyRefusesEd448AlgIDWithMismatchedKey`, `TestVerifyRefusesEd448AlgIDWithParameters`, `TestGetRemoteCertNamesEd448SignedChain`. Same header note.
- [ ] `internal/component/ike/engine/rfc7427_notify_sent_test.go` - `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify`, tagged RFC7427-3-4, asserts the exact ordered list 2, 3, 4, 5 in both the IKE_SA_INIT request and response.
- [ ] `internal/core/ed448/ed448_test.go` - `TestSignMatchesRFC8032Ed448Vectors`, `TestVerifyRefusesAlteredInput`, `TestParseRefusesOtherAlgorithmsAndMalformedKeys`.
- [ ] `internal/le/interoplab/ipsec/checkers.go` `scenarioCheckers` - maps `x509-ed25519-ze-initiator`, `x509-ed25519-ze-responder`, `x509-ed448-ze-initiator`, `x509-ed448-ze-responder` to `checkEd25519ZeSigns`, `checkEd25519ZeVerifies`, `checkEd448ZeSigns`, `checkEd448ZeVerifies` (`eddsa.go`). The four pass against strongSwan 6.1.0 (owner's statement in the brief).
- [ ] `test/parse/pki-ed448-certificate-accepted.ci` - an Ed448 key and its certificate load through the config path; a mismatched pair is refused.

**Requirement against code, today:**

| Requirement | Producer | Met in code | Test today | Missing |
|-------------|----------|-------------|------------|---------|
| §2 MUST NOT send EdDSA without the peer's Identity | `chooseEd25519Signature`, `chooseEd448Signature` | Yes | `TestEdDSARefusedWithoutIdentity`, `TestEd448RefusedWithoutIdentity` | tag and record only |
| §2 MUST NOT use Ed25519ph or Ed448ph, sign half | `signEd25519`, `signEd448`, `ed448.Sign` | Yes | `TestEd25519SignsPureRFC8032Vector` (Ed25519 only), `TestSignMatchesRFC8032Ed448Vectors` (core) | an engine test that `signEd448` refuses a prehash h |
| §2 MUST NOT use Ed25519ph or Ed448ph, verify half | `verifyEd25519`, `verifyEd448`, `ed448.Verify` | Yes (pure verify only) | none presents a prehashed signature | a test that a ph signature is refused |
| §3 MUST NOT announce Identity with no arbitrary-size family | `announcedHashAlgos` | Yes (Identity comes only from a family's `announce`) | none | a test over a table without the EdDSA families |
| §3 MUST announce Identity when EdDSA is supported and configured | `announcedHashAlgos` over the two EdDSA `announce` lists | Yes | `TestSignatureHashNotifyAnnouncesIdentity`, `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` | tag and record only |
| §3 SHOULD NOT announce other hashes when every family is arbitrary-size | `announcedHashAlgos` | Yes (the list is the union of the families' needs) | none | a test over a table holding only the EdDSA families |

**Behavior to preserve:**
- Every behaviour above, the notify list 2, 3, 4, 5, and every existing test assertion. No producer code changes.

**Behavior to change:**
- None in the product. Comments that say "no summary" or "not enrolled" for RFC 8420 become false once the summary lands and are rewritten in the same commit (`ai/rules/stale-comments.md`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Summary Meta use-case rows | `internal/le/rfc/meta.go` `readUseCases`, `checkUseCases`; `rfc-conformance-gates.md` "Profiles and aggregates" | A summary declares the profiles that require it and those that list it as optional | extends: `rfc/short/rfc8420.md` declares `Optional use cases: vpn-concentrator`; no profile file is edited |
| Technology aggregate | `rfc/profiles/ikev2-ipsec.md`, `meta.go` `readSupport`, `checkAggregate` | Groups the IKE and IPsec summaries on the status page | extends: RFC 8420 joins `ikev2-ipsec` through its `Support` cell |
| Extraction sign-off | `./le rfc extraction create`, `./le rfc extraction classify`; `rfc/extraction/README.md` | Records the walk of the RFC text and bounds the gate | extends: one new artifact, `rfc/extraction/rfc8420.json` |
| Requirement tags and the row checklist | `// RFC requirement:` lines read by `internal/le/rfc`; `rfc/short/<stem>.md` Compliance Checklist | Ties each gated row to its tests | extends: five rows, tags on existing and new tests |
| Discrimination records | `./le rfc discriminate stem <stem> report <path>`, `./le rfc record`; `rfc/discrimination/<stem>/<id>.json` | Proves a tagged unit goes red under a break of its producer | extends: one record per tagged unit and polarity |
| Signature-family table | `ike/engine/auth_signature_scheme.go` `signatureSchemes`, `registerSignatureScheme`, `announcedHashAlgos` | Derives the notify list, selection and verification from registered families | does not apply as a change: the new Section 3 tests swap this table for the test's duration, the registry's own shape, rather than adding a seam |
| The interop scenarios | `internal/le/interoplab/ipsec/checkers.go` `scenarioCheckers`, `eddsa.go` | Prove Ze signs and verifies Ed25519 and Ed448 against strongSwan | does not apply as a change: they are this spec's goal-validation evidence; the unit tests carry the tags, because a unit record is cheap and an interop record runs a lab per break |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Ledger: `rfc/short/rfc8420.md` read by `./le rfc check` and `./le rfc update`.
- Product (unchanged): the peer's SIGNATURE_HASH_ALGORITHMS notify in IKE_SA_INIT and its AUTH payload in IKE_AUTH; Ze's own notify and AUTH; the `pki` config that loads an EdDSA key.

### Transformation Path
1. `./le rfc check` loads the profiles, then the summary's Meta (`ParseMeta`), the rows (`checkRowQuotes` against `rfc/full/rfc8420.txt`), the sign-off, the tags and the records.
2. `./le rfc update` regenerates the derived files, so `docs/features/rfc-status.md` gains an `ikev2-ipsec` row for RFC 8420 and the vpn-concentrator optional figure gains its rows.
3. Product path, unchanged: `announcedHashAlgos` → notify; `selectSignatureAlgorithm` → family `choose` → `sign`; `verifyX509Auth` → `requireKeyFamilyNamesAlgorithm` → family `verify`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary ↔ gate | Meta table and checklist rows parsed by `internal/le/rfc` | No |
| Gate ↔ tests | `// RFC requirement:` tags, records replayed by `./le rfc check` | No |
| Ze ↔ strongSwan | AUTH method 14, notify 16431 (unchanged) | Yes, by the four scenarios |

### Changed-Wire Census

**Search evidence:** no wire changes. The spec edits a summary, a sign-off, records, comments and tests. The product writers (`buildSignatureHashAlgosNotify`, `computeX509Auth`) and reader (`verifyX509Auth`) were read and stay unchanged.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| SIGNATURE_HASH_ALGORITHMS notify | writer | `ike/engine/initiator.go` `buildSignatureHashAlgosNotify` over `announcedHashAlgos` | read 2026-11-08 | preserve: no edit |
| AUTH method 14 | writer and reader | `ike/engine/auth.go` `computeX509Auth`, `verifyX509Auth` | read 2026-11-08 | preserve: no edit |

### Integration Points
- `announcedHashAlgos`, `signDigest`, `verifySignature`, and the family functions `chooseEd25519Signature`, `chooseEd448Signature`, `signEd25519`, `signEd448`, `verifyEd25519`, `verifyEd448`: the producers the tags and records name.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The tests call the functions the IKE path calls |
| No unintended coupling (components stay isolated) | Yes | The Ed448ph test lives in `internal/core/ed448`, so circl stays in that one package |
| No duplicated functionality (extends existing, does not recreate) | Yes | The table swap reuses `signatureSchemes`; no second notify builder |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No product code changes |
| Registration over hardcoding, outbound | Yes | No producer change; the notify list stays derived from registered families |
| Registration over hardcoding, inbound | Yes | The summary declares its own profile membership; `rfc/profiles/vpn-concentrator.md` lists no member and is not edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | RFC 8420 belongs under `Optional use cases`, not `Use cases`, for vpn-concentrator | META "Goal and scope": an RFC for an optional feature is listed as optional; RFC 7634, 8031 and 7383 are listed that way; RFC 8420 adds an optional signature algorithm, and RFC 8247 (the IKE algorithm requirements, listed as required) names no EdDSA requirement (grep of `rfc/full/rfc8247.txt` for Ed25519 and Ed448 found none) | The row moves to `Use cases`, and RFC 8420 counts in vpn-concentrator's required figure. Nothing else changes | Thomas's answer to O-1 | unvalidated |
| A-2 | Ze's EdDSA support counts as "supported and configured" in every build, so Section 3's MUST holds unconditionally | Both EdDSA families register from `ike/engine` with no config knob, and Ze verifies an EdDSA AUTH from any peer; the notify goes out before the peer's certificate is known. strongSwan 6.1.0 announces Identity the same way (`spec-home-cpe-5-ike-eddsa.md` A-4, source-read) | A config knob would be owed, and the MUST NOT row's test would gain a config case | Read `feature-gates.txt` for any tag that drops one EdDSA family alone | unvalidated |
| A-3 | A test that swaps `signatureSchemes` proves the derivation rule the shipped code runs | `announcedHashAlgos` reads only the table; the shipped build is one population among those the rule covers | The Section 3 MUST NOT and SHOULD NOT rows would need an exclusion instead (Key Design Decisions) | The record: a break in `announcedHashAlgos` turns the swapped-table test red | unvalidated |
| A-4 | `crypto/ed25519` produces an Ed25519ph signature for the verify-half test through `ed25519.PrivateKey.Sign` with `ed25519.Options` naming SHA-512, over a SHA-512 digest | Go standard library documentation of `ed25519.Options` | The test takes RFC 8032 Section 7.3's Ed25519ph vector instead | The test, cross-checked against RFC 8032 Section 7.3 | unvalidated |
| A-5 | circl's `sign/ed448` exposes an Ed448ph signer usable from `internal/core/ed448`'s own test | circl v1.6.5 vendored by `bfc6dfe868` | The test takes RFC 8032 Section 7.5's Ed448ph vector instead | Read `vendor/github.com/cloudflare/circl/sign/ed448/` before writing the test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A single break of one family's `announce` leaves Identity announced by the other, so the RFC8420-3-2 record sees no red | `./le rfc record` reports the unit stayed green | Break `announcedHashAlgos` (drop identifier 5), or take a paired record over both families' `announce`, with its solo runs |
| R-2 | The verify-half no-prehash break is hard to state: a pure verifier never accepts a ph signature, and the plausible regression is calling a ph verifier | No candidate in the `./le rfc discriminate` report | Mutant route on `verifyEd25519` (verify with SHA-512 options instead of pure); if no break exists, take the escape the gates page allows and say so in the row, never a claim the record does not support |
| R-3 | The table-swap tests poison sibling tests | Unrelated `ike/engine` tests fail only when run together | Snapshot and restore in `t.Cleanup`; no `t.Parallel` in those tests |
| R-4 | A row quote misses by whitespace or a quote mark | `checkRowQuotes` refuses "Not verbatim" | Copy from `rfc/full/rfc8420.txt`, which uses ASCII double quotes around Identity |
| R-5 | The extraction counts keyword sites in Section 1.1's boilerplate, or reads Section 1's lower-case "not recommended" | Skeleton lists sites outside Sections 2 and 3 | Classify 1.1 as boilerplate; a lower-case word is not a BCP 14 keyword, and the walk records that reason |
| R-6 | Enrolling makes `./le rfc check` demand an artifact this spec did not plan (an audit verdict, a status-page field) | `./le rfc check` names it | Produce it here: it is part of enrolling, not new scope |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: no product code changes. A bad summary or record reddens `./le rfc check` for the corpus |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-home-cpe-5-ike-eddsa.md` (the implementing spec, still open); RFC 7427's tag on `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_SA_INIT Ze sends, either role | → | `buildSignatureHashAlgosNotify` → `announcedHashAlgos` | `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` |
| Peer list without Identity, Ed25519 certificate | → | `computeX509Auth` → `chooseEd25519Signature` | `TestEdDSARefusedWithoutIdentity` |
| Peer AUTH under id-Ed25519 carrying an Ed25519ph signature | → | `verifyX509Auth` → `verifyEd25519` | `TestVerifyRefusesEd25519phSignature` |
| `pki` config with an Ed448 key | → | PKI load and match | `test/parse/pki-ed448-certificate-accepted.ci` |
| `rfc/short/rfc8420.md` | → | `./le rfc check` (`ParseMeta`, `checkRowQuotes`, `checkEnrolment`) | `./le rfc check` green with RFC 8420 enrolled |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `rfc/short/rfc8420.md` Meta | `Use cases` is `-` and `Optional use cases` is `vpn-concentrator` (the reverse if O-1 says required); `vpn-endpoint` appears in neither row; `Support` is `ikev2-ipsec` with a `Support area`; `Implementation` is `ze` with a reason naming `ike/engine`; `Enrolment` is `enrolled` with a reason; `Depends on` names RFC 7296, 7427, 8032 and 8410 | `rfc/profiles/vpn-concentrator.md` and `vpn-endpoint.md` stay byte-identical, because a profile lists no member |
| AC-2 | `rfc/extraction/rfc8420.json` | Every section has a disposition: 2 and 3 walked; front, 1, 1.1 (boilerplate), 4 (IANA), 5 (references), A, A.1, A.2 skipped or walked with a reason; every keyword site maps to one of the five rows; register `rfc2119`; `./le rfc check` accepts it | Appendix A's "Parameters are absent." is not a row: it is RFC 8410's obligation, repeated for convenience |
| AC-3 | Compliance Checklist | Exactly five rows, each quote verbatim. RFC8420-2-1 [MUST NOT] "Ed25519 and Ed448 are only defined with the "Identity" hash and MUST NOT be sent to a receiver that has not indicated support for the "Identity" hash." (§2). RFC8420-2-2 [MUST NOT] "The prehashed versions of Ed25519 and Ed448 (Ed25519ph and Ed448ph, respectively) MUST NOT be used in IKE." (§2). RFC8420-3-1 [MUST NOT] "It MUST NOT be used if none of the supported and configured algorithms have this property." (§3). RFC8420-3-2 [MUST] "For this reason, implementations MUST have the "Identity" value in the SIGNATURE_HASH_ALGORITHMS notification when EdDSA is supported and configured." (§3). RFC8420-3-3 [SHOULD NOT] "Implementations SHOULD NOT have other hash algorithms in the notification if all supported and configured signature algorithms have this property." (§3). `checkRowQuotes` accepts all five | No `{gap}` row, because the research found every requirement met in code; a row the walk shows unmet becomes `{gap}` naming the absent producer |
| AC-4 | RFC8420-2-1 | Tagged `negative` on `TestEdDSARefusedWithoutIdentity` and `TestEd448RefusedWithoutIdentity`, `positive` on `TestComputeX509AuthSignsWithEd25519UnderIdentity` and `TestComputeX509AuthSignsWithEd448UnderIdentity`; each has a record whose break is the Identity check in that family's `choose` | none |
| AC-5 | RFC8420-2-2 | Tagged on `TestEd25519SignsPureRFC8032Vector`; new `TestEd448SignRefusesPrehash` (engine: `signDigest` with an Ed448 key and a non-Identity h is refused, and under Identity the signature verifies with `ed448.Verify`); new `TestVerifyRefusesEd25519phSignature` (engine: an Ed25519ph signature over the signed octets under id-Ed25519 fails `verifyX509Auth`, while the pure signature over the same octets verifies); new `TestVerifyRefusesEd448phSignature` (`internal/core/ed448`: `Verify` refuses an Ed448ph signature over the message and accepts the pure one); each tagged and recorded | Where no break of the producer turns a verify-half test red (R-2), the row's claim says which half is proven, never more |
| AC-6 | RFC8420-3-1 | New `TestIdentityNotAnnouncedWithoutAnArbitrarySizeFamily`: with the table holding only the RSA and ECDSA families, `announcedHashAlgos` answers 2, 3, 4 and the notify carries no 5; the table is restored after; tagged and recorded | none |
| AC-7 | RFC8420-3-2 | Tagged `positive` on `TestSignatureHashNotifyAnnouncesIdentity` and `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` (its RFC 7427 tag and record unchanged); recorded with a break that removes identifier 5 from the list (R-1) | none |
| AC-8 | RFC8420-3-3 | New `TestOnlyIdentityAnnouncedWhenEveryFamilyIsArbitrarySize`: with the table holding only the Ed25519 and Ed448 families, `announcedHashAlgos` answers exactly 5; tagged and recorded | The shipped list carries 2, 3, 4 beside 5, which the SHOULD NOT permits because the RSA and ECDSA families are supported |
| AC-9 | Stale comments | The "(no summary: no release profile lists RFC 8420)" and "(not enrolled ...)" notes in `auth.go`, `auth_signature_scheme.go`, `register_signature_eddsa.go`, `register_signature_ed448.go`, `internal/core/ed448/ed448.go`, `auth_eddsa_test.go` and `auth_ed448_test.go` point at `rfc/short/rfc8420.md` instead, as 96 other `ike/engine` files point at their summaries | none |
| AC-10 | `./le rfc check`, `./le rfc update` | Green for rfc8420; the derived `docs/features/rfc-status.md` shows the row under `ikev2-ipsec`; `./le rfc tree profile vpn-concentrator` lists RFC 8420 in the optional tier with 4 of 4 mandatory and 1 of 1 optional rows proven | none |
| AC-11 | Interop goal validation | The four `x509-ed*` scenarios pass, and a reverted Ed25519 sign branch turns `x509-ed25519-ze-initiator` red, recorded in the Goal Validation table (or cited from `spec-home-cpe-5-ike-eddsa.md` if it recorded that red) | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads Ze's RFC 8420 support on the RFC status page and in the vpn-concentrator profile | summary → `./le rfc update` → status page | `./le rfc check`, `./le rfc tree profile vpn-concentrator` |
| 2 | Loads an Ed448 certificate and terminates a remote-access client holding an Ed25519 or Ed448 certificate | `pki` config → strongSwan AUTH → Ze verify | `test/parse/pki-ed448-certificate-accepted.ci`, `x509-ed25519-ze-responder`, `x509-ed448-ze-responder` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEd448SignRefusesPrehash` | `internal/component/ike/engine/auth_ed448_test.go` | AC-5, sign half for Ed448 | new |
| `TestVerifyRefusesEd25519phSignature` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-5, verify half for Ed25519 (A-4) | new |
| `TestVerifyRefusesEd448phSignature` | `internal/core/ed448/ed448_test.go` | AC-5, verify half for Ed448 (A-5) | new |
| `TestIdentityNotAnnouncedWithoutAnArbitrarySizeFamily` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-6 | new |
| `TestOnlyIdentityAnnouncedWhenEveryFamilyIsArbitrarySize` | `internal/component/ike/engine/auth_eddsa_test.go` | AC-8 | new |
| `TestEdDSARefusedWithoutIdentity`, `TestEd448RefusedWithoutIdentity`, `TestComputeX509AuthSignsWithEd25519UnderIdentity`, `TestComputeX509AuthSignsWithEd448UnderIdentity` | existing | AC-4 (tag and record) | exists |
| `TestEd25519SignsPureRFC8032Vector` | existing | AC-5 (tag and record) | exists |
| `TestSignatureHashNotifyAnnouncesIdentity`, `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` | existing | AC-7 (tag and record) | exists |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hash identifier 5 in Ze's notify | present or absent | present only when an arbitrary-size family is registered | N/A | N/A |

The signature and AlgorithmIdentifier length boundaries are tested by the implementing spec and are not RFC 8420 rows.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-ed448-certificate-accepted` | `test/parse/pki-ed448-certificate-accepted.ci`, run by `./le test bgp parse --pattern pki-ed448-certificate-accepted` | An Ed448 key and certificate load through the config path the IKE signer reads; a mismatched pair is refused | exists (no change); rerun as daemon-path evidence |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `x509-ed25519-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | Ze signs Ed25519 under Identity (RFC8420-2-1 and 3-2 on the wire) | passes (owner) |
| `x509-ed25519-ze-responder` | same | strongSwan 6.1.0 | Ze verifies Ed25519 | passes (owner) |
| `x509-ed448-ze-initiator` | same | strongSwan 6.1.0 | Ze signs Ed448 | passes (owner) |
| `x509-ed448-ze-responder` | same | strongSwan 6.1.0 | Ze verifies Ed448 and refuses an untrusted chain | passes (owner) |

## Files to Modify
- `internal/component/ike/engine/auth_eddsa_test.go` - tags; three new tests; header note (AC-9)
- `internal/component/ike/engine/auth_ed448_test.go` - tags; `TestEd448SignRefusesPrehash`; header note
- `internal/component/ike/engine/rfc7427_notify_sent_test.go` - second tag, for RFC8420-3-2
- `internal/core/ed448/ed448_test.go` - `TestVerifyRefusesEd448phSignature`, tag
- `internal/component/ike/engine/auth.go`, `auth_signature_scheme.go`, `register_signature_eddsa.go`, `register_signature_ed448.go`, `internal/core/ed448/ed448.go` - header notes only (AC-9)
- `features/ikev2-engine.md` - the feature's evidence names RFC 8420 and its summary
- `plan/release/fixes/spec-home-cpe-5-ike-eddsa.md` - its O-1 and Documentation row 9 point to this spec (appended note)

## Files to Create
- `rfc/short/rfc8420.md` - the summary (AC-1, AC-3)
- `rfc/extraction/rfc8420.json` - the sign-off (AC-2)
- `rfc/discrimination/rfc8420/<id>.json` - one per requirement, written by `./le rfc record` (AC-4 to AC-8)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config change |
| YANG validation constraints | No | no config change |
| YANG custom validators | No | no config change |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | No | none added |
| Editor autocomplete | No | none added |
| Functional test for new RPC/API | No | no RPC; the existing `.ci` covers the config path |
| Pipe completeness | No | no output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | IKE, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ikev2-engine.md` (evidence gains the RFC 8420 rows) |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | `docs/guide/ipsec.md` already states the behaviour (`spec-home-cpe-5-ike-eddsa.md` AC-13); nothing changes |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8420.md`; the `docs/features/rfc-status.md` row is derived by `./le rfc update` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | no claim changes |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md`; source edits are comments only, so `docs/architecture/ike/ipsec-7-ikev2-engine.md` (declared by the changed files' `// Design:` headers) is unaffected, and the derived list names it so |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring and the summary**
   - Through `/ze-rfc`: `./le rfc extraction create stem rfc8420`, classify in scratch, write `rfc/short/rfc8420.md` (AC-1, AC-3), move the sign-off in (AC-2).
   - Tag the existing tests (AC-4, AC-5 sign half for Ed25519, AC-7) and take their records with `./le rfc discriminate stem rfc8420 report <path>`, then `./le rfc record`.
   - Rewrite the header notes (AC-9).
   - Verify: `./le rfc check` green; the scoped `ike/engine` package test under `./le job run`. One commit carries summary, sign-off, tags, records and comments, because the gate refuses a tag without a record and a listed RFC without a summary.
2. **Phase: Missing tests**
   - Tests: AC-5 verify half and Ed448 sign half, AC-6, AC-8.
   - Each new test is first shown red under the break its record names, then green; records taken with `./le rfc record`.
   - Verify: the scoped `ike/engine` and `internal/core/ed448` package tests; `./le rfc check`.
3. **Phase: Evidence**
   - `./le rfc update`, `./le rfc tree profile vpn-concentrator` (AC-10); the `.ci` rerun; interop red proof or citation (AC-11); `features/ikev2-engine.md`; the note in `spec-home-cpe-5-ike-eddsa.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each of the five rows has a tagged unit and a record; every new test exists |
| Correctness | Each tag's claim states what the test body checks and no more (`ai/rules/rfc-compliance.md`) |
| Scope | `vpn-endpoint` appears nowhere in the summary; no profile file edited |
| Isolation | Table-swap tests restore `signatureSchemes`; no circl import outside `internal/core/ed448` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Summary and sign-off | `ls rfc/short/rfc8420.md rfc/extraction/rfc8420.json`; `./le rfc check` |
| Records | `ls rfc/discrimination/rfc8420/` shows the five ids |
| Tags | `grep -rn "RFC8420-" internal/component/ike/engine internal/core/ed448` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Algorithm confusion | The new verify-half tests do not relax `requireKeyFamilyNamesAlgorithm` or the families' h checks |
| Test isolation | A swapped table never leaks into another test, where it could mask a refusal |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `checkRowQuotes` refuses a row | Re-copy from `rfc/full/rfc8420.txt` (R-4) |
| `./le rfc record` stays green | R-1 or R-2; never widen a claim to fit, reword it to what is proven and re-record |
| A walk finds an unmet requirement | Add a `{gap}` row naming the absent producer; the code fix is owed work under `ai/rules/rfc-compliance.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

RFC 8420's Section 3 obligations are about how a notify list is derived, not
about one list. Ze's list is derived from the registered families, so the
derivation is what the MUST NOT and SHOULD NOT bind, and a table swap tests it
without a seam.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | No row exists (grep of `plan/journal/` for 8420, Ed25519 and EdDSA on 2026-10-31 by the implementing spec found none) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `Optional use cases: vpn-concentrator`, `Enrolment: enrolled` | `Use cases: vpn-concentrator`; or optional and `out-of-scope` | META lists an optional feature as optional; RFC 7634 sets the precedent that an implemented optional RFC is enrolled and gated, counted in the optional tier. Thomas confirms in O-1 |
| Tag the unit tests, cite the interop scenarios | Tag the interop checkers in `internal/le/interoplab/ipsec/eddsa.go` | A unit record is cheap and runs alone; an interop record runs a strongSwan lab per break. The scenarios stay as goal validation |
| Test Section 3's conditional rows by swapping `signatureSchemes` | Exclude them because Ze never builds such a table | `announcedHashAlgos` is Ze's producer and can regress; an exclusion proves nothing and rests on the condition never arising, which a future gated family would break |
| Ed448ph verify test in `internal/core/ed448` | In `ike/engine` | Only `internal/core/ed448` may import circl |

## Open decisions for the owner

| ID | Decision | Options and effect |
|----|----------|--------------------|
| O-1 | RFC 8420 under `Optional use cases` or `Use cases` for vpn-concentrator | Optional (recommended): matches META's optional-feature rule and RFC 7634, 8031 and 7383; its rows show in the optional figure. Required: they count in vpn-concentrator's required figure. vpn-concentrator is `Release: no`, so neither changes a release figure |
| O-2 | No vpn-concentrator umbrella spec exists | Leave this spec beside its siblings with a Related line (as now), or have an umbrella written that parents this spec and the RFC 5998 and RFC 4739 specs |

## Known Limitations
- RFC 8410's obligations (absent parameters, the OIDs) are tested by the implementing spec but stay off the ledger: RFC 8410 has no summary and no profile lists it.
- The vpn-endpoint profile does not list RFC 8420, by Thomas's instruction.

## RFC Documentation (Scope: protocol)

The producers already carry the Section 2 and Section 3 quotes. New tests carry
`// RFC requirement: RFC8420-<id> <polarity> -- <claim>` lines whose claim states
only what the test body checks.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (RFC 8420: written by this spec)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped `ike/engine` and `internal/core/ed448` package tests, the `.ci` above, and `./le rfc check`. No agent runs any `./le verify` command
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` only, in the same `./le commit create` script
