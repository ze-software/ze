# RFC 5280 - Internet X.509 Public Key Infrastructure Certificate and Certificate Revocation List (CRL) Profile

Partial. Every requirement this repository extracted from RFC 5280, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.8% | 8 of 102 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 17.6% | 18 of 102 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 102 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 102 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 34 of 34 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 102 | of 210 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 102 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 102 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 52.0% | 53 of 102 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 102 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 22.5% | 23 of 102 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 102 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 210 |
| Gated MUST-level | 102 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 23 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 0 |
| Discrimination records | 34 |
| Summary | `rfc/short/rfc5280.md` |
| Requirement shard | `rfc/requirements/rfc5280.md` |
| RFC text | `rfc/full/rfc5280.txt` |

## Enrolment

Enrolled: The X.509 profile behind every certificate IKEv2, EAP-TLS and Ze's own TLS listeners use. The 2026-10-26 walk classified all 280 normative sites: 102 MUST-level rows, of which 23 are {gap}, 53 are {lower-layer} (Go crypto/x509 encodes, parses and validates), and 26 are met by Ze's own code and owe tagged tests. CRL issuance binds a role Ze does not fill (it issues no CRL), and the extensions Ze's CA never issues are feature-out-of-scope in the extraction.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze's local CA mints a self-signed ECDSA P-256 root (cA TRUE, pathLenConstraint 0, keyCertSign and cRLSign) and short-lived leaves with a positive 128-bit random serial, digitalSignature key usage, serverAuth extended key usage and DNS or IP subject alternative names. Peer certificates are path-validated by Go crypto/x509 against the configured trust anchor, with basic constraints, key usage for certificate signing, path length, DNS, IP, email and URI name constraints and policy constraints enforced. EAP-TLS checks every chain certificate against a current CRL signed by its issuer.

**What the ledger says remains**

Ze's CA cannot issue a certificate policies extension; it maintains no status information for what it issues; the self-signed web certificate has an empty issuer DN; DNS subject alternative names are not checked for preferred name syntax, a single space or emptiness, and IDNs are refused rather than converted to ACE.

- **As a relying party:** no revocation check of IKE peer certificates (EAP-TLS checks CRLs); extended key usage is not restricted (ExtKeyUsageAny) and leaf key usage is not checked for IKE; version 1 CRLs are refused; a CRL or CRL entry with a critical extension Ze cannot process is still used, and no issuing distribution point, delta CRL or indirect CRL processing exists; Go crypto/x509 compares names bytewise instead of the Section 7.1 rules, rejects rather than processes directoryName constraints, skips minimum and maximum, never applies rfc822Name constraints to a subject emailAddress, and ignores policy qualifiers. Requirement tests are owed for every row Ze meets in its own code.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated (including scoped evidence) | 94 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **102** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC5280-4.1.2.6-5`](#rfc5280-4.1.2.6-5), [`RFC5280-4.2.1.6-7`](#rfc5280-4.2.1.6-7), [`RFC5280-4.2.1.6-10`](#rfc5280-4.2.1.6-10), [`RFC5280-4.2.1.9-1`](#rfc5280-4.2.1.9-1), [`RFC5280-4.2.1.9-5`](#rfc5280-4.2.1.9-5), [`RFC5280-5.1.1.3-1`](#rfc5280-5.1.1.3-1), [`RFC5280-6.3.3-1`](#rfc5280-6.3.3-1), [`RFC5280-7.2-2`](#rfc5280-7.2-2)

**Annotated (including scoped evidence) (94):** [`RFC5280-4.1.1.2-1`](#rfc5280-4.1.1.2-1), [`RFC5280-4.1.2.1-1`](#rfc5280-4.1.2.1-1), [`RFC5280-4.1.2.1-2`](#rfc5280-4.1.2.1-2), [`RFC5280-4.1.2.2-1`](#rfc5280-4.1.2.2-1), [`RFC5280-4.1.2.2-2`](#rfc5280-4.1.2.2-2), [`RFC5280-4.1.2.2-3`](#rfc5280-4.1.2.2-3), [`RFC5280-4.1.2.2-4`](#rfc5280-4.1.2.2-4), [`RFC5280-4.1.2.2-5`](#rfc5280-4.1.2.2-5), [`RFC5280-4.1.2.4-1`](#rfc5280-4.1.2.4-1), [`RFC5280-4.1.2.4-2`](#rfc5280-4.1.2.4-2), [`RFC5280-4.1.2.4-3`](#rfc5280-4.1.2.4-3), [`RFC5280-4.1.2.4-4`](#rfc5280-4.1.2.4-4), [`RFC5280-4.1.2.4-5`](#rfc5280-4.1.2.4-5), [`RFC5280-4.1.2.5-1`](#rfc5280-4.1.2.5-1), [`RFC5280-4.1.2.5-2`](#rfc5280-4.1.2.5-2), [`RFC5280-4.1.2.5-3`](#rfc5280-4.1.2.5-3), [`RFC5280-4.1.2.5.1-1`](#rfc5280-4.1.2.5.1-1), [`RFC5280-4.1.2.5.1-2`](#rfc5280-4.1.2.5.1-2), [`RFC5280-4.1.2.5.1-3`](#rfc5280-4.1.2.5.1-3), [`RFC5280-4.1.2.5.2-1`](#rfc5280-4.1.2.5.2-1), [`RFC5280-4.1.2.5.2-2`](#rfc5280-4.1.2.5.2-2), [`RFC5280-4.1.2.6-1`](#rfc5280-4.1.2.6-1), [`RFC5280-4.1.2.6-2`](#rfc5280-4.1.2.6-2), [`RFC5280-4.1.2.6-3`](#rfc5280-4.1.2.6-3), [`RFC5280-4.1.2.6-4`](#rfc5280-4.1.2.6-4), [`RFC5280-4.1.2.6-6`](#rfc5280-4.1.2.6-6), [`RFC5280-4.1.2.8-1`](#rfc5280-4.1.2.8-1), [`RFC5280-4.1.2.8-2`](#rfc5280-4.1.2.8-2), [`RFC5280-4.1.2.9-1`](#rfc5280-4.1.2.9-1), [`RFC5280-4.2-1`](#rfc5280-4.2-1), [`RFC5280-4.2-2`](#rfc5280-4.2-2), [`RFC5280-4.2-3`](#rfc5280-4.2-3), [`RFC5280-4.2-4`](#rfc5280-4.2-4), [`RFC5280-4.2-5`](#rfc5280-4.2-5), [`RFC5280-4.2-6`](#rfc5280-4.2-6), [`RFC5280-4.2.1.1-1`](#rfc5280-4.2.1.1-1), [`RFC5280-4.2.1.1-2`](#rfc5280-4.2.1.1-2), [`RFC5280-4.2.1.2-1`](#rfc5280-4.2.1.2-1), [`RFC5280-4.2.1.2-2`](#rfc5280-4.2.1.2-2), [`RFC5280-4.2.1.2-3`](#rfc5280-4.2.1.2-3), [`RFC5280-4.2.1.3-1`](#rfc5280-4.2.1.3-1), [`RFC5280-4.2.1.3-2`](#rfc5280-4.2.1.3-2), [`RFC5280-4.2.1.3-3`](#rfc5280-4.2.1.3-3), [`RFC5280-4.2.1.3-4`](#rfc5280-4.2.1.3-4), [`RFC5280-4.2.1.4-1`](#rfc5280-4.2.1.4-1), [`RFC5280-4.2.1.4-2`](#rfc5280-4.2.1.4-2), [`RFC5280-4.2.1.4-3`](#rfc5280-4.2.1.4-3), [`RFC5280-4.2.1.4-4`](#rfc5280-4.2.1.4-4), [`RFC5280-4.2.1.6-1`](#rfc5280-4.2.1.6-1), [`RFC5280-4.2.1.6-2`](#rfc5280-4.2.1.6-2), [`RFC5280-4.2.1.6-3`](#rfc5280-4.2.1.6-3), [`RFC5280-4.2.1.6-4`](#rfc5280-4.2.1.6-4), [`RFC5280-4.2.1.6-5`](#rfc5280-4.2.1.6-5), [`RFC5280-4.2.1.6-6`](#rfc5280-4.2.1.6-6), [`RFC5280-4.2.1.6-8`](#rfc5280-4.2.1.6-8), [`RFC5280-4.2.1.6-9`](#rfc5280-4.2.1.6-9), [`RFC5280-4.2.1.6-11`](#rfc5280-4.2.1.6-11), [`RFC5280-4.2.1.9-2`](#rfc5280-4.2.1.9-2), [`RFC5280-4.2.1.9-3`](#rfc5280-4.2.1.9-3), [`RFC5280-4.2.1.9-4`](#rfc5280-4.2.1.9-4), [`RFC5280-4.2.1.10-1`](#rfc5280-4.2.1.10-1), [`RFC5280-4.2.1.10-2`](#rfc5280-4.2.1.10-2), [`RFC5280-4.2.1.10-3`](#rfc5280-4.2.1.10-3), [`RFC5280-4.2.1.10-4`](#rfc5280-4.2.1.10-4), [`RFC5280-4.2.1.10-5`](#rfc5280-4.2.1.10-5), [`RFC5280-4.2.1.11-1`](#rfc5280-4.2.1.11-1), [`RFC5280-4.2.1.11-2`](#rfc5280-4.2.1.11-2), [`RFC5280-4.2.1.11-3`](#rfc5280-4.2.1.11-3), [`RFC5280-4.2.1.12-1`](#rfc5280-4.2.1.12-1), [`RFC5280-4.2.1.12-2`](#rfc5280-4.2.1.12-2), [`RFC5280-4.2.2.1-1`](#rfc5280-4.2.2.1-1), [`RFC5280-5-1`](#rfc5280-5-1), [`RFC5280-5.1.2.4-1`](#rfc5280-5.1.2.4-1), [`RFC5280-5.2-1`](#rfc5280-5.2-1), [`RFC5280-5.2.3-1`](#rfc5280-5.2.3-1), [`RFC5280-5.2.5-1`](#rfc5280-5.2.5-1), [`RFC5280-5.3-1`](#rfc5280-5.3-1), [`RFC5280-6.1-1`](#rfc5280-6.1-1), [`RFC5280-6.1-2`](#rfc5280-6.1-2), [`RFC5280-6.1.3-1`](#rfc5280-6.1.3-1), [`RFC5280-6.1.4-1`](#rfc5280-6.1.4-1), [`RFC5280-6.3-1`](#rfc5280-6.3-1), [`RFC5280-7.1-1`](#rfc5280-7.1-1), [`RFC5280-7.1-2`](#rfc5280-7.1-2), [`RFC5280-7.2-1`](#rfc5280-7.2-1), [`RFC5280-7.2-3`](#rfc5280-7.2-3), [`RFC5280-7.2-4`](#rfc5280-7.2-4), [`RFC5280-8-1`](#rfc5280-8-1), [`RFC5280-A.2-1`](#rfc5280-a.2-1), [`RFC5280-B-1`](#rfc5280-b-1), [`RFC5280-B-2`](#rfc5280-b-2), [`RFC5280-B-3`](#rfc5280-b-3), [`RFC5280-B-4`](#rfc5280-b-4), [`RFC5280-B-5`](#rfc5280-b-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5280-4.1.1.2-1` | This field MUST contain the same algorithm identifier as the signature field in the sequence tbsCertificate (Section 4.1.2.3). (§4.1.1.2) | MUST | 4.1.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.1-1` | When extensions are used, as expected in this profile, version MUST be 3 (value is 2). (§4.1.2.1) | MUST | 4.1.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.1-2` | At a minimum, conforming implementations MUST recognize version 3 certificates. (§4.1.2.1) | MUST | 4.1.2.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.2-1` | The serial number MUST be a positive integer assigned by the CA to each certificate. (§4.1.2.2) | MUST | 4.1.2.2 | **positive:** `unit/verify` [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L251). **negative:** no negative test. **{single-polarity}:** every serial internal/component/pki/ca.go::randomSerial draws is in [1, 2^128 - 1], so no input yields a non-positive serial to contrast |
| `RFC5280-4.1.2.2-2` | It MUST be unique for each certificate issued by a given CA (i.e., the issuer name and serial number identify a unique certificate). (§4.1.2.2) | MUST | 4.1.2.2 | **positive:** `unit/verify` [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L253). **negative:** no negative test. **{single-polarity}:** the CA keeps no ledger and draws each serial at random from 2^128 - 1 values, so there is no issuance that repeats a serial on purpose to contrast |
| `RFC5280-4.1.2.2-3` | CAs MUST force the serialNumber to be a non-negative integer. (§4.1.2.2) | MUST | 4.1.2.2 | **positive:** `unit/verify` [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L255). **negative:** no negative test. **{single-polarity}:** randomSerial adds one to a non-negative draw, so no input yields a negative serial to contrast |
| `RFC5280-4.1.2.2-4` | Certificate users MUST be able to handle serialNumber values up to 20 octets. (§4.1.2.2) | MUST | 4.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.2-5` | Conforming CAs MUST NOT use serialNumber values longer than 20 octets. (§4.1.2.2) | MUST NOT | 4.1.2.2 | **positive:** `unit/verify` [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L256). **negative:** no negative test. **{single-polarity}:** randomSerial draws below 2^128, so no input yields a serial longer than 17 DER octets to contrast |
| `RFC5280-4.1.2.4-1` | The issuer field MUST contain a non-empty distinguished name (DN). (§4.1.2.4) | MUST | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** GenerateWebCertWithNames signs a self-signed web certificate whose template sets no Subject, so its issuer is an empty DN; the pki CA names its issuer CN=Ze Local CA; internal/core/selfcert/selfcert.go::GenerateWebCertWithNames |
| `RFC5280-4.1.2.4-2` | CAs conforming to this profile MUST use either the PrintableString or UTF8String encoding of DirectoryString, with two exceptions. (§4.1.2.4) | MUST | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.4-3` | However, conforming implementations MUST be prepared to receive certificates with issuer names containing the set of attribute types defined below. (§4.1.2.4) | MUST | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.4-4` | In addition, implementations of this specification MUST be prepared to receive the domainComponent attribute, as defined in [RFC4519]. (§4.1.2.4) | MUST | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.4-5` | Certificate users MUST be prepared to process the issuer distinguished name and subject distinguished name (Section 4.1.2.6) fields to perform name chaining for certification path validation (Section 6). (§4.1.2.4) | MUST | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.1.2.5-1` | CAs conforming to this profile MUST always encode certificate validity dates through the year 2049 as UTCTime; certificate validity dates in 2050 or later MUST be encoded as GeneralizedTime. (§4.1.2.5) | MUST | 4.1.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.5-2` | Conforming applications MUST be able to process validity dates that are encoded in either UTCTime or GeneralizedTime. (§4.1.2.5) | MUST | 4.1.2.5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.5-3` | When the issuer will not be able to maintain status information until the notAfter date (including when the notAfter date is 99991231235959Z), the issuer MUST ensure that no valid certification path exists for the certificate after maintenance of status information is terminated. (§4.1.2.5) | MUST | 4.1.2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the pki CA maintains no status information for any certificate it issues (no CRL, no OCSP responder), and nothing ends the certification paths of its ten-year root or its leaves; internal/component/pki/ca.go::generateRoot |
| `RFC5280-4.1.2.5.1-1` | For the purposes of this profile, UTCTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.1) | MUST | 4.1.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.5.1-2` | Where YY is greater than or equal to 50, the year SHALL be interpreted as 19YY; and (§4.1.2.5.1) | SHALL | 4.1.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.5.1-3` | Where YY is less than 50, the year SHALL be interpreted as 20YY. (§4.1.2.5.1) | SHALL | 4.1.2.5.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.5.2-1` | For the purposes of this profile, GeneralizedTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYYYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.2) | MUST | 4.1.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.5.2-2` | GeneralizedTime values MUST NOT include fractional seconds. (§4.1.2.5.2) | MUST NOT | 4.1.2.5.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.6-1` | If the subject is a CA (e.g., the basic constraints extension, as discussed in Section 4.2.1.9, is present and the value of cA is TRUE), then the subject field MUST be populated with a non-empty distinguished name matching the contents of the issuer field (Section 4.1.2.4) in all certificates issued by the subject CA. (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L76). **negative:** no negative test. **{single-polarity}:** generateRoot always writes the non-empty subject CN=Ze Local CA and Root.IssueLeafFor always signs with that root as issuer, so there is no CA certificate with another subject to contrast |
| `RFC5280-4.1.2.6-2` | If the subject is a CRL issuer (e.g., the key usage extension, as discussed in Section 4.2.1.3, is present and the value of cRLSign is TRUE), then the subject field MUST be populated with a non-empty distinguished name matching the contents of the issuer field (Section 5.1.2.3) in all CRLs issued by the subject CRL issuer. (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L88). **negative:** no negative test. **{single-polarity}:** generateRoot is the only certificate Ze writes that asserts cRLSign and its subject is the constant CN=Ze Local CA, so there is no CRL issuer with an empty subject to contrast |
| `RFC5280-4.1.2.6-3` | If subject naming information is present only in the subjectAltName extension (e.g., a key bound only to an email address or URI), then the subject name MUST be an empty sequence and the subjectAltName extension MUST be critical. (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** `unit/verify` [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L33). **negative:** no negative test. **{single-polarity}:** internal/core/selfcert/selfcert.go::GenerateWebCertWithNames writes the only empty-subject certificate and Go crypto/x509 CreateCertificate marks its subjectAltName critical whenever the subject is empty, so no Ze path yields the other case to contrast |
| `RFC5280-4.1.2.6-4` | Where it is non-empty, the subject field MUST contain an X.500 distinguished name (DN). (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.6-5` | The DN MUST be unique for each subject entity certified by the one CA as defined by the issuer field. (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L227). **negative:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L238) |
| `RFC5280-4.1.2.6-6` | (a) When the subject of the certificate is a CA, the subject field MUST be encoded in the same way as it is encoded in the issuer field (Section 4.1.2.4) in all certificates issued by the subject CA. (§4.1.2.6) | MUST | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.8-1` | These fields MUST only appear if the version is 2 or 3 (Section 4.1.2.1). (§4.1.2.8) | MUST | 4.1.2.8 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.8-2` | CAs conforming to this profile MUST NOT generate certificates with unique identifiers. (§4.1.2.8) | MUST NOT | 4.1.2.8 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.1.2.9-1` | This field MUST only appear if the version is 3 (Section 4.1.2.1). (§4.1.2.9) | MUST | 4.1.2.9 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2-1` | A certificate-using system MUST reject the certificate if it encounters a critical extension it does not recognize or a critical extension that contains information that it cannot process. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2-2` | A non-critical extension MAY be ignored if it is not recognized, but MUST be processed if it is recognized. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2-3` | A certificate MUST NOT include more than one instance of a particular extension. (§4.2) | MUST NOT | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2-4` | Conforming CAs MUST support key identifiers (Sections 4.2.1.1 and 4.2.1.2), basic constraints (Section 4.2.1.9), key usage (Section 4.2.1.3), and certificate policies (Section 4.2.1.4) extensions. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the pki CA writes key identifiers (through CreateCertificate), basic constraints and key usage, but no template it builds can carry certificate policies; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2-5` | If the CA issues certificates with an empty sequence for the subject field, the CA MUST support the subject alternative name extension (Section 4.2.1.6). (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L37). **negative:** no negative test. **{single-polarity}:** the only empty-subject certificate Ze writes (GenerateWebCertWithNames) always carries the listener names in subjectAltName, and WebCertHosts always yields localhost and the loopback addresses, so there is no empty-subject certificate without the extension to contrast |
| `RFC5280-4.2-6` | At a minimum, applications conforming to this profile MUST recognize the following extensions: key usage (Section 4.2.1.3), certificate policies (Section 4.2.1.4), subject alternative name (Section 4.2.1.6), basic constraints (Section 4.2.1.9), name constraints (Section 4.2.1.10), policy constraints (Section 4.2.1.11), extended key usage (Section 4.2.1.12), and inhibit anyPolicy (Section 4.2.1.14). (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.2.1.1-1` | The keyIdentifier field of the authorityKeyIdentifier extension MUST be included in all certificates generated by conforming CAs to facilitate certification path construction. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.1-2` | Conforming CAs MUST mark this extension as non-critical. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.2-1` | To facilitate certification path construction, this extension MUST appear in all conforming CA certificates, that is, all certificates including the basic constraints extension (Section 4.2.1.9) where the value of cA is TRUE. (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.2-2` | In conforming CA certificates, the value of the subject key identifier MUST be the value placed in the key identifier field of the authority key identifier extension (Section 4.2.1.1) of certificates issued by the subject of this certificate. (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.2-3` | Conforming CAs MUST mark this extension as non-critical. (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.3-1` | Conforming CAs MUST include this extension in certificates that contain public keys that are used to validate digital signatures on other public key certificates or CRLs. (§4.2.1.3) | MUST | 4.2.1.3 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L97). **negative:** no negative test. **{single-polarity}:** generateRoot is the only certificate whose key validates signatures on other certificates and its template always sets keyUsage, so there is no CA certificate without the extension to contrast |
| `RFC5280-4.2.1.3-2` | If the keyCertSign bit is asserted, then the cA bit in the basic constraints extension (Section 4.2.1.9) MUST also be asserted. (§4.2.1.3) | MUST | 4.2.1.3 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L106). **negative:** no negative test. **{single-polarity}:** generateRoot is the only certificate asserting keyCertSign and its template always sets IsCA, so there is no keyCertSign certificate without cA to contrast |
| `RFC5280-4.2.1.3-3` | If the keyUsage extension is present, then the subject public key MUST NOT be used to verify signatures on certificates or CRLs unless the corresponding keyCertSign or cRLSign bit is set. (§4.2.1.3) | MUST NOT | 4.2.1.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.3-4` | When the keyUsage extension appears in a certificate, at least one of the bits MUST be set to 1. (§4.2.1.3) | MUST | 4.2.1.3 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L115). **negative:** no negative test. **{single-polarity}:** both templates set a constant non-zero keyUsage (generateRoot keyCertSign, cRLSign, digitalSignature; Root.IssueLeafFor digitalSignature), so no input yields an empty keyUsage to contrast |
| `RFC5280-4.2.1.4-1` | A certificate policy OID MUST NOT appear more than once in a certificate policies extension. (§4.2.1.4) | MUST NOT | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.4-2` | If this extension is critical, the path validation software MUST be able to interpret this extension (including the optional qualifier), or MUST reject the certificate. (§4.2.1.4) | MUST | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Go crypto/x509 parses the policy identifiers of a critical certificatePolicies extension and skips their qualifiers, so it neither interprets the qualifier nor rejects the certificate; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.1.4-3` | When qualifiers are used with the special policy anyPolicy, they MUST be limited to the qualifiers identified in this section. (§4.2.1.4) | MUST | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.4-4` | Conforming CAs MUST NOT encode explicitText as VisibleString or BMPString. (§4.2.1.4) | MUST NOT | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.6-1` | Whenever such identities are to be bound into a certificate, the subject alternative name (or issuer alternative name) extension MUST be used; however, a DNS name MAY also be represented in the subject field using the domainComponent attribute as described in Section 4.1.2.4. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L183). **negative:** no negative test. **{single-polarity}:** Root.IssueLeafFor binds every host it is given into subjectAltName and nothing else, so there is no input whose identity lands elsewhere to contrast |
| `RFC5280-4.2.1.6-2` | Because the subject alternative name is considered to be definitively bound to the public key, all parts of the subject alternative name MUST be verified by the CA. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L192). **negative:** no negative test. **{single-polarity}:** the CA issues only in process, to Ze components naming their own listener hosts, and Root.IssueLeafFor classifies each host by parsing it and binds it once in that form; the test asserts that classification and the refusal of an empty host list, and there is no unverified binding to contrast |
| `RFC5280-4.2.1.6-3` | Further, if the only subject identity included in the certificate is an alternative name form (e.g., an electronic mail address), then the subject distinguished name MUST be empty (an empty sequence), and the subjectAltName extension MUST be present. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** `unit/verify` [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L35). **negative:** no negative test. **{single-polarity}:** GenerateWebCertWithNames writes the only certificate whose subject identities are all alternative names, its subject is the empty sequence and its subjectAltName always holds the listener names, so no Ze path yields the other case to contrast |
| `RFC5280-4.2.1.6-4` | When the subjectAltName extension contains an iPAddress, the address MUST be stored in the octet string in "network byte order", as specified in [RFC791]. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.6-5` | For IP version 4, as specified in [RFC791], the octet string MUST contain exactly four octets. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.6-6` | For IP version 6, as specified in [RFC2460], the octet string MUST contain exactly sixteen octets. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-4.2.1.6-7` | When the subjectAltName extension contains a domain name system label, the domain name MUST be stored in the dNSName (an IA5String). (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L203). **negative:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L208) |
| `RFC5280-4.2.1.6-8` | The name MUST be in the "preferred name syntax", as specified by Section 3.5 of [RFC1034] and as modified by Section 2.1 of [RFC1123]. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.6-9` | In addition, while the string " " is a legal domain name, subjectAltName extensions with a dNSName of " " MUST NOT be used. (§4.2.1.6) | MUST NOT | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.6-10` | If the subjectAltName extension is present, the sequence MUST contain at least one entry. (§4.2.1.6) | MUST | 4.2.1.6 | **positive:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L216). **negative:** `unit/verify` [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L221) |
| `RFC5280-4.2.1.6-11` | Unlike the subject field, conforming CAs MUST NOT issue certificates with subjectAltNames containing empty GeneralName fields. (§4.2.1.6) | MUST NOT | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-4.2.1.9-1` | If the cA boolean is not asserted, then the keyCertSign bit in the key usage extension MUST NOT be asserted. (§4.2.1.9) | MUST NOT | 4.2.1.9 | **positive:** `unit/verify` [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L143). **negative:** `unit/verify` [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L151) |
| `RFC5280-4.2.1.9-2` | If the basic constraints extension is not present in a version 3 certificate, or the extension is present but the cA boolean is not asserted, then the certified public key MUST NOT be used to verify certificate signatures. (§4.2.1.9) | MUST NOT | 4.2.1.9 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.9-3` | Where it appears, the pathLenConstraint field MUST be greater than or equal to zero. (§4.2.1.9) | MUST | 4.2.1.9 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L124). **negative:** no negative test. **{single-polarity}:** generateRoot writes the constant pathLenConstraint 0 and Root.IssueLeafFor writes none, so no input yields a negative value to contrast |
| `RFC5280-4.2.1.9-4` | Conforming CAs MUST include this extension in all CA certificates that contain public keys used to validate digital signatures on certificates and MUST mark the extension as critical in such certificates. (§4.2.1.9) | MUST | 4.2.1.9 | **positive:** `unit/verify` [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L130). **negative:** no negative test. **{single-polarity}:** generateRoot is the only CA certificate Ze writes and its template always sets BasicConstraintsValid, so there is no CA certificate without the extension to contrast |
| `RFC5280-4.2.1.9-5` | CAs MUST NOT include the pathLenConstraint field unless the cA boolean is asserted and the key usage extension asserts the keyCertSign bit. (§4.2.1.9) | MUST NOT | 4.2.1.9 | **positive:** `unit/verify` [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L157). **negative:** `unit/verify` [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L165) |
| `RFC5280-4.2.1.10-1` | Applications conforming to this profile MUST be able to process name constraints that are imposed on the directoryName name form and SHOULD be able to process name constraints that are imposed on the rfc822Name, uniformResourceIdentifier, dNSName, and iPAddress name forms. (§4.2.1.10) | MUST | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Go crypto/x509 handles no directoryName constraint: parseNameConstraintsExtension marks the extension unhandled, so x509.Certificate.Verify rejects a certificate under one instead of processing it; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.1.10-2` | If a name constraints extension that is marked as critical imposes constraints on a particular name form, and an instance of that name form appears in the subject field or subjectAltName extension of a subsequent certificate, then the application MUST either process the constraint or reject the certificate. (§4.2.1.10) | MUST | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.10-3` | However, if an application encounters a critical name constraints extension that specifies other values for minimum or maximum for a name form that appears in a subsequent certificate, the application MUST either process these fields or reject the certificate. (§4.2.1.10) | MUST | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Go crypto/x509 reads the base of each GeneralSubtree and skips minimum and maximum, so it neither processes them nor rejects the certificate; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.1.10-4` | If a constraint is applied to the uniformResourceIdentifier name form and a subsequent certificate includes a subjectAltName extension with a uniformResourceIdentifier that does not include an authority component with a host name specified as a fully qualified domain name (e.g., if the URI either does not include an authority component or includes an authority component in which the host name is specified as an IP address), then the application MUST reject the certificate. (§4.2.1.10) | MUST | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.10-5` | When constraints are imposed on the rfc822Name name form, but the certificate does not include a subject alternative name, the rfc822Name constraint MUST be applied to the attribute of type emailAddress in the subject distinguished name. (§4.2.1.10) | MUST | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Go crypto/x509 applies rfc822Name constraints to subjectAltName entries only, never to an emailAddress attribute of the subject DN; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.1.11-1` | Conforming applications MUST be able to process the requireExplicitPolicy field and SHOULD be able to process the inhibitPolicyMapping field. (§4.2.1.11) | MUST | 4.2.1.11 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.11-2` | Applications that support the inhibitPolicyMapping field MUST also implement support for the policyMappings extension. (§4.2.1.11) | MUST | 4.2.1.11 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.11-3` | If the policyConstraints extension is marked as critical and the inhibitPolicyMapping field is present, applications that do not implement support for the inhibitPolicyMapping field MUST reject the certificate. (§4.2.1.11) | MUST | 4.2.1.11 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-4.2.1.12-1` | If the extension is present, then the certificate MUST only be used for one of the purposes indicated. (§4.2.1.12) | MUST | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** getRemoteCert and verifyEntryChain pass KeyUsages ExtKeyUsageAny to x509.Certificate.Verify and check no key usage bit of the leaf, so a peer certificate whose extensions name no purpose IKE fills is still used for IKE authentication; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.1.12-2` | If a certificate contains both a key usage extension and an extended key usage extension, then both extensions MUST be processed independently and the certificate MUST only be used for a purpose consistent with both extensions. (§4.2.1.12) | MUST | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** getRemoteCert and verifyEntryChain pass KeyUsages ExtKeyUsageAny to x509.Certificate.Verify and check no key usage bit of the leaf, so a peer certificate whose extensions name no purpose IKE fills is still used for IKE authentication; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-4.2.2.1-1` | Conforming applications that support HTTP or FTP for accessing certificates MUST be able to accept individual DER encoded certificates and SHOULD be able to accept "certs-only" CMS messages. (§4.2.2.1) | MUST | 4.2.2.1 | **positive:** `unit/verify` [`TestRFC7296ReceivedHashAndURLFormatsAreAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_hash_url_accept_test.go#L69). **negative:** no negative test. **{single-polarity}:** Ze accesses certificates over HTTP only through RFC 7296 Hash and URL (internal/component/ike/engine/certurl.go::fetchHashAndURL), and encoding 12 is defined as one individual DER certificate, so there is no other format it must also refuse to contrast |
| `RFC5280-5-1` | Conforming applications that support CRLs are REQUIRED to process both version 1 and version 2 complete CRLs that provide revocation information for all certificates issued by one CA. (§5) | REQUIRED | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** x509.ParseRevocationList refuses a CRL without a version field (a version 1 CRL), and parseCRLs and the pki crl leaf-list refuse the material with it; internal/core/eap/revocation.go::parseCRLs |
| `RFC5280-5.1.1.3-1` | Applications that perform CRL checking MUST support certification path validation when certificates and CRLs are digitally signed with the same CA private key. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L49). **negative:** `unit/verify` [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L55) |
| `RFC5280-5.1.2.4-1` | Conforming applications MUST be able to process dates that are encoded in either UTCTime or GeneralizedTime. (§5.1.2.4) | MUST | 5.1.2.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseRevocationList; internal/core/eap/revocation.go::parseCRLs parses every configured CRL with x509.ParseRevocationList, which decodes this field; ze installs no value it reads |
| `RFC5280-5.2-1` | If a CRL contains a critical extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of certificates. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| `RFC5280-5.2.3-1` | CRL verifiers MUST be able to handle CRLNumber values up to 20 octets. (§5.2.3) | MUST | 5.2.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseRevocationList; internal/core/eap/revocation.go::parseCRLs parses every configured CRL with x509.ParseRevocationList, which decodes this field; ze installs no value it reads |
| `RFC5280-5.2.5-1` | However, implementations that do not support this extension MUST either treat the status of any certificate not listed on this CRL as unknown or locate another CRL that does not contain any unrecognized critical extensions. (§5.2.5) | MUST | 5.2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| `RFC5280-5.3-1` | If a CRL contains a critical CRL entry extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of any certificates. (§5.3) | MUST NOT | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| `RFC5280-6.1-1` | A conforming implementation MUST include an X.509 path processing procedure that is functionally equivalent to the external behavior of this algorithm. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-6.1-2` | A certificate MUST NOT appear more than once in a prospective certification path. (§6.1) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-6.1.3-1` | The certificate MUST satisfy each of the following: (§6.1.3) | MUST | 6.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** x509.Certificate.Verify checks the signature, the validity period and issuer name chaining, and nothing checks condition (a)(3), the revocation status, of an IKE peer certificate: getRemoteCert runs no CRL or OCSP check (EAP-TLS does, in internal/core/eap/revocation.go::checkChainRevocation); internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-6.1.4-1` | (If certificate i is a version 1 or version 2 certificate, then the application MUST either verify that certificate i is a CA certificate through out-of-band means or reject the certificate. (§6.1.4) | MUST | 6.1.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-6.3-1` | Conforming implementations that support CRLs are not required to implement this algorithm, but they MUST be functionally equivalent to the external behavior resulting from this procedure when processing CRLs that are issued in conformance with this profile. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkChainRevocation matches a list to the issuer, checks its signature and its nextUpdate, and implements none of the scope, reason, delta and critical extension steps of Section 6.3.3; internal/core/eap/revocation.go::checkChainRevocation |
| `RFC5280-6.3.3-1` | The trust anchor for the certification path MUST be the same as the trust anchor used to validate the target certificate. (§6.3.3) | MUST | 6.3.3 | **positive:** `unit/verify` [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L61). **negative:** `unit/verify` [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L74) |
| `RFC5280-7.1-1` | Conforming implementations MUST support UTF8String and PrintableString. (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-7.1-2` | Conforming implementations MUST use the LDAP StringPrep profile (including insignificant space handling), as specified in [RFC4518], as the basis for comparison of distinguished name attributes encoded in either PrintableString or UTF8String. (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Go crypto/x509 chains names by comparing the DER bytes of issuer and subject, and performs no RFC 4518 string preparation or caseIgnoreMatch; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| `RFC5280-7.2-1` | To accommodate internationalized domain names in the current structure, conforming implementations MUST convert internationalized domain names to the ASCII Compatible Encoding (ACE) format as specified in Section 4 of RFC 3490 before storage in the dNSName field. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Root.IssueLeafFor and GenerateWebCertWithNames convert no internationalized name: CreateCertificate refuses a dNSName that is not an IA5String, so an IDN host is refused rather than stored in ACE form; internal/component/pki/ca.go::Root.IssueLeafFor |
| `RFC5280-7.2-2` | When comparing DNS names for equality, conforming implementations MUST perform a case-insensitive exact match on the entire DNS name. (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestRFC5280DNSNamesCompareCaseInsensitiveAndExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L38). **negative:** `unit/verify` [`TestRFC5280DNSNamesCompareCaseInsensitiveAndExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L48) |
| `RFC5280-7.2-3` | When evaluating name constraints, conforming implementations MUST perform a case-insensitive exact match on a label-by-label basis. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| `RFC5280-7.2-4` | Note: Implementations MUST allow for increased space requirements for IDNs. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-8-1` | Relying parties that choose to validate the server's certificate when obtaining information pointed to by an https URI in the cRLDistributionPoints, authorityInfoAccess, or subjectInfoAccess extensions MUST be prepared for the possibility that this will result in unbounded recursion. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC5280OCSPFetchDoesNotRecurseIntoTheResponderCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L98). **negative:** no negative test. **{single-polarity}:** internal/component/ike/engine/postauth_revocation.go::checkServerChainStatus is the only fetch of an https URI a certificate names, and validating the responder certificate goes through crypto/tls, which fetches nothing, so there is no recursing configuration to contrast |
| `RFC5280-A.2-1` | AuthorityKeyIdentifier ::= SEQUENCE { keyIdentifier [0] KeyIdentifier OPTIONAL, authorityCertIssuer [1] GeneralNames OPTIONAL, authorityCertSerialNumber [2] CertificateSerialNumber OPTIONAL } -- authorityCertIssuer and authorityCertSerialNumber MUST both -- be present or both be absent (§A.2) | MUST | A.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-B-1` | Conforming implementations MUST NOT encode strings that include either the at sign or underscore character as PrintableString. (§B) | MUST NOT | B | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-B-2` | When DER encoding a named bit list, trailing zeros MUST be omitted. (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| `RFC5280-B-3` | This specification mandates support for OIDs that have arc elements with values that are less than 2^28, that is, they MUST be between 0 and 268,435,455, inclusive. (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-B-4` | Implementations MUST also support OIDs where the length of the dotted decimal (see Section 1.4 of [RFC4512]) string representation can be up to 100 bytes (inclusive). (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-B-5` | Implementations MUST be able to handle OIDs with up to 20 elements (inclusive). (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| `RFC5280-4.1.2.1-3` | If no extensions are present, but a UniqueIdentifier is present, the version SHOULD be 2 (value is 1); however, the version MAY be 3. (§4.1.2.1) | SHOULD | 4.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.1-4` | If only basic fields are present, the version SHOULD be 1 (the value is omitted from the certificate as the default value); however, the version MAY be 2 or 3. (§4.1.2.1) | SHOULD | 4.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.1-5` | Implementations SHOULD be prepared to accept any version certificate. (§4.1.2.1) | SHOULD | 4.1.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.2-6` | Certificate users SHOULD be prepared to gracefully handle such certificates. (§4.1.2.2) | SHOULD | 4.1.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.4-8` | In addition, implementations of this specification SHOULD be prepared to receive the following standard attribute types in issuer and subject names: (§4.1.2.4) | SHOULD | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.5-4` | To indicate that a certificate has no well-defined expiration date, the notAfter SHOULD be assigned the GeneralizedTime value of 99991231235959Z. (§4.1.2.5) | SHOULD | 4.1.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.6-9` | Implementations of this specification SHOULD be prepared to receive subject names containing the recommended attribute types for the issuer field. (§4.1.2.6) | SHOULD | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.8-3` | this profile SHOULD be capable of parsing certificates that include unique identifiers, but there are no processing requirements associated with the unique identifiers. (§4.1.2.8) | SHOULD | 4.1.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2-9` | In addition, applications conforming to this profile SHOULD recognize the authority and subject key identifier (Sections 4.2.1.1 and 4.2.1.2) and policy mappings (Section 4.2.1.5) extensions. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.1-5` | The value of the keyIdentifier field SHOULD be derived from the public key used to verify the certificate's signature or a method (§4.2.1.1) | SHOULD | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.1-6` | Where a key identifier has been previously established, the CA SHOULD use the previously established identifier. (§4.2.1.1) | SHOULD | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.2-4` | For CA certificates, subject key identifiers SHOULD be derived from the public key or a method that generates unique values. (§4.2.1.2) | SHOULD | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.2-5` | To assist applications in identifying the appropriate end entity certificate, this extension SHOULD be included in all end entity certificates. (§4.2.1.2) | SHOULD | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.2-6` | For end entity certificates, subject key identifiers SHOULD be derived from the public key. (§4.2.1.2) | SHOULD | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.2-7` | Where a key identifier has been previously established, the CA SHOULD use the previously established identifier. (§4.2.1.2) | SHOULD | 4.2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.3-5` | When present, conforming CAs SHOULD mark this extension as critical. (§4.2.1.3) | SHOULD | 4.2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.3-6` | If the subject public key is only to be used for verifying signatures on certificates and/or CRLs, then the digitalSignature and nonRepudiation bits SHOULD NOT be set. (§4.2.1.3) | SHOULD NOT | 4.2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-7` | To prevent such duplication, this qualifier SHOULD only be present in end entity certificates and CA certificates issued to other organizations. (§4.2.1.4) | SHOULD | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-8` | Conforming CAs SHOULD NOT use the noticeRef option. (§4.2.1.4) | SHOULD NOT | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-10` | Conforming CAs SHOULD use the UTF8String encoding for explicitText, but MAY use IA5String. (§4.2.1.4) | SHOULD | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-11` | The explicitText string SHOULD NOT include any control characters (e.g., U+0000 to U+001F and U+007F to U+009F). (§4.2.1.4) | SHOULD NOT | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-12` | When the UTF8String encoding is used, all character sequences SHOULD be normalized according to Unicode normalization form C (NFC) [NFC]. (§4.2.1.4) | SHOULD | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-13` | If both the noticeRef and explicitText options are included in the one qualifier and if the application software can locate the notice text indicated by the noticeRef option, then that text SHOULD be displayed; otherwise, the explicitText string SHOULD be displayed. (§4.2.1.4) | SHOULD | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-14` | Therefore, certificate users SHOULD gracefully handle explicitText with more than 200 characters. (§4.2.1.4) | SHOULD | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.5-1` | Each issuerDomainPolicy named in the policy mappings extension SHOULD also be asserted in a certificate policies extension in the same certificate. (§4.2.1.5) | SHOULD | 4.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.5-3` | Conforming CAs SHOULD mark this extension as critical. (§4.2.1.5) | SHOULD | 4.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-13` | When including the subjectAltName extension in a certificate that has a non-empty subject distinguished name, conforming CAs SHOULD mark the subjectAltName extension as non-critical. (§4.2.1.6) | SHOULD | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.7-1` | Where present, conforming CAs SHOULD mark this extension as non- critical. (§4.2.1.7) | SHOULD | 4.2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.10-8` | CAs issuing certificates with a restriction of the form directoryName SHOULD NOT rely on implementation of the full ISO DN name comparison algorithm. (§4.2.1.10) | SHOULD NOT | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.12-5` | Conforming CAs SHOULD NOT mark this extension as critical if the anyExtendedKeyUsage KeyPurposeId is present. (§4.2.1.12) | SHOULD NOT | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.13-1` | The extension SHOULD be non-critical, but this profile RECOMMENDS support for this extension by CAs and applications. (§4.2.1.13) | SHOULD | 4.2.1.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.13-2` | HTTP server implementations accessed via the URI SHOULD specify the media type application/pkix-crl in the content-type header field of the response. (§4.2.1.13) | SHOULD | 4.2.1.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.13-3` | When present, DistributionPointName SHOULD include at least one LDAP or HTTP URI. (§4.2.1.13) | SHOULD | 4.2.1.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.13-4` | Conforming CAs SHOULD NOT use nameRelativeToCRLIssuer to specify distribution point names. (§4.2.1.13) | SHOULD NOT | 4.2.1.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.1-2` | Where the information is available via LDAP, the accessLocation SHOULD be a uniformResourceIdentifier. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.1-3` | HTTP server implementations accessed via the URI SHOULD specify the media type application/pkix-cert [RFC2585] in the content-type header field of the response for a single DER encoded certificate and SHOULD specify the media type application/pkcs7-mime [RFC2797] in the content-type header field of the response for "certs-only" CMS messages. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.1-4` | For FTP, the name of a file that contains a single DER encoded certificate SHOULD have a suffix of ".cer" [RFC2585] and the name of a file that contains a "certs-only" CMS message SHOULD have a suffix of ".p7c" [RFC2797]. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.1-5` | When the id-ad-caIssuers accessMethod is used, at least one instance SHOULD specify an accessLocation that is an HTTP [RFC2616] or LDAP [RFC4516] URI. (§4.2.2.1) | SHOULD | 4.2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.2-1` | Where the information is available via LDAP, the accessLocation SHOULD be a uniformResourceIdentifier. (§4.2.2.2) | SHOULD | 4.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.2-2` | HTTP server implementations accessed via the URI SHOULD specify the media type application/pkix-cert [RFC2585] in the content-type header field of the response for a single DER encoded certificate and SHOULD specify the media type application/pkcs7-mime [RFC2797] in the content-type header field of the response for "certs-only" CMS messages. (§4.2.2.2) | SHOULD | 4.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.2-3` | For FTP, the name of a file that contains a single DER encoded certificate SHOULD have a suffix of ".cer" [RFC2585] and the name of a file that contains a "certs-only" CMS message SHOULD have a suffix of ".p7c" [RFC2797]. (§4.2.2.2) | SHOULD | 4.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.2.2-4` | When the id-ad-caRepository accessMethod is used, at least one instance SHOULD specify an accessLocation that is an HTTP [RFC2616] or LDAP [RFC4516] URI. (§4.2.2.2) | SHOULD | 4.2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.1.1.3-3` | These applications SHOULD support certification path validation when certificates and CRLs are digitally signed with different CA private keys. (§5.1.1.3) | SHOULD | 5.1.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.1.2.5-1` | CRL issuers SHOULD issue CRLs with a nextUpdate time equal to or later than all previous CRLs. nextUpdate may be encoded as UTCTime or GeneralizedTime. (§5.1.2.5) | SHOULD | 5.1.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.2-1` | Conforming CRL issuers SHOULD mark the issuerAltName extension as non-critical. (§5.2.2) | SHOULD | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.4-4` | If more than one current delta CRL for a given scope is encountered, the application SHOULD consider the one with the latest value in thisUpdate to be the most current one. (§5.2.4) | SHOULD | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.7-1` | At least one instance of AccessDescription SHOULD specify an accessLocation that is an HTTP [RFC2616] or LDAP [RFC4516] URI. (§5.2.7) | SHOULD | 5.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.7-2` | HTTP server implementations accessed via the URI SHOULD specify the media type application/pkix-cert [RFC2585] in the content-type header field of the response for a single DER encoded certificate and SHOULD specify the media type application/pkcs7-mime [RFC2797] in the content-type header field of the response for "certs-only" CMS messages. (§5.2.7) | SHOULD | 5.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.7-3` | For FTP, the name of a file that contains a single DER encoded certificate SHOULD have a suffix of ".cer" [RFC2585] and the name of a file that contains a "certs-only" CMS message SHOULD have a suffix of ".p7c" [RFC2797]. (§5.2.7) | SHOULD | 5.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.7-4` | Where the information is available via LDAP, the accessLocation SHOULD be a uniformResourceIdentifier. (§5.2.7) | SHOULD | 5.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.3-2` | However, CRL issuers SHOULD include reason codes (Section 5.3.1) and invalidity dates (Section 5.3.2) whenever this information is available. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.3.1-1` | CRL issuers are strongly encouraged to include meaningful reason codes in CRL entries; however, the reason code CRL entry extension SHOULD be absent instead of using the unspecified (0) reasonCode value. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.3.2-1` | When a revocation is first posted by a CRL issuer in a CRL, the invalidity date may precede the date of issue of earlier CRLs, but the revocation date SHOULD NOT precede the date of issue of earlier CRLs. (§5.3.2) | SHOULD NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-2` | CAs SHOULD maintain secure backup for signing keys. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-3` | CAs SHOULD take extra care when making revocation information available only through CRLs that contain critical extensions, particularly if support for those extensions is not mandated by this profile. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-4` | In addition, CAs SHOULD decline to issue certificates to CAs or end entities that generate weak signatures. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-5` | To avoid acceptance of invalid paths, CAs SHOULD state name constraints for distinguished names as permittedSubtrees wherever possible. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-6` | As a means of reducing problems and security issues related to issuer name collisions, CA and CRL issuer names SHOULD be formed in a way that reduces the likelihood of name collisions. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-7` | CAs SHOULD NOT include URIs that specify https, ldaps, or similar schemes in extensions. (§8) | SHOULD NOT | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-B-6` | CAs SHOULD NOT issue certificates that contain OIDs that exceed these requirements. (§B) | SHOULD NOT | B | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-B-7` | Likewise, CRL issuers SHOULD NOT issue CRLs that contain OIDs that exceed these requirements. (§B) | SHOULD NOT | B | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.1.2-2` | The signatureAlgorithm field contains the identifier for the cryptographic algorithm used by the CA to sign this certificate. [RFC3279], [RFC4055], and [RFC4491] list supported signature algorithms, but other signature algorithms MAY also be supported. (§4.1.1.2) | MAY | 4.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2-1` | key associated with the subject, a validity period, a version number, and a serial number; some MAY contain optional unique identifier fields. (§4.1.2) | MAY | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.3-1` | The contents of the optional parameters field will vary according to the algorithm identified. [RFC3279], [RFC4055], and [RFC4491] list supported signature algorithms, but other signature algorithms MAY also be supported. (§4.1.2.3) | MAY | 4.1.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.4-6` | When CAs have previously issued certificates with issuer fields with attributes encoded using TeletexString, BMPString, or UniversalString, then the CA MAY continue to use these encodings of the DirectoryString to preserve backward compatibility. (§4.1.2.4) | MAY | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.4-7` | CAs that are added to a domain where existing CAs issue certificates with issuer fields with attributes encoded using TeletexString, BMPString, or UniversalString MAY encode attributes that they share with the existing CAs using the same encodings as the existing CAs use. (§4.1.2.4) | MAY | 4.1.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.6-7` | The subject name MAY be carried in the subject field and/or the subjectAltName extension. (§4.1.2.6) | MAY | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.6-8` | A CA MAY issue more than one certificate with the same DN to the same subject entity. (§4.1.2.6) | MAY | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.1.2.6-10` | Implementations of this specification MAY use the comparison rules in Section 7.1 to process unfamiliar attribute types (i.e., for name chaining) whose attribute values use one of the encoding options from DirectoryString. (§4.1.2.6) | MAY | 4.1.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2-7` | Support for the remaining extensions is OPTIONAL. (§4.2) | OPTIONAL | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2-8` | Conforming CAs MAY support extensions that are not identified within (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.1-3` | The identification MAY be based on either the key identifier (the subject key identifier in the issuer's certificate) or the issuer name and serial number. (§4.2.1.1) | MAY | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.1-4` | There is one exception; where a CA distributes its public key in the form of a "self-signed" certificate, the authority key identifier MAY be omitted. (§4.2.1.1) | MAY | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.3-7` | However, the digitalSignature and/or nonRepudiation bits MAY be set in addition to the keyCertSign and/or cRLSign bits if the subject public key is to be used to verify signatures on certificates and/or CRLs as well as other objects. (§4.2.1.3) | MAY | 4.2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-5` | Optional qualifiers, which MAY be present, are not expected to change the definition of the policy. (§4.2.1.4) | MAY | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-6` | When a CA does not wish to limit the set of policies for certification paths that include this certificate, it MAY assert the special policy anyPolicy, with a value of { 2 5 29 32 0 }. (§4.2.1.4) | MAY | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.4-9` | Messages MAY be multilingual, allowing the software to select the particular language message for its own environment. (§4.2.1.4) | MAY | 4.2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.5-2` | This extension MAY be supported by CAs and/or applications. (§4.2.1.5) | MAY | 4.2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-12` | Multiple name forms, and multiple instances of each name form, MAY be included. (§4.2.1.6) | MAY | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-14` | A CA MAY issue more than one certificate with the same DN to the same subject entity. (§4.2.1.6) | MAY | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-15` | The subjectAltName MAY carry additional name types through the use of the otherName field. (§4.2.1.6) | MAY | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-16` | Subject alternative names MAY be constrained in the same manner as subject distinguished names using the name constraints extension as described in Section 4.2.1.10. (§4.2.1.6) | MAY | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.6-17` | Applications with specific requirements MAY use such names, but they must define the semantics. (§4.2.1.6) | MAY | 4.2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.9-6` | This extension MAY appear as a critical or non- critical extension in CA certificates that contain public keys used exclusively for purposes other than validating digital signatures on certificates. (§4.2.1.9) | MAY | 4.2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.9-7` | This extension MAY appear as a critical or non-critical extension in end entity certificates. (§4.2.1.9) | MAY | 4.2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.10-6` | When the constraint begins with a period, it MAY be expanded with one or more labels. (§4.2.1.10) | MAY | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.10-7` | A name constraint for Internet mail addresses MAY specify a particular mailbox, all addresses at a particular host, or all mailboxes in a domain. (§4.2.1.10) | MAY | 4.2.1.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.12-3` | This extension MAY, at the option of the certificate issuer, be either critical or non-critical. (§4.2.1.12) | MAY | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.12-4` | Certificate using applications MAY require that the extended key usage extension be present and that a particular purpose be indicated in order for the certificate to be acceptable to that application. (§4.2.1.12) | MAY | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-4.2.1.12-6` | Applications that require the presence of a particular purpose MAY reject certificates that include the anyExtendedKeyUsage OID but not the particular OID expected for the application. (§4.2.1.12) | MAY | 4.2.1.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5-2` | If the issuer of the indirect CRL is a CA, then the scope of the indirect CRL MAY also include certificates issued by the issuer of the CRL. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5-3` | The CRL issuer MAY also generate delta CRLs. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.1.1.2-1` | The field is of type AlgorithmIdentifier, which is defined in Section 4.1.1.2. [RFC3279], [RFC4055], and [RFC4491] list supported algorithms for this specification, but other signature algorithms MAY also be supported. (§5.1.1.2) | MAY | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.1.1.3-2` | CAs that are also CRL issuers MAY use one private key to digitally sign certificates and CRLs, or MAY use separate private keys to digitally sign certificates and CRLs. (§5.1.1.3) | MAY | 5.1.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.1.2.5-2` | Note that the ASN.1 syntax of TBSCertList describes this field as OPTIONAL, which is consistent with the ASN.1 structure defined in [X.509]. (§5.1.2.5) | OPTIONAL | 5.1.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.4-1` | A complete CRL and a delta CRL MAY be combined if the following four conditions are satisfied: (§5.2.4) | MAY | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.4-2` | A CRL issuer MAY optionally list a certificate on a delta CRL with reason code removeFromCRL if the notAfter time specified in the certificate precedes the thisUpdate time specified in the delta CRL and the certificate was listed on the referenced base CRL or in any CRL issued after the base but before this delta CRL. (§5.2.4) | MAY | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-5.2.4-3` | An application that supports delta CRLs MAY also be able to construct a current complete CRL by combining a previously locally constructed complete CRL and the current delta CRL. (§5.2.4) | MAY | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-3` | However, support for some of the certificate extensions processed in this algorithm are OPTIONAL for compliant implementations. (§6.1) | OPTIONAL | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-4` | Clients that do not support these extensions MAY omit the corresponding steps in the path validation algorithm. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-5` | Clients that do not support this extension MAY omit the path validation steps where policy mappings are processed. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-6` | A conforming implementation MAY also support validation with respect to some point in the past. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-7` | Different trust anchors MAY be used to validate different paths, as discussed further in Section 6.2. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.1-8` | Therefore, an application MAY augment this algorithm to further limit the set of valid paths. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.2-1` | An implementation MAY augment the algorithm presented in Section 6.1 to further limit the set of valid certification paths that begin with a particular trust anchor. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-6.2-2` | For example, an implementation MAY modify the algorithm to apply a path length constraint to a specific trust anchor during the initialization phase, or the application MAY require the presence of a particular alternative name form in the target certificate, or the application MAY impose requirements on application-specific extensions. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-7.1-3` | Implementations may encounter certificates and CRLs with names encoded using TeletexString, BMPString, or UniversalString, but support for these is OPTIONAL. (§7.1) | OPTIONAL | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5280-8-8` | Self-issued certificates MAY be used to support other changes in CA operations, such as additions to the CA's policy set, using similar procedures. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5280-4.1.1.2-1`](#rfc5280-4.1.1.2-1) This field MUST contain the same algorithm identifier as the signature field in the sequence tbsCertificate (Section 4.1.2.3). (§4.1.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.1-1`](#rfc5280-4.1.2.1-1) When extensions are used, as expected in this profile, version MUST be 3 (value is 2). (§4.1.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.1-2`](#rfc5280-4.1.2.1-2) At a minimum, conforming implementations MUST recognize version 3 certificates. (§4.1.2.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.2-4`](#rfc5280-4.1.2.2-4) Certificate users MUST be able to handle serialNumber values up to 20 octets. (§4.1.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.4-1`](#rfc5280-4.1.2.4-1) The issuer field MUST contain a non-empty distinguished name (DN). (§4.1.2.4) | {gap}, no test | GenerateWebCertWithNames signs a self-signed web certificate whose template sets no Subject, so its issuer is an empty DN; the pki CA names its issuer CN=Ze Local CA; internal/core/selfcert/selfcert.go::GenerateWebCertWithNames |
| [`RFC5280-4.1.2.4-2`](#rfc5280-4.1.2.4-2) CAs conforming to this profile MUST use either the PrintableString or UTF8String encoding of DirectoryString, with two exceptions. (§4.1.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.4-3`](#rfc5280-4.1.2.4-3) However, conforming implementations MUST be prepared to receive certificates with issuer names containing the set of attribute types defined below. (§4.1.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.4-4`](#rfc5280-4.1.2.4-4) In addition, implementations of this specification MUST be prepared to receive the domainComponent attribute, as defined in [RFC4519]. (§4.1.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.4-5`](#rfc5280-4.1.2.4-5) Certificate users MUST be prepared to process the issuer distinguished name and subject distinguished name (Section 4.1.2.6) fields to perform name chaining for certification path validation (Section 6). (§4.1.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.1.2.5-1`](#rfc5280-4.1.2.5-1) CAs conforming to this profile MUST always encode certificate validity dates through the year 2049 as UTCTime; certificate validity dates in 2050 or later MUST be encoded as GeneralizedTime. (§4.1.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.5-2`](#rfc5280-4.1.2.5-2) Conforming applications MUST be able to process validity dates that are encoded in either UTCTime or GeneralizedTime. (§4.1.2.5) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.5-3`](#rfc5280-4.1.2.5-3) When the issuer will not be able to maintain status information until the notAfter date (including when the notAfter date is 99991231235959Z), the issuer MUST ensure that no valid certification path exists for the certificate after maintenance of status information is terminated. (§4.1.2.5) | {gap}, no test | the pki CA maintains no status information for any certificate it issues (no CRL, no OCSP responder), and nothing ends the certification paths of its ten-year root or its leaves; internal/component/pki/ca.go::generateRoot |
| [`RFC5280-4.1.2.5.1-1`](#rfc5280-4.1.2.5.1-1) For the purposes of this profile, UTCTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.5.1-2`](#rfc5280-4.1.2.5.1-2) Where YY is greater than or equal to 50, the year SHALL be interpreted as 19YY; and (§4.1.2.5.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.5.1-3`](#rfc5280-4.1.2.5.1-3) Where YY is less than 50, the year SHALL be interpreted as 20YY. (§4.1.2.5.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.1.2.5.2-1`](#rfc5280-4.1.2.5.2-1) For the purposes of this profile, GeneralizedTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYYYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.5.2-2`](#rfc5280-4.1.2.5.2-2) GeneralizedTime values MUST NOT include fractional seconds. (§4.1.2.5.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.6-4`](#rfc5280-4.1.2.6-4) Where it is non-empty, the subject field MUST contain an X.500 distinguished name (DN). (§4.1.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.6-6`](#rfc5280-4.1.2.6-6) (a) When the subject of the certificate is a CA, the subject field MUST be encoded in the same way as it is encoded in the issuer field (Section 4.1.2.4) in all certificates issued by the subject CA. (§4.1.2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.8-1`](#rfc5280-4.1.2.8-1) These fields MUST only appear if the version is 2 or 3 (Section 4.1.2.1). (§4.1.2.8) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.8-2`](#rfc5280-4.1.2.8-2) CAs conforming to this profile MUST NOT generate certificates with unique identifiers. (§4.1.2.8) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.1.2.9-1`](#rfc5280-4.1.2.9-1) This field MUST only appear if the version is 3 (Section 4.1.2.1). (§4.1.2.9) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2-1`](#rfc5280-4.2-1) A certificate-using system MUST reject the certificate if it encounters a critical extension it does not recognize or a critical extension that contains information that it cannot process. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2-2`](#rfc5280-4.2-2) A non-critical extension MAY be ignored if it is not recognized, but MUST be processed if it is recognized. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2-3`](#rfc5280-4.2-3) A certificate MUST NOT include more than one instance of a particular extension. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2-4`](#rfc5280-4.2-4) Conforming CAs MUST support key identifiers (Sections 4.2.1.1 and 4.2.1.2), basic constraints (Section 4.2.1.9), key usage (Section 4.2.1.3), and certificate policies (Section 4.2.1.4) extensions. (§4.2) | {gap}, no test | the pki CA writes key identifiers (through CreateCertificate), basic constraints and key usage, but no template it builds can carry certificate policies; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2-6`](#rfc5280-4.2-6) At a minimum, applications conforming to this profile MUST recognize the following extensions: key usage (Section 4.2.1.3), certificate policies (Section 4.2.1.4), subject alternative name (Section 4.2.1.6), basic constraints (Section 4.2.1.9), name constraints (Section 4.2.1.10), policy constraints (Section 4.2.1.11), extended key usage (Section 4.2.1.12), and inhibit anyPolicy (Section 4.2.1.14). (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-4.2.1.1-1`](#rfc5280-4.2.1.1-1) The keyIdentifier field of the authorityKeyIdentifier extension MUST be included in all certificates generated by conforming CAs to facilitate certification path construction. (§4.2.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.1-2`](#rfc5280-4.2.1.1-2) Conforming CAs MUST mark this extension as non-critical. (§4.2.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.2-1`](#rfc5280-4.2.1.2-1) To facilitate certification path construction, this extension MUST appear in all conforming CA certificates, that is, all certificates including the basic constraints extension (Section 4.2.1.9) where the value of cA is TRUE. (§4.2.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.2-2`](#rfc5280-4.2.1.2-2) In conforming CA certificates, the value of the subject key identifier MUST be the value placed in the key identifier field of the authority key identifier extension (Section 4.2.1.1) of certificates issued by the subject of this certificate. (§4.2.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.2-3`](#rfc5280-4.2.1.2-3) Conforming CAs MUST mark this extension as non-critical. (§4.2.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.3-3`](#rfc5280-4.2.1.3-3) If the keyUsage extension is present, then the subject public key MUST NOT be used to verify signatures on certificates or CRLs unless the corresponding keyCertSign or cRLSign bit is set. (§4.2.1.3) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.4-1`](#rfc5280-4.2.1.4-1) A certificate policy OID MUST NOT appear more than once in a certificate policies extension. (§4.2.1.4) | {gap}, no test | no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.4-2`](#rfc5280-4.2.1.4-2) If this extension is critical, the path validation software MUST be able to interpret this extension (including the optional qualifier), or MUST reject the certificate. (§4.2.1.4) | {gap}, no test | Go crypto/x509 parses the policy identifiers of a critical certificatePolicies extension and skips their qualifiers, so it neither interprets the qualifier nor rejects the certificate; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-4.2.1.4-3`](#rfc5280-4.2.1.4-3) When qualifiers are used with the special policy anyPolicy, they MUST be limited to the qualifiers identified in this section. (§4.2.1.4) | {gap}, no test | no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.4-4`](#rfc5280-4.2.1.4-4) Conforming CAs MUST NOT encode explicitText as VisibleString or BMPString. (§4.2.1.4) | {gap}, no test | no template the pki CA builds carries a certificatePolicies extension, so ze cannot issue one (RFC5280-4.2-4) and no producer holds this rule; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.6-4`](#rfc5280-4.2.1.6-4) When the subjectAltName extension contains an iPAddress, the address MUST be stored in the octet string in "network byte order", as specified in [RFC791]. (§4.2.1.6) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.6-5`](#rfc5280-4.2.1.6-5) For IP version 4, as specified in [RFC791], the octet string MUST contain exactly four octets. (§4.2.1.6) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.6-6`](#rfc5280-4.2.1.6-6) For IP version 6, as specified in [RFC2460], the octet string MUST contain exactly sixteen octets. (§4.2.1.6) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-4.2.1.6-8`](#rfc5280-4.2.1.6-8) The name MUST be in the "preferred name syntax", as specified by Section 3.5 of [RFC1034] and as modified by Section 2.1 of [RFC1123]. (§4.2.1.6) | {gap}, no test | Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.6-9`](#rfc5280-4.2.1.6-9) In addition, while the string " " is a legal domain name, subjectAltName extensions with a dNSName of " " MUST NOT be used. (§4.2.1.6) | {gap}, no test | Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.6-11`](#rfc5280-4.2.1.6-11) Unlike the subject field, conforming CAs MUST NOT issue certificates with subjectAltNames containing empty GeneralName fields. (§4.2.1.6) | {gap}, no test | Root.IssueLeafFor and GenerateWebCertWithNames put every host that does not parse as an IP address into DNSNames unchecked, and CreateCertificate checks only that it is an IA5String, so no layer enforces this; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-4.2.1.9-2`](#rfc5280-4.2.1.9-2) If the basic constraints extension is not present in a version 3 certificate, or the extension is present but the cA boolean is not asserted, then the certified public key MUST NOT be used to verify certificate signatures. (§4.2.1.9) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.10-1`](#rfc5280-4.2.1.10-1) Applications conforming to this profile MUST be able to process name constraints that are imposed on the directoryName name form and SHOULD be able to process name constraints that are imposed on the rfc822Name, uniformResourceIdentifier, dNSName, and iPAddress name forms. (§4.2.1.10) | {gap}, no test | Go crypto/x509 handles no directoryName constraint: parseNameConstraintsExtension marks the extension unhandled, so x509.Certificate.Verify rejects a certificate under one instead of processing it; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-4.2.1.10-2`](#rfc5280-4.2.1.10-2) If a name constraints extension that is marked as critical imposes constraints on a particular name form, and an instance of that name form appears in the subject field or subjectAltName extension of a subsequent certificate, then the application MUST either process the constraint or reject the certificate. (§4.2.1.10) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.10-3`](#rfc5280-4.2.1.10-3) However, if an application encounters a critical name constraints extension that specifies other values for minimum or maximum for a name form that appears in a subsequent certificate, the application MUST either process these fields or reject the certificate. (§4.2.1.10) | {gap}, no test | Go crypto/x509 reads the base of each GeneralSubtree and skips minimum and maximum, so it neither processes them nor rejects the certificate; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-4.2.1.10-4`](#rfc5280-4.2.1.10-4) If a constraint is applied to the uniformResourceIdentifier name form and a subsequent certificate includes a subjectAltName extension with a uniformResourceIdentifier that does not include an authority component with a host name specified as a fully qualified domain name (e.g., if the URI either does not include an authority component or includes an authority component in which the host name is specified as an IP address), then the application MUST reject the certificate. (§4.2.1.10) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.10-5`](#rfc5280-4.2.1.10-5) When constraints are imposed on the rfc822Name name form, but the certificate does not include a subject alternative name, the rfc822Name constraint MUST be applied to the attribute of type emailAddress in the subject distinguished name. (§4.2.1.10) | {gap}, no test | Go crypto/x509 applies rfc822Name constraints to subjectAltName entries only, never to an emailAddress attribute of the subject DN; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-4.2.1.11-1`](#rfc5280-4.2.1.11-1) Conforming applications MUST be able to process the requireExplicitPolicy field and SHOULD be able to process the inhibitPolicyMapping field. (§4.2.1.11) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.11-2`](#rfc5280-4.2.1.11-2) Applications that support the inhibitPolicyMapping field MUST also implement support for the policyMappings extension. (§4.2.1.11) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.11-3`](#rfc5280-4.2.1.11-3) If the policyConstraints extension is marked as critical and the inhibitPolicyMapping field is present, applications that do not implement support for the inhibitPolicyMapping field MUST reject the certificate. (§4.2.1.11) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-4.2.1.12-1`](#rfc5280-4.2.1.12-1) If the extension is present, then the certificate MUST only be used for one of the purposes indicated. (§4.2.1.12) | {gap}, no test | getRemoteCert and verifyEntryChain pass KeyUsages ExtKeyUsageAny to x509.Certificate.Verify and check no key usage bit of the leaf, so a peer certificate whose extensions name no purpose IKE fills is still used for IKE authentication; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-4.2.1.12-2`](#rfc5280-4.2.1.12-2) If a certificate contains both a key usage extension and an extended key usage extension, then both extensions MUST be processed independently and the certificate MUST only be used for a purpose consistent with both extensions. (§4.2.1.12) | {gap}, no test | getRemoteCert and verifyEntryChain pass KeyUsages ExtKeyUsageAny to x509.Certificate.Verify and check no key usage bit of the leaf, so a peer certificate whose extensions name no purpose IKE fills is still used for IKE authentication; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-5-1`](#rfc5280-5-1) Conforming applications that support CRLs are REQUIRED to process both version 1 and version 2 complete CRLs that provide revocation information for all certificates issued by one CA. (§5) | {gap}, no test | x509.ParseRevocationList refuses a CRL without a version field (a version 1 CRL), and parseCRLs and the pki crl leaf-list refuse the material with it; internal/core/eap/revocation.go::parseCRLs |
| [`RFC5280-5.1.2.4-1`](#rfc5280-5.1.2.4-1) Conforming applications MUST be able to process dates that are encoded in either UTCTime or GeneralizedTime. (§5.1.2.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseRevocationList; internal/core/eap/revocation.go::parseCRLs parses every configured CRL with x509.ParseRevocationList, which decodes this field; ze installs no value it reads |
| [`RFC5280-5.2-1`](#rfc5280-5.2-1) If a CRL contains a critical extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of certificates. (§5.2) | {gap}, no test | checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| [`RFC5280-5.2.3-1`](#rfc5280-5.2.3-1) CRL verifiers MUST be able to handle CRLNumber values up to 20 octets. (§5.2.3) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseRevocationList; internal/core/eap/revocation.go::parseCRLs parses every configured CRL with x509.ParseRevocationList, which decodes this field; ze installs no value it reads |
| [`RFC5280-5.2.5-1`](#rfc5280-5.2.5-1) However, implementations that do not support this extension MUST either treat the status of any certificate not listed on this CRL as unknown or locate another CRL that does not contain any unrecognized critical extensions. (§5.2.5) | {gap}, no test | checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| [`RFC5280-5.3-1`](#rfc5280-5.3-1) If a CRL contains a critical CRL entry extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of any certificates. (§5.3) | {gap}, no test | checkChainRevocation reads any list x509.ParseRevocationList accepts, and neither it nor Go looks at the critical flag of a CRL or CRL entry extension, so a list carrying an extension ze cannot process (a delta CRL indicator, an issuing distribution point, a certificate issuer entry) still decides certificate status; internal/core/eap/revocation.go::crlSet.currentListFrom |
| [`RFC5280-6.1-1`](#rfc5280-6.1-1) A conforming implementation MUST include an X.509 path processing procedure that is functionally equivalent to the external behavior of this algorithm. (§6.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-6.1-2`](#rfc5280-6.1-2) A certificate MUST NOT appear more than once in a prospective certification path. (§6.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-6.1.3-1`](#rfc5280-6.1.3-1) The certificate MUST satisfy each of the following: (§6.1.3) | {gap}, no test | x509.Certificate.Verify checks the signature, the validity period and issuer name chaining, and nothing checks condition (a)(3), the revocation status, of an IKE peer certificate: getRemoteCert runs no CRL or OCSP check (EAP-TLS does, in internal/core/eap/revocation.go::checkChainRevocation); internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-6.1.4-1`](#rfc5280-6.1.4-1) (If certificate i is a version 1 or version 2 certificate, then the application MUST either verify that certificate i is a CA certificate through out-of-band means or reject the certificate. (§6.1.4) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-6.3-1`](#rfc5280-6.3-1) Conforming implementations that support CRLs are not required to implement this algorithm, but they MUST be functionally equivalent to the external behavior resulting from this procedure when processing CRLs that are issued in conformance with this profile. (§6.3) | {gap}, no test | checkChainRevocation matches a list to the issuer, checks its signature and its nextUpdate, and implements none of the scope, reason, delta and critical extension steps of Section 6.3.3; internal/core/eap/revocation.go::checkChainRevocation |
| [`RFC5280-7.1-1`](#rfc5280-7.1-1) Conforming implementations MUST support UTF8String and PrintableString. (§7.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-7.1-2`](#rfc5280-7.1-2) Conforming implementations MUST use the LDAP StringPrep profile (including insignificant space handling), as specified in [RFC4518], as the basis for comparison of distinguished name attributes encoded in either PrintableString or UTF8String. (§7.1) | {gap}, no test | Go crypto/x509 chains names by comparing the DER bytes of issuer and subject, and performs no RFC 4518 string preparation or caseIgnoreMatch; internal/component/ike/engine/cert_payload.go::getRemoteCert |
| [`RFC5280-7.2-1`](#rfc5280-7.2-1) To accommodate internationalized domain names in the current structure, conforming implementations MUST convert internationalized domain names to the ASCII Compatible Encoding (ACE) format as specified in Section 4 of RFC 3490 before storage in the dNSName field. (§7.2) | {gap}, no test | Root.IssueLeafFor and GenerateWebCertWithNames convert no internationalized name: CreateCertificate refuses a dNSName that is not an IA5String, so an IDN host is refused rather than stored in ACE form; internal/component/pki/ca.go::Root.IssueLeafFor |
| [`RFC5280-7.2-3`](#rfc5280-7.2-3) When evaluating name constraints, conforming implementations MUST perform a case-insensitive exact match on a label-by-label basis. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 Certificate.Verify; internal/component/ike/engine/cert_payload.go::getRemoteCert and internal/component/pki/tls.go::verifyEntryChain hand the certificate, its intermediates and the configured trust anchor to x509.Certificate.Verify, which performs this check; ze installs no value it reads |
| [`RFC5280-7.2-4`](#rfc5280-7.2-4) Note: Implementations MUST allow for increased space requirements for IDNs. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-A.2-1`](#rfc5280-a.2-1) AuthorityKeyIdentifier ::= SEQUENCE { keyIdentifier [0] KeyIdentifier OPTIONAL, authorityCertIssuer [1] GeneralNames OPTIONAL, authorityCertSerialNumber [2] CertificateSerialNumber OPTIONAL } -- authorityCertIssuer and authorityCertSerialNumber MUST both -- be present or both be absent (§A.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-B-1`](#rfc5280-b-1) Conforming implementations MUST NOT encode strings that include either the at sign or underscore character as PrintableString. (§B) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-B-2`](#rfc5280-b-2) When DER encoding a named bit list, trailing zeros MUST be omitted. (§B) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 CreateCertificate and encoding/asn1; internal/component/pki/ca.go::generateRoot and internal/component/pki/ca.go::Root.IssueLeafFor and internal/core/selfcert/selfcert.go::GenerateWebCertWithNames fill an x509.Certificate template and CreateCertificate encodes it, deciding this field; no template value ze sets decides it |
| [`RFC5280-B-3`](#rfc5280-b-3) This specification mandates support for OIDs that have arc elements with values that are less than 2^28, that is, they MUST be between 0 and 268,435,455, inclusive. (§B) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-B-4`](#rfc5280-b-4) Implementations MUST also support OIDs where the length of the dotted decimal (see Section 1.4 of [RFC4512]) string representation can be up to 100 bytes (inclusive). (§B) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |
| [`RFC5280-B-5`](#rfc5280-b-5) Implementations MUST be able to handle OIDs with up to 20 elements (inclusive). (§B) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509 ParseCertificate; internal/component/ike/engine/cert_payload.go::getRemoteCert parses the certificates a peer sends with x509.ParseCertificate, which decides what it accepts; ze installs no value it reads |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5280-4.1.1.2-1`](#rfc5280-4.1.1.2-1)

This field MUST contain the same algorithm identifier as the signature field in the sequence tbsCertificate (Section 4.1.2.3). (§4.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.1.2-1, so no unit is bound to it.

### [`RFC5280-4.1.2.1-1`](#rfc5280-4.1.2.1-1)

When extensions are used, as expected in this profile, version MUST be 3 (value is 2). (§4.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.1-1, so no unit is bound to it.

### [`RFC5280-4.1.2.1-2`](#rfc5280-4.1.2.1-2)

At a minimum, conforming implementations MUST recognize version 3 certificates. (§4.1.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.1-2, so no unit is bound to it.

### [`RFC5280-4.1.2.2-1`](#rfc5280-4.1.2.2-1)

The serial number MUST be a positive integer assigned by the CA to each certificate. (§4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L251) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.2-2`](#rfc5280-4.1.2.2-2)

It MUST be unique for each certificate issued by a given CA (i.e., the issuer name and serial number identify a unique certificate). (§4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L253) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.2-3`](#rfc5280-4.1.2.2-3)

CAs MUST force the serialNumber to be a non-negative integer. (§4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L255) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.2-4`](#rfc5280-4.1.2.2-4)

Certificate users MUST be able to handle serialNumber values up to 20 octets. (§4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.2-4, so no unit is bound to it.

### [`RFC5280-4.1.2.2-5`](#rfc5280-4.1.2.2-5)

Conforming CAs MUST NOT use serialNumber values longer than 20 octets. (§4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIssueLeafDrawsAUniqueSerial`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L256) | unit/verify | mutant, verified |

### [`RFC5280-4.1.2.4-1`](#rfc5280-4.1.2.4-1)

The issuer field MUST contain a non-empty distinguished name (DN). (§4.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.4-1, so no unit is bound to it.

### [`RFC5280-4.1.2.4-2`](#rfc5280-4.1.2.4-2)

CAs conforming to this profile MUST use either the PrintableString or UTF8String encoding of DirectoryString, with two exceptions. (§4.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.4-2, so no unit is bound to it.

### [`RFC5280-4.1.2.4-3`](#rfc5280-4.1.2.4-3)

However, conforming implementations MUST be prepared to receive certificates with issuer names containing the set of attribute types defined below. (§4.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.4-3, so no unit is bound to it.

### [`RFC5280-4.1.2.4-4`](#rfc5280-4.1.2.4-4)

In addition, implementations of this specification MUST be prepared to receive the domainComponent attribute, as defined in [RFC4519]. (§4.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.4-4, so no unit is bound to it.

### [`RFC5280-4.1.2.4-5`](#rfc5280-4.1.2.4-5)

Certificate users MUST be prepared to process the issuer distinguished name and subject distinguished name (Section 4.1.2.6) fields to perform name chaining for certification path validation (Section 6). (§4.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.4-5, so no unit is bound to it.

### [`RFC5280-4.1.2.5-1`](#rfc5280-4.1.2.5-1)

CAs conforming to this profile MUST always encode certificate validity dates through the year 2049 as UTCTime; certificate validity dates in 2050 or later MUST be encoded as GeneralizedTime. (§4.1.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5-1, so no unit is bound to it.

### [`RFC5280-4.1.2.5-2`](#rfc5280-4.1.2.5-2)

Conforming applications MUST be able to process validity dates that are encoded in either UTCTime or GeneralizedTime. (§4.1.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5-2, so no unit is bound to it.

### [`RFC5280-4.1.2.5-3`](#rfc5280-4.1.2.5-3)

When the issuer will not be able to maintain status information until the notAfter date (including when the notAfter date is 99991231235959Z), the issuer MUST ensure that no valid certification path exists for the certificate after maintenance of status information is terminated. (§4.1.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5-3, so no unit is bound to it.

### [`RFC5280-4.1.2.5.1-1`](#rfc5280-4.1.2.5.1-1)

For the purposes of this profile, UTCTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5.1-1, so no unit is bound to it.

### [`RFC5280-4.1.2.5.1-2`](#rfc5280-4.1.2.5.1-2)

Where YY is greater than or equal to 50, the year SHALL be interpreted as 19YY; and (§4.1.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5.1-2, so no unit is bound to it.

### [`RFC5280-4.1.2.5.1-3`](#rfc5280-4.1.2.5.1-3)

Where YY is less than 50, the year SHALL be interpreted as 20YY. (§4.1.2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5.1-3, so no unit is bound to it.

### [`RFC5280-4.1.2.5.2-1`](#rfc5280-4.1.2.5.2-1)

For the purposes of this profile, GeneralizedTime values MUST be expressed in Greenwich Mean Time (Zulu) and MUST include seconds (i.e., times are YYYYMMDDHHMMSSZ), even where the number of seconds is zero. (§4.1.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5.2-1, so no unit is bound to it.

### [`RFC5280-4.1.2.5.2-2`](#rfc5280-4.1.2.5.2-2)

GeneralizedTime values MUST NOT include fractional seconds. (§4.1.2.5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.5.2-2, so no unit is bound to it.

### [`RFC5280-4.1.2.6-1`](#rfc5280-4.1.2.6-1)

If the subject is a CA (e.g., the basic constraints extension, as discussed in Section 4.2.1.9, is present and the value of cA is TRUE), then the subject field MUST be populated with a non-empty distinguished name matching the contents of the issuer field (Section 4.1.2.4) in all certificates issued by the subject CA. (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L76) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.6-2`](#rfc5280-4.1.2.6-2)

If the subject is a CRL issuer (e.g., the key usage extension, as discussed in Section 4.2.1.3, is present and the value of cRLSign is TRUE), then the subject field MUST be populated with a non-empty distinguished name matching the contents of the issuer field (Section 5.1.2.3) in all CRLs issued by the subject CRL issuer. (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L88) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.6-3`](#rfc5280-4.1.2.6-3)

If subject naming information is present only in the subjectAltName extension (e.g., a key bound only to an email address or URI), then the subject name MUST be an empty sequence and the subjectAltName extension MUST be critical. (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L33) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.6-4`](#rfc5280-4.1.2.6-4)

Where it is non-empty, the subject field MUST contain an X.500 distinguished name (DN). (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.6-4, so no unit is bound to it.

### [`RFC5280-4.1.2.6-5`](#rfc5280-4.1.2.6-5)

The DN MUST be unique for each subject entity certified by the one CA as defined by the issuer field. (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L238) | unit/verify | mutant, verified |
| positive | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L227) | unit/verify | revert, verified |

### [`RFC5280-4.1.2.6-6`](#rfc5280-4.1.2.6-6)

(a) When the subject of the certificate is a CA, the subject field MUST be encoded in the same way as it is encoded in the issuer field (Section 4.1.2.4) in all certificates issued by the subject CA. (§4.1.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.6-6, so no unit is bound to it.

### [`RFC5280-4.1.2.8-1`](#rfc5280-4.1.2.8-1)

These fields MUST only appear if the version is 2 or 3 (Section 4.1.2.1). (§4.1.2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.8-1, so no unit is bound to it.

### [`RFC5280-4.1.2.8-2`](#rfc5280-4.1.2.8-2)

CAs conforming to this profile MUST NOT generate certificates with unique identifiers. (§4.1.2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.8-2, so no unit is bound to it.

### [`RFC5280-4.1.2.9-1`](#rfc5280-4.1.2.9-1)

This field MUST only appear if the version is 3 (Section 4.1.2.1). (§4.1.2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.1.2.9-1, so no unit is bound to it.

### [`RFC5280-4.2-1`](#rfc5280-4.2-1)

A certificate-using system MUST reject the certificate if it encounters a critical extension it does not recognize or a critical extension that contains information that it cannot process. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2-1, so no unit is bound to it.

### [`RFC5280-4.2-2`](#rfc5280-4.2-2)

A non-critical extension MAY be ignored if it is not recognized, but MUST be processed if it is recognized. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2-2, so no unit is bound to it.

### [`RFC5280-4.2-3`](#rfc5280-4.2-3)

A certificate MUST NOT include more than one instance of a particular extension. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2-3, so no unit is bound to it.

### [`RFC5280-4.2-4`](#rfc5280-4.2-4)

Conforming CAs MUST support key identifiers (Sections 4.2.1.1 and 4.2.1.2), basic constraints (Section 4.2.1.9), key usage (Section 4.2.1.3), and certificate policies (Section 4.2.1.4) extensions. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2-4, so no unit is bound to it.

### [`RFC5280-4.2-5`](#rfc5280-4.2-5)

If the CA issues certificates with an empty sequence for the subject field, the CA MUST support the subject alternative name extension (Section 4.2.1.6). (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L37) | unit/verify | revert, verified |

### [`RFC5280-4.2-6`](#rfc5280-4.2-6)

At a minimum, applications conforming to this profile MUST recognize the following extensions: key usage (Section 4.2.1.3), certificate policies (Section 4.2.1.4), subject alternative name (Section 4.2.1.6), basic constraints (Section 4.2.1.9), name constraints (Section 4.2.1.10), policy constraints (Section 4.2.1.11), extended key usage (Section 4.2.1.12), and inhibit anyPolicy (Section 4.2.1.14). (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2-6, so no unit is bound to it.

### [`RFC5280-4.2.1.1-1`](#rfc5280-4.2.1.1-1)

The keyIdentifier field of the authorityKeyIdentifier extension MUST be included in all certificates generated by conforming CAs to facilitate certification path construction. (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.1-1, so no unit is bound to it.

### [`RFC5280-4.2.1.1-2`](#rfc5280-4.2.1.1-2)

Conforming CAs MUST mark this extension as non-critical. (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.1-2, so no unit is bound to it.

### [`RFC5280-4.2.1.2-1`](#rfc5280-4.2.1.2-1)

To facilitate certification path construction, this extension MUST appear in all conforming CA certificates, that is, all certificates including the basic constraints extension (Section 4.2.1.9) where the value of cA is TRUE. (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.2-1, so no unit is bound to it.

### [`RFC5280-4.2.1.2-2`](#rfc5280-4.2.1.2-2)

In conforming CA certificates, the value of the subject key identifier MUST be the value placed in the key identifier field of the authority key identifier extension (Section 4.2.1.1) of certificates issued by the subject of this certificate. (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.2-2, so no unit is bound to it.

### [`RFC5280-4.2.1.2-3`](#rfc5280-4.2.1.2-3)

Conforming CAs MUST mark this extension as non-critical. (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.2-3, so no unit is bound to it.

### [`RFC5280-4.2.1.3-1`](#rfc5280-4.2.1.3-1)

Conforming CAs MUST include this extension in certificates that contain public keys that are used to validate digital signatures on other public key certificates or CRLs. (§4.2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L97) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.3-2`](#rfc5280-4.2.1.3-2)

If the keyCertSign bit is asserted, then the cA bit in the basic constraints extension (Section 4.2.1.9) MUST also be asserted. (§4.2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L106) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.3-3`](#rfc5280-4.2.1.3-3)

If the keyUsage extension is present, then the subject public key MUST NOT be used to verify signatures on certificates or CRLs unless the corresponding keyCertSign or cRLSign bit is set. (§4.2.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.3-3, so no unit is bound to it.

### [`RFC5280-4.2.1.3-4`](#rfc5280-4.2.1.3-4)

When the keyUsage extension appears in a certificate, at least one of the bits MUST be set to 1. (§4.2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L115) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.4-1`](#rfc5280-4.2.1.4-1)

A certificate policy OID MUST NOT appear more than once in a certificate policies extension. (§4.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.4-1, so no unit is bound to it.

### [`RFC5280-4.2.1.4-2`](#rfc5280-4.2.1.4-2)

If this extension is critical, the path validation software MUST be able to interpret this extension (including the optional qualifier), or MUST reject the certificate. (§4.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.4-2, so no unit is bound to it.

### [`RFC5280-4.2.1.4-3`](#rfc5280-4.2.1.4-3)

When qualifiers are used with the special policy anyPolicy, they MUST be limited to the qualifiers identified in this section. (§4.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.4-3, so no unit is bound to it.

### [`RFC5280-4.2.1.4-4`](#rfc5280-4.2.1.4-4)

Conforming CAs MUST NOT encode explicitText as VisibleString or BMPString. (§4.2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.4-4, so no unit is bound to it.

### [`RFC5280-4.2.1.6-1`](#rfc5280-4.2.1.6-1)

Whenever such identities are to be bound into a certificate, the subject alternative name (or issuer alternative name) extension MUST be used; however, a DNS name MAY also be represented in the subject field using the domainComponent attribute as described in Section 4.1.2.4. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L183) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.6-2`](#rfc5280-4.2.1.6-2)

Because the subject alternative name is considered to be definitively bound to the public key, all parts of the subject alternative name MUST be verified by the CA. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L192) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.6-3`](#rfc5280-4.2.1.6-3)

Further, if the only subject identity included in the certificate is an alternative name form (e.g., an electronic mail address), then the subject distinguished name MUST be empty (an empty sequence), and the subjectAltName extension MUST be present. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical`](https://github.com/ze-software/ze/blob/main/internal/core/selfcert/rfc5280_alt_name_only_test.go#L35) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.6-4`](#rfc5280-4.2.1.6-4)

When the subjectAltName extension contains an iPAddress, the address MUST be stored in the octet string in "network byte order", as specified in [RFC791]. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-4, so no unit is bound to it.

### [`RFC5280-4.2.1.6-5`](#rfc5280-4.2.1.6-5)

For IP version 4, as specified in [RFC791], the octet string MUST contain exactly four octets. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-5, so no unit is bound to it.

### [`RFC5280-4.2.1.6-6`](#rfc5280-4.2.1.6-6)

For IP version 6, as specified in [RFC2460], the octet string MUST contain exactly sixteen octets. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-6, so no unit is bound to it.

### [`RFC5280-4.2.1.6-7`](#rfc5280-4.2.1.6-7)

When the subjectAltName extension contains a domain name system label, the domain name MUST be stored in the dNSName (an IA5String). (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L208) | unit/verify | revert, verified |
| positive | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L203) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.6-8`](#rfc5280-4.2.1.6-8)

The name MUST be in the "preferred name syntax", as specified by Section 3.5 of [RFC1034] and as modified by Section 2.1 of [RFC1123]. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-8, so no unit is bound to it.

### [`RFC5280-4.2.1.6-9`](#rfc5280-4.2.1.6-9)

In addition, while the string " " is a legal domain name, subjectAltName extensions with a dNSName of " " MUST NOT be used. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-9, so no unit is bound to it.

### [`RFC5280-4.2.1.6-10`](#rfc5280-4.2.1.6-10)

If the subjectAltName extension is present, the sequence MUST contain at least one entry. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestRFC5280LeafIdentitiesSitInTheSubjectAltName`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L216) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.6-11`](#rfc5280-4.2.1.6-11)

Unlike the subject field, conforming CAs MUST NOT issue certificates with subjectAltNames containing empty GeneralName fields. (§4.2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.6-11, so no unit is bound to it.

### [`RFC5280-4.2.1.9-1`](#rfc5280-4.2.1.9-1)

If the cA boolean is not asserted, then the keyCertSign bit in the key usage extension MUST NOT be asserted. (§4.2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L143) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.9-2`](#rfc5280-4.2.1.9-2)

If the basic constraints extension is not present in a version 3 certificate, or the extension is present but the cA boolean is not asserted, then the certified public key MUST NOT be used to verify certificate signatures. (§4.2.1.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.9-2, so no unit is bound to it.

### [`RFC5280-4.2.1.9-3`](#rfc5280-4.2.1.9-3)

Where it appears, the pathLenConstraint field MUST be greater than or equal to zero. (§4.2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L124) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.9-4`](#rfc5280-4.2.1.9-4)

Conforming CAs MUST include this extension in all CA certificates that contain public keys used to validate digital signatures on certificates and MUST mark the extension as critical in such certificates. (§4.2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280RootIsAMarkedCertificationAuthority`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L130) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.9-5`](#rfc5280-4.2.1.9-5)

CAs MUST NOT include the pathLenConstraint field unless the cA boolean is asserted and the key usage extension asserts the keyCertSign bit. (§4.2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC5280LeafIsAMarkedEndEntity`](https://github.com/ze-software/ze/blob/main/internal/component/pki/rfc5280_ca_profile_test.go#L157) | unit/verify | revert, verified |

### [`RFC5280-4.2.1.10-1`](#rfc5280-4.2.1.10-1)

Applications conforming to this profile MUST be able to process name constraints that are imposed on the directoryName name form and SHOULD be able to process name constraints that are imposed on the rfc822Name, uniformResourceIdentifier, dNSName, and iPAddress name forms. (§4.2.1.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.10-1, so no unit is bound to it.

### [`RFC5280-4.2.1.10-2`](#rfc5280-4.2.1.10-2)

If a name constraints extension that is marked as critical imposes constraints on a particular name form, and an instance of that name form appears in the subject field or subjectAltName extension of a subsequent certificate, then the application MUST either process the constraint or reject the certificate. (§4.2.1.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.10-2, so no unit is bound to it.

### [`RFC5280-4.2.1.10-3`](#rfc5280-4.2.1.10-3)

However, if an application encounters a critical name constraints extension that specifies other values for minimum or maximum for a name form that appears in a subsequent certificate, the application MUST either process these fields or reject the certificate. (§4.2.1.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.10-3, so no unit is bound to it.

### [`RFC5280-4.2.1.10-4`](#rfc5280-4.2.1.10-4)

If a constraint is applied to the uniformResourceIdentifier name form and a subsequent certificate includes a subjectAltName extension with a uniformResourceIdentifier that does not include an authority component with a host name specified as a fully qualified domain name (e.g., if the URI either does not include an authority component or includes an authority component in which the host name is specified as an IP address), then the application MUST reject the certificate. (§4.2.1.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.10-4, so no unit is bound to it.

### [`RFC5280-4.2.1.10-5`](#rfc5280-4.2.1.10-5)

When constraints are imposed on the rfc822Name name form, but the certificate does not include a subject alternative name, the rfc822Name constraint MUST be applied to the attribute of type emailAddress in the subject distinguished name. (§4.2.1.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.10-5, so no unit is bound to it.

### [`RFC5280-4.2.1.11-1`](#rfc5280-4.2.1.11-1)

Conforming applications MUST be able to process the requireExplicitPolicy field and SHOULD be able to process the inhibitPolicyMapping field. (§4.2.1.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.11-1, so no unit is bound to it.

### [`RFC5280-4.2.1.11-2`](#rfc5280-4.2.1.11-2)

Applications that support the inhibitPolicyMapping field MUST also implement support for the policyMappings extension. (§4.2.1.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.11-2, so no unit is bound to it.

### [`RFC5280-4.2.1.11-3`](#rfc5280-4.2.1.11-3)

If the policyConstraints extension is marked as critical and the inhibitPolicyMapping field is present, applications that do not implement support for the inhibitPolicyMapping field MUST reject the certificate. (§4.2.1.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.11-3, so no unit is bound to it.

### [`RFC5280-4.2.1.12-1`](#rfc5280-4.2.1.12-1)

If the extension is present, then the certificate MUST only be used for one of the purposes indicated. (§4.2.1.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.12-1, so no unit is bound to it.

### [`RFC5280-4.2.1.12-2`](#rfc5280-4.2.1.12-2)

If a certificate contains both a key usage extension and an extended key usage extension, then both extensions MUST be processed independently and the certificate MUST only be used for a purpose consistent with both extensions. (§4.2.1.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-4.2.1.12-2, so no unit is bound to it.

### [`RFC5280-4.2.2.1-1`](#rfc5280-4.2.2.1-1)

Conforming applications that support HTTP or FTP for accessing certificates MUST be able to accept individual DER encoded certificates and SHOULD be able to accept "certs-only" CMS messages. (§4.2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7296ReceivedHashAndURLFormatsAreAcceptedWhenConfigured`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_hash_url_accept_test.go#L69) | unit/verify | revert, verified |

### [`RFC5280-5-1`](#rfc5280-5-1)

Conforming applications that support CRLs are REQUIRED to process both version 1 and version 2 complete CRLs that provide revocation information for all certificates issued by one CA. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5-1, so no unit is bound to it.

### [`RFC5280-5.1.1.3-1`](#rfc5280-5.1.1.3-1)

Applications that perform CRL checking MUST support certification path validation when certificates and CRLs are digitally signed with the same CA private key. (§5.1.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L49) | unit/verify | revert, verified |

### [`RFC5280-5.1.2.4-1`](#rfc5280-5.1.2.4-1)

Conforming applications MUST be able to process dates that are encoded in either UTCTime or GeneralizedTime. (§5.1.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5.1.2.4-1, so no unit is bound to it.

### [`RFC5280-5.2-1`](#rfc5280-5.2-1)

If a CRL contains a critical extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of certificates. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5.2-1, so no unit is bound to it.

### [`RFC5280-5.2.3-1`](#rfc5280-5.2.3-1)

CRL verifiers MUST be able to handle CRLNumber values up to 20 octets. (§5.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5.2.3-1, so no unit is bound to it.

### [`RFC5280-5.2.5-1`](#rfc5280-5.2.5-1)

However, implementations that do not support this extension MUST either treat the status of any certificate not listed on this CRL as unknown or locate another CRL that does not contain any unrecognized critical extensions. (§5.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5.2.5-1, so no unit is bound to it.

### [`RFC5280-5.3-1`](#rfc5280-5.3-1)

If a CRL contains a critical CRL entry extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of any certificates. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-5.3-1, so no unit is bound to it.

### [`RFC5280-6.1-1`](#rfc5280-6.1-1)

A conforming implementation MUST include an X.509 path processing procedure that is functionally equivalent to the external behavior of this algorithm. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-6.1-1, so no unit is bound to it.

### [`RFC5280-6.1-2`](#rfc5280-6.1-2)

A certificate MUST NOT appear more than once in a prospective certification path. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-6.1-2, so no unit is bound to it.

### [`RFC5280-6.1.3-1`](#rfc5280-6.1.3-1)

The certificate MUST satisfy each of the following: (§6.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-6.1.3-1, so no unit is bound to it.

### [`RFC5280-6.1.4-1`](#rfc5280-6.1.4-1)

(If certificate i is a version 1 or version 2 certificate, then the application MUST either verify that certificate i is a CA certificate through out-of-band means or reject the certificate. (§6.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-6.1.4-1, so no unit is bound to it.

### [`RFC5280-6.3-1`](#rfc5280-6.3-1)

Conforming implementations that support CRLs are not required to implement this algorithm, but they MUST be functionally equivalent to the external behavior resulting from this procedure when processing CRLs that are issued in conformance with this profile. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-6.3-1, so no unit is bound to it.

### [`RFC5280-6.3.3-1`](#rfc5280-6.3.3-1)

The trust anchor for the certification path MUST be the same as the trust anchor used to validate the target certificate. (§6.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC5280CRLMustChainToTheCertificatesOwnAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5280_crl_path_test.go#L61) | unit/verify | revert, verified |

### [`RFC5280-7.1-1`](#rfc5280-7.1-1)

Conforming implementations MUST support UTF8String and PrintableString. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-7.1-1, so no unit is bound to it.

### [`RFC5280-7.1-2`](#rfc5280-7.1-2)

Conforming implementations MUST use the LDAP StringPrep profile (including insignificant space handling), as specified in [RFC4518], as the basis for comparison of distinguished name attributes encoded in either PrintableString or UTF8String. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-7.1-2, so no unit is bound to it.

### [`RFC5280-7.2-1`](#rfc5280-7.2-1)

To accommodate internationalized domain names in the current structure, conforming implementations MUST convert internationalized domain names to the ASCII Compatible Encoding (ACE) format as specified in Section 4 of RFC 3490 before storage in the dNSName field. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-7.2-1, so no unit is bound to it.

### [`RFC5280-7.2-2`](#rfc5280-7.2-2)

When comparing DNS names for equality, conforming implementations MUST perform a case-insensitive exact match on the entire DNS name. (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5280DNSNamesCompareCaseInsensitiveAndExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC5280DNSNamesCompareCaseInsensitiveAndExact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L38) | unit/verify | revert, verified |

### [`RFC5280-7.2-3`](#rfc5280-7.2-3)

When evaluating name constraints, conforming implementations MUST perform a case-insensitive exact match on a label-by-label basis. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-7.2-3, so no unit is bound to it.

### [`RFC5280-7.2-4`](#rfc5280-7.2-4)

Note: Implementations MUST allow for increased space requirements for IDNs. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-7.2-4, so no unit is bound to it.

### [`RFC5280-8-1`](#rfc5280-8-1)

Relying parties that choose to validate the server's certificate when obtaining information pointed to by an https URI in the cRLDistributionPoints, authorityInfoAccess, or subjectInfoAccess extensions MUST be prepared for the possibility that this will result in unbounded recursion. (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5280OCSPFetchDoesNotRecurseIntoTheResponderCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5280_relying_party_test.go#L98) | unit/verify | revert, verified |

### [`RFC5280-A.2-1`](#rfc5280-a.2-1)

AuthorityKeyIdentifier ::= SEQUENCE { keyIdentifier [0] KeyIdentifier OPTIONAL, authorityCertIssuer [1] GeneralNames OPTIONAL, authorityCertSerialNumber [2] CertificateSerialNumber OPTIONAL } -- authorityCertIssuer and authorityCertSerialNumber MUST both -- be present or both be absent (§A.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-A.2-1, so no unit is bound to it.

### [`RFC5280-B-1`](#rfc5280-b-1)

Conforming implementations MUST NOT encode strings that include either the at sign or underscore character as PrintableString. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-B-1, so no unit is bound to it.

### [`RFC5280-B-2`](#rfc5280-b-2)

When DER encoding a named bit list, trailing zeros MUST be omitted. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-B-2, so no unit is bound to it.

### [`RFC5280-B-3`](#rfc5280-b-3)

This specification mandates support for OIDs that have arc elements with values that are less than 2^28, that is, they MUST be between 0 and 268,435,455, inclusive. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-B-3, so no unit is bound to it.

### [`RFC5280-B-4`](#rfc5280-b-4)

Implementations MUST also support OIDs where the length of the dotted decimal (see Section 1.4 of [RFC4512]) string representation can be up to 100 bytes (inclusive). (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-B-4, so no unit is bound to it.

### [`RFC5280-B-5`](#rfc5280-b-5)

Implementations MUST be able to handle OIDs with up to 20 elements (inclusive). (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5280-B-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5280.txt |
| Source fingerprint | b2df89806526069e |
| Record | rfc/extraction/rfc5280.json |
| Mapped sentences | 102 |
| Declined as scope | 178 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 2 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.2` | not stated | 1 | walked | not stated |
| `4.1.1.3` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.1.2.1` | not stated | 2 | walked | not stated |
| `4.1.2.2` | not stated | 5 | walked | not stated |
| `4.1.2.3` | not stated | 1 | walked | not stated |
| `4.1.2.4` | not stated | 6 | walked | not stated |
| `4.1.2.5` | not stated | 3 | walked | not stated |
| `4.1.2.5.1` | not stated | 4 | walked | not stated |
| `4.1.2.5.2` | not stated | 2 | walked | not stated |
| `4.1.2.6` | not stated | 11 | walked | not stated |
| `4.1.2.7` | not stated | 0 | walked | not stated |
| `4.1.2.8` | not stated | 3 | walked | not stated |
| `4.1.2.9` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 6 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.1.1` | not stated | 2 | walked | not stated |
| `4.2.1.2` | not stated | 3 | walked | not stated |
| `4.2.1.3` | not stated | 4 | walked | not stated |
| `4.2.1.4` | not stated | 4 | walked | not stated |
| `4.2.1.5` | not stated | 1 | walked | not stated |
| `4.2.1.6` | not stated | 19 | walked | not stated |
| `4.2.1.7` | not stated | 1 | walked | not stated |
| `4.2.1.8` | not stated | 1 | walked | not stated |
| `4.2.1.9` | not stated | 5 | walked | not stated |
| `4.2.1.10` | not stated | 19 | walked | not stated |
| `4.2.1.11` | not stated | 6 | walked | not stated |
| `4.2.1.12` | not stated | 4 | walked | not stated |
| `4.2.1.13` | not stated | 12 | walked | not stated |
| `4.2.1.14` | not stated | 1 | walked | not stated |
| `4.2.1.15` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.1` | not stated | 4 | walked | not stated |
| `4.2.2.2` | not stated | 6 | walked | not stated |
| `5` | not stated | 3 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.1.1` | not stated | 0 | walked | not stated |
| `5.1.1.2` | not stated | 1 | walked | not stated |
| `5.1.1.3` | not stated | 1 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.2.1` | not stated | 1 | walked | not stated |
| `5.1.2.2` | not stated | 1 | walked | not stated |
| `5.1.2.3` | not stated | 2 | walked | not stated |
| `5.1.2.4` | not stated | 5 | walked | not stated |
| `5.1.2.5` | not stated | 6 | walked | not stated |
| `5.1.2.6` | not stated | 2 | walked | not stated |
| `5.1.2.7` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 2 | walked | not stated |
| `5.2.1` | not stated | 1 | walked | not stated |
| `5.2.2` | not stated | 1 | walked | not stated |
| `5.2.3` | not stated | 8 | walked | not stated |
| `5.2.4` | not stated | 11 | walked | not stated |
| `5.2.5` | not stated | 15 | walked | not stated |
| `5.2.6` | not stated | 4 | walked | not stated |
| `5.2.7` | not stated | 7 | walked | not stated |
| `5.3` | not stated | 1 | walked | not stated |
| `5.3.1` | not stated | 0 | walked | not stated |
| `5.3.2` | not stated | 1 | walked | not stated |
| `5.3.3` | not stated | 3 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 3 | walked | not stated |
| `6.1.1` | not stated | 1 | walked | not stated |
| `6.1.2` | not stated | 1 | walked | not stated |
| `6.1.3` | not stated | 1 | walked | not stated |
| `6.1.4` | not stated | 1 | walked | not stated |
| `6.1.5` | not stated | 0 | walked | not stated |
| `6.1.6` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 1 | walked | not stated |
| `6.3.1` | not stated | 0 | walked | not stated |
| `6.3.2` | not stated | 0 | walked | not stated |
| `6.3.3` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 6 | walked | not stated |
| `7.2` | not stated | 9 | walked | not stated |
| `7.3` | not stated | 3 | walked | not stated |
| `7.4` | not stated | 7 | walked | not stated |
| `7.5` | not stated | 2 | walked | not stated |
| `8` | not stated | 6 | walked | not stated |
| `9` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `10` | acknowledgments | 0 | skipped (acknowledgements) | acknowledgments |
| `11` | references | 0 | skipped (references) | references |
| `11.1` | normative references | 0 | skipped (references) | normative references |
| `11.2` | informative references | 0 | skipped (references) | informative references |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 3 | walked | not stated |
| `A.2` | not stated | 3 | walked | not stated |
| `B` | not stated | 12 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |
| `C.1` | not stated | 0 | walked | not stated |
| `C.2` | not stated | 0 | walked | not stated |
| `C.3` | not stated | 0 | walked | not stated |
| `C.4` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 6 rule that an implementation derives the results of the algorithms without following their steps | Implementations are REQUIRED to derive the same results but are not required to use the specified procedures. |
| `1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the encoding of public key material and signatures belongs to RFC 3279, RFC 4055 and RFC 4491, which the sentence names | However, conforming implementations that use the algorithms identified in [RFC3279], [RFC4055], and [RFC4491] MUST identify and encode the public key materials and digital signatures as described in those specifications. |
| `3.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | An entry MUST NOT be removed from the CRL until it appears on one regularly scheduled CRL issued beyond the revoked certificate's validity period. |
| `4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ASN.1 comments of the TBSCertificate definition restate the version rules of Sections 4.1.2.8 and 4.1.2.9 | TBSCertificate ::= SEQUENCE { version [0] EXPLICIT Version DEFAULT v1, serialNumber CertificateSerialNumber, signature AlgorithmIdentifier, issuer Name, validity Validity, subject Name, subjectPublicKeyInfo SubjectPublicKeyInfo, issuerUniqueID [1] IMPLICIT UniqueIdentifier OPTIONAL, -- If present, version MUST be v2 or v3 subjectUniqueID [2] IMPLICIT UniqueIdentifier OPTIONAL, -- If present, version MUST be v2 or v3 extensions [3] EXPLICIT Extensions OPTIONAL -- If present, version MUST be v3 } |
| `4.1.2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | states from the tbsCertificate side the same equality of the two algorithm identifiers | This field MUST contain the same algorithm identifier as the signatureAlgorithm field in the sequence Certificate (Section 4.1.1.2). |
| `4.1.2.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | lists the attribute types the previous sentence obliges an implementation to receive | Implementations of this specification MUST be prepared to receive the following standard attribute types in issuer and subject (Section 4.1.2.6) names: |
| `4.1.2.5.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | introduces the two interpretation rules that follow it | Conforming systems MUST interpret the year field (YY) as follows: |
| `4.1.2.6:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates for the subject field the attribute types an implementation must receive in the issuer field | Implementations of this specification MUST be prepared to receive subject names containing the attribute types required for the issuer field. |
| `4.1.2.6:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates for subject attributes the PrintableString or UTF8String rule of Section 4.1.2.4 | When encoding attribute values of type DirectoryString, conforming CAs MUST use PrintableString or UTF8String encoding, with the following exceptions: |
| `4.1.2.6:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the consequence the previous sentence draws for TeletexString, BMPString and UniversalString issuers | Thus, if the subject CA encodes attributes in the issuer fields of certificates that it issues using the TeletexString, BMPString, or UniversalString encodings, then the subject field of certificates issued to that CA MUST use the same encoding. |
| `4.1.2.6:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | (b) When the subject of the certificate is a CRL issuer, the subject field MUST be encoded in the same way as it is encoded in the issuer field (Section 5.1.2.3) in all CRLs issued by the subject CRL issuer. |
| `4.1.2.6:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies electronic mail identities. Root.IssueLeafFor and GenerateWebCertWithNames put hosts into dNSName and iPAddress entries only and never write an email address (internal/component/pki/ca.go) | Conforming implementations generating new certificates with electronic mail addresses MUST use the rfc822Name in the subject alternative name extension (Section 4.2.1.6) to describe such identities. |
| `4.1.2.8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the negative form of the previous sentence | These fields MUST NOT appear if the version is 1. |
| `4.2.1.5:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the policy mappings extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Policies MUST NOT be mapped either to or from the special value anyPolicy (Section 4.2.1.4). |
| `4.2.1.6:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 4.1.2.6 rule that a certificate with an empty subject carries a critical subjectAltName | If the subject field contains an empty sequence, then the issuing CA MUST include a subjectAltName extension that is marked as critical. |
| `4.2.1.6:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies electronic mail identities. Root.IssueLeafFor and GenerateWebCertWithNames put hosts into dNSName and iPAddress entries only and never write an email address (internal/component/pki/ca.go) | When the subjectAltName extension contains an Internet mail address, the address MUST be stored in the rfc822Name. |
| `4.2.1.6:12` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies electronic mail identities. Root.IssueLeafFor and GenerateWebCertWithNames put hosts into dNSName and iPAddress entries only and never write an email address (internal/component/pki/ca.go) | Finally, the use of the DNS representation for Internet mail addresses (subscriber.example.com instead of subscriber@example.com) MUST NOT be used; such identities are to be encoded as rfc822Name. |
| `4.2.1.6:13` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | When the subjectAltName extension contains a URI, the name MUST be stored in the uniformResourceIdentifier (an IA5String). |
| `4.2.1.6:14` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | The name MUST NOT be a relative URI, and it MUST follow the URI syntax and encoding rules specified in [RFC3986]. |
| `4.2.1.6:15` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | The name MUST include both a scheme (e.g., "http" or "ftp") and a scheme-specific-part. |
| `4.2.1.6:16` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | URIs that include an authority ([RFC3986], Section 3.2) MUST include a fully qualified domain name or IP address as the host. |
| `4.2.1.6:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.1.2.6 uniqueness rule for subject names word for word | The DN MUST be unique for each subject entity certified by the one CA as defined by the issuer field. |
| `4.2.1.7:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the issuer alternative name extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Issuer alternative name MUST be encoded as in 4.2.1.6. |
| `4.2.1.8:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject directory attributes extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as non-critical. |
| `4.2.1.10:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The name constraints extension, which MUST be used only in a CA certificate, indicates a name space within which all subject names in subsequent certificates in a certification path MUST be located. |
| `4.2.1.10:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as critical and SHOULD NOT impose name constraints on the x400Address, ediPartyName, or registeredID name forms. |
| `4.2.1.10:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST NOT issue certificates where name constraints is an empty sequence. |
| `4.2.1.10:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | That is, either the permittedSubtrees field or the excludedSubtrees MUST be present. |
| `4.2.1.10:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Within this profile, the minimum and maximum fields are not used with any name forms, thus, the minimum MUST be zero, and maximum MUST be absent. |
| `4.2.1.10:9` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The constraint MUST be specified as a fully qualified domain name and MAY specify a host or a domain. |
| `4.2.1.10:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | states where the directoryName constraint processing RFC5280-4.2.1.10 requires is applied | Restrictions of the form directoryName MUST be applied to the subject field in the certificate (when the certificate includes a non-empty subject field) and to any names of type directoryName in the subjectAltName extension. |
| `4.2.1.10:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | x400Address is a name form the constraint must be processed for or the certificate rejected, which the earlier sentence requires | Restrictions of the form x400Address MUST be applied to any names of type x400Address in the subjectAltName extension. |
| `4.2.1.10:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | states how directoryName constraint processing compares names | When applying restrictions of the form directoryName, an implementation MUST compare DN attributes. |
| `4.2.1.10:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | states how directoryName constraint processing compares names | At a minimum, implementations MUST perform the DN comparison rules specified in Section 7.1. |
| `4.2.1.10:16` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | This implies name restrictions MUST be stated identically to the encoding used in the subject field or subjectAltName extension. |
| `4.2.1.10:17` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The syntax of iPAddress MUST be as described in Section 4.2.1.6 with the following additions specifically for name constraints. |
| `4.2.1.10:18` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | For IPv4 addresses, the iPAddress field of GeneralName MUST contain eight (8) octets, encoded in the style of RFC 4632 (CIDR) to represent an address range [RFC4632]. |
| `4.2.1.10:19` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | For IPv6 addresses, the iPAddress field MUST contain 32 octets similarly encoded. |
| `4.2.1.11:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the policy constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST NOT issue certificates where policy constraints is an empty sequence. |
| `4.2.1.11:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the policy constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | That is, either the inhibitPolicyMapping field or the requireExplicitPolicy field MUST be present. |
| `4.2.1.11:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the policy constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as critical. id-ce-policyConstraints OBJECT IDENTIFIER ::= { id-ce 36 } |
| `4.2.1.12:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds whoever defines a key purpose: the OID is assigned by IANA or ITU-T. Ze defines none; Root.IssueLeafFor names id-kp-serverAuth through x509.ExtKeyUsageServerAuth (internal/component/pki/ca.go) | Object identifiers used to identify key purposes MUST be assigned in accordance with IANA or ITU-T Recommendation X.660 [X.660]. |
| `4.2.1.12:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the consequence of the previous sentence when no purpose is consistent with both extensions | If there is no purpose consistent with both extensions, then the certificate MUST NOT be used for any purpose. |
| `4.2.1.13:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | While each of these fields is optional, a DistributionPoint MUST NOT consist of only the reasons field; either distributionPoint or cRLIssuer MUST be present. |
| `4.2.1.13:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | If the certificate issuer is not the CRL issuer, then the cRLIssuer field MUST be present and contain the Name of the CRL issuer. |
| `4.2.1.13:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | If the certificate issuer is also the CRL issuer, then conforming CAs MUST omit the cRLIssuer field and MUST include the distributionPoint field. |
| `4.2.1.13:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a relying party that dereferences CRL distribution points. Ze reads only the CRLs the operator configures (internal/core/eap/revocation.go::parseCRLs) and fetches none | If the DistributionPointName contains a general name of type URI, the following semantics MUST be assumed: the URI is a pointer to the current CRL for the associated reasons and will be issued by the associated cRLIssuer. |
| `4.2.1.13:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | When the HTTP or FTP URI scheme is used, the URI MUST point to a single DER encoded CRL as specified in [RFC2585]. |
| `4.2.1.13:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | When the LDAP URI scheme [RFC4516] is used, the URI MUST include a <dn> field containing the distinguished name of the entry holding the CRL, MUST include a single <attrdesc> that contains an appropriate attribute description for the attribute that holds the CRL [RFC4523], and SHOULD include a <host> (e.g., <ldap://ldap.example.com/cn=example%20CA,dc=example,dc=com? certificateRevocationList;binary>). |
| `4.2.1.13:7` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The DistributionPointName MUST NOT use the nameRelativeToCRLIssuer alternative when cRLIssuer contains more than one distinguished name. |
| `4.2.1.13:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the DistributionPoint omits the reasons field, the CRL MUST include revocation information for all reasons. |
| `4.2.1.13:9` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | When a conforming CA includes a cRLDistributionPoints extension in a certificate, it MUST include at least one DistributionPoint that points to a CRL that covers the certificate for all reasons. |
| `4.2.1.13:10` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | If present, the cRLIssuer MUST only contain the distinguished name (DN) from the issuer field of the CRL to which the DistributionPoint is pointing. |
| `4.2.1.13:11` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The encoding of the name in the cRLIssuer field MUST be exactly the same as the encoding in issuer field of the CRL. |
| `4.2.1.13:12` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | If the cRLIssuer field is included and the DN in that field does not correspond to an X.500 or LDAP directory entry where CRL is located, then conforming CAs MUST include the distributionPoint field. |
| `4.2.1.14:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the inhibit anyPolicy extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as critical. |
| `4.2.1.15:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the freshest CRL extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The extension MUST be marked as non-critical by conforming CAs. |
| `4.2.2.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the authority information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as non-critical. |
| `4.2.2.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the authority information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The LDAP URI [RFC4516] MUST include a <dn> field containing the distinguished name of the entry holding the certificates, MUST include an <attributes> field that lists appropriate attribute descriptions for the attributes that hold the DER encoded certificates or cross-certificate pairs [RFC4523], and SHOULD include a <host> (e.g., <ldap://ldap.example.com/cn=CA, dc=example,dc=com?cACertificate;binary,crossCertificatePair;binary>). |
| `4.2.2.1:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the authority information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Where the information is available via HTTP or FTP, accessLocation MUST be a uniformResourceIdentifier and the URI MUST point to either a single DER encoded certificate as specified in [RFC2585] or a collection of certificates in a BER or DER encoded "certs-only" CMS message as specified in [RFC2797]. |
| `4.2.2.2:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Conforming CAs MUST mark this extension as non-critical. |
| `4.2.2.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | The LDAP URI [RFC4516] MUST include a <dn> field containing the distinguished name of the entry holding the certificates, MUST include an <attributes> field that lists appropriate attribute descriptions for the attributes that hold the DER encoded certificates or cross-certificate pairs [RFC4523], and SHOULD include a <host> (e.g., <ldap://ldap.example.com/cn=CA, dc=example,dc=com?cACertificate;binary,crossCertificatePair;binary>). |
| `4.2.2.2:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Where the information is available via HTTP or FTP, accessLocation MUST be a uniformResourceIdentifier and the URI MUST point to either a single DER encoded certificate as specified in [RFC2585] or a collection of certificates in a BER or DER encoded "certs-only" CMS message as specified in [RFC2797]. |
| `4.2.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.2.2.1 rule for applications that fetch certificates over HTTP or FTP | Conforming applications that support HTTP or FTP for accessing certificates MUST be able to accept individual DER encoded certificates and SHOULD be able to accept "certs-only" CMS messages. |
| `4.2.2.2:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Where the timestamping services are available via HTTP or FTP, accessLocation MUST be a uniformResourceIdentifier. |
| `4.2.2.2:6` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the subject information access extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | Where the timestamping services are available via electronic mail, accessLocation MUST be an rfc822Name. |
| `5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The scope of a delta CRL MUST be the same as the base CRL that it references. |
| `5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When CRLs are issued, the CRLs MUST be version 2 CRLs, include the date by which the next CRL will be issued in the nextUpdate field (Section 5.1.2.5), include the CRL number extension (Section 5.2.3), and include the authority key identifier extension (Section 5.2.1). |
| `5.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | TBSCertList ::= SEQUENCE { version Version OPTIONAL, -- if present, MUST be v2 signature AlgorithmIdentifier, issuer Name, thisUpdate Time, nextUpdate Time OPTIONAL, revokedCertificates SEQUENCE OF SEQUENCE { userCertificate CertificateSerialNumber, revocationDate Time, crlEntryExtensions Extensions OPTIONAL -- if present, version MUST be v2 } OPTIONAL, crlExtensions [0] EXPLICIT Extensions OPTIONAL -- if present, version MUST be v2 } |
| `5.1.1.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | This field MUST contain the same algorithm identifier as the signature field in the sequence tbsCertList (Section 5.1.2.2). |
| `5.1.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When extensions are used, as required by this profile, this field MUST be present and MUST specify version 2 (the integer value is 1). |
| `5.1.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | This field MUST contain the same algorithm identifier as the signatureAlgorithm field in the sequence CertificateList (Section 5.1.1.2). |
| `5.1.2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The issuer field MUST contain a non-empty X.500 distinguished name (DN). |
| `5.1.2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The issuer field is defined as the X.501 type Name, and MUST follow the encoding rules for the issuer name field in the certificate (Section 4.1.2.4). |
| `5.1.2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers conforming to this profile MUST encode thisUpdate as UTCTime for dates through the year 2049. |
| `5.1.2.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers conforming to this profile MUST encode thisUpdate as GeneralizedTime for dates in the year 2050 or later. |
| `5.1.2.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the UTCTime half of the date processing the earlier sentence requires | Where encoded as UTCTime, thisUpdate MUST be specified and interpreted as defined in Section 4.1.2.5.1. |
| `5.1.2.4:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the GeneralizedTime half of the date processing the earlier sentence requires | Where encoded as GeneralizedTime, thisUpdate MUST be specified and interpreted as defined in Section 4.1.2.5.2. |
| `5.1.2.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST include the nextUpdate field in all CRLs. |
| `5.1.2.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers conforming to this profile MUST encode nextUpdate as UTCTime for dates through the year 2049. |
| `5.1.2.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers conforming to this profile MUST encode nextUpdate as GeneralizedTime for dates in the year 2050 or later. |
| `5.1.2.5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats for nextUpdate the Section 5.1.2.4 date processing rule | Conforming applications MUST be able to process dates that are encoded in either UTCTime or GeneralizedTime. |
| `5.1.2.5:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats for nextUpdate the Section 5.1.2.4 date processing rule | Where encoded as UTCTime, nextUpdate MUST be specified and interpreted as defined in Section 4.1.2.5.1. |
| `5.1.2.5:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats for nextUpdate the Section 5.1.2.4 date processing rule | Where encoded as GeneralizedTime, nextUpdate MUST be specified and interpreted as defined in Section 4.1.2.5.2. |
| `5.1.2.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When there are no revoked certificates, the revoked certificates list MUST be absent. |
| `5.1.2.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The time for revocationDate MUST be expressed as described in Section 5.1.2.4. |
| `5.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers are REQUIRED to include the authority key identifier (Section 5.2.1) and the CRL number (Section 5.2.3) extensions in all CRLs issued. |
| `5.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST use the key identifier method, and MUST include this extension in all CRLs issued. |
| `5.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Whenever such identities are used, the issuer alternative name extension MUST be used; however, a DNS name MAY be represented in the issuer field using the domainComponent attribute as described in Section 4.1.2.4. |
| `5.2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers conforming to this profile MUST include this extension in all CRLs and MUST mark this extension as non-critical. |
| `5.2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If a CRL issuer generates delta CRLs in addition to complete CRLs for a given scope, the complete CRLs and delta CRLs MUST share one numbering sequence. |
| `5.2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If a delta CRL and a complete CRL that cover the same scope are issued at the same time, they MUST have the same CRL number and provide the same revocation information. |
| `5.2.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | That is, the combination of the delta CRL and an acceptable complete CRL MUST provide the same revocation information as the simultaneously issued complete CRL. |
| `5.2.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If a CRL issuer generates two CRLs (two complete CRLs, two delta CRLs, or a complete CRL and a delta CRL) for the same scope at different times, the two CRLs MUST NOT have the same CRL number. |
| `5.2.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | That is, if the this update field (Section 5.1.2.4) in the two CRLs are not identical, the CRL numbers MUST be different. |
| `5.2.3:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST NOT use CRLNumber values longer than 20 octets. |
| `5.2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | A conforming CRL issuer MUST publish the referenced base CRL as a complete CRL. |
| `5.2.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When a conforming CRL issuer generates a delta CRL, the delta CRL MUST include a critical delta CRL indicator extension. |
| `5.2.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When a delta CRL is issued, it MUST cover the same set of reasons and the same set of certificates that were covered by the base CRL it references. |
| `5.2.4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | That is, the scope of the delta CRL MUST be the same as the scope of the complete CRL referenced as the base. |
| `5.2.4:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The referenced base CRL and the delta CRL MUST omit the issuing distribution point extension or contain identical issuing distribution point extensions. |
| `5.2.4:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Further, the CRL issuer MUST use the same private key to sign the delta CRL and any complete CRL that it can be used to update. |
| `5.2.4:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers MUST ensure that the combination of a delta CRL and any appropriate complete CRL accurately reflects the current revocation status. |
| `5.2.4:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The CRL issuer MUST include an entry in the delta CRL for each certificate within the scope of the delta CRL whose status has changed since the generation of the referenced base CRL: |
| `5.2.4:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the certificate was placed on hold in any CRL issued after the base but before this delta CRL and then released from hold, it MUST be listed on the delta CRL with revocation reason removeFromCRL. |
| `5.2.4:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | In this case, the revocation notice MUST be included in all subsequent delta CRLs until the revocation notice is included on at least one explicitly issued complete CRL for this scope. |
| `5.2.4:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a relying party that supports delta CRLs. Ze combines no base and delta CRL: checkChainRevocation reads each configured list on its own (internal/core/eap/revocation.go::parseCRLs) | An application that supports delta CRLs MUST be able to construct a current complete CRL by combining a previously issued complete CRL and the most current delta CRL. |
| `5.2.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The reason codes associated with a distribution point MUST be specified in onlySomeReasons. |
| `5.2.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If onlySomeReasons does not appear, the distribution point MUST contain revocations for all reason codes. |
| `5.2.5:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If a CRL includes an issuingDistributionPoint extension with onlySomeReasons present, then every certificate in the scope of the CRL that is revoked MUST be assigned a revocation reason other than unspecified. |
| `5.2.5:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the distributionPoint field is present, then it MUST include at least one of names from the corresponding distributionPoint field of the cRLDistributionPoints extension of every certificate that is within the scope of this CRL. |
| `5.2.5:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The identical encoding MUST be used in the distributionPoint fields of the certificate and the CRL. |
| `5.2.5:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the distributionPoint field is absent, the CRL MUST contain entries for all revoked unexpired certificates issued by the CRL issuer, if any, within the scope of the CRL. |
| `5.2.5:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the scope of the CRL only includes certificates issued by the CRL issuer, then the indirectCRL boolean MUST be set to FALSE. |
| `5.2.5:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Otherwise, if the scope of the CRL includes certificates issued by one or more authorities other than the CRL issuer, the indirectCRL boolean MUST be set to TRUE. |
| `5.2.5:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the scope of the CRL only includes end entity public key certificates, then onlyContainsUserCerts MUST be set to TRUE. |
| `5.2.5:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If the scope of the CRL only includes CA certificates, then onlyContainsCACerts MUST be set to TRUE. |
| `5.2.5:12` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | If either onlyContainsUserCerts or onlyContainsCACerts is set to TRUE, then the scope of the CRL MUST NOT include any version 1 or version 2 certificates. |
| `5.2.5:13` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRLs issuers MUST set the onlyContainsAttributeCerts boolean to FALSE. |
| `5.2.5:14` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRLs issuers MUST NOT issue CRLs where the DER encoding of the issuing distribution point extension is an empty sequence. |
| `5.2.5:15` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | That is, if onlyContainsUserCerts, onlyContainsCACerts, indirectCRL, and onlyContainsAttributeCerts are all FALSE, then either the distributionPoint field or the onlySomeReasons field MUST be present. |
| `5.2.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST mark this extension as non-critical. |
| `5.2.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | This extension MUST NOT appear in delta CRLs. |
| `5.2.6:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The reasons and cRLIssuer fields MUST be omitted from this CRL extension. |
| `5.2.6:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The scope of these delta CRLs MUST be the same as the scope of this complete CRL. |
| `5.2.7:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | This CRL extension MUST be marked as non-critical. |
| `5.2.7:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | When present in a CRL, this extension MUST include at least one AccessDescription specifying id-ad-caIssuers as the accessMethod. |
| `5.2.7:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Access method types other than id-ad-caIssuers MUST NOT be included. |
| `5.2.7:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Where the information is available via HTTP or FTP, accessLocation MUST be a uniformResourceIdentifier and the URI MUST point to either a single DER encoded certificate as specified in [RFC2585] or a collection of certificates in a BER or DER encoded "certs-only" CMS message as specified in [RFC2797]. |
| `5.2.7:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.2.2.1 rule for applications that fetch certificates over HTTP or FTP | Conforming applications that support HTTP or FTP for accessing certificates MUST be able to accept individual DER encoded certificates and SHOULD be able to accept "certs-only" CMS messages. |
| `5.2.7:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds an implementation that follows a directoryName accessLocation. Ze follows no access location of that form (internal/core/eap/revocation.go::parseCRLs reads configured lists only) | Thus, implementations that support the directoryName form of accessLocation MUST be prepared to find the needed certificate in any of these three attributes. |
| `5.2.7:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The LDAP URI [RFC4516] MUST include a <dn> field containing the distinguished name of the entry holding the certificates, MUST include an <attributes> field that lists appropriate attribute descriptions for the attributes that hold the DER encoded certificates or cross-certificate pairs [RFC4523], and SHOULD include a <host> (e.g., <ldap://ldap.example.com/cn=CA, dc=example,dc=com?cACertificate;binary,crossCertificatePair;binary>). |
| `5.3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The GeneralizedTime values included in this field MUST be expressed in Greenwich Mean Time (Zulu), and MUST be specified and interpreted as defined in Section 4.1.2.5.2. |
| `5.3.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST include in this extension the distinguished name (DN) from the issuer field of the certificate that corresponds to this CRL entry. |
| `5.3.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | The encoding of the DN MUST be identical to the encoding used in the certificate. |
| `5.3.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | CRL issuers MUST mark this extension as critical since an implementation that ignored this extension could not correctly attribute CRL entries to certificates. |
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | states for the whole of Section 6 the equivalence the Section 6.1 sentence requires of path processing | Conforming implementations of this specification are not required to implement this algorithm, but MUST provide functionality equivalent to the external behavior resulting from this procedure. |
| `6.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.2 rule that a certificate with an unsupported critical extension is rejected | Note that clients MUST reject the certificate if it contains an unsupported critical extension. |
| `6.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the initial-permitted-subtrees input of the algorithm; the MUST describes the input's meaning and obliges no implementer | (h) initial-permitted-subtrees, which indicates for each name type (e.g., X.500 distinguished names, email addresses, or IP addresses) a set of subtrees within which all subject names in every certificate in the certification path MUST fall. |
| `6.1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the permitted_subtrees state variable; the MUST describes its meaning and obliges no implementer | (b) permitted_subtrees: a set of root names for each name type (e.g., X.500 distinguished names, email addresses, or IP addresses) defining a set of subtrees within which all subject names in subsequent certificates in the certification path MUST fall. |
| `7.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | names the matching rule of the comparison the earlier sentence requires | Conforming implementations MUST support name comparisons using caseIgnoreMatch. |
| `7.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the string preparation step of the comparison the earlier sentence requires | Before comparing names using the caseIgnoreMatch matching rule, conforming implementations MUST perform the six-step string preparation algorithm described in [RFC4518] for each attribute of type DirectoryString, with the following clarifications: |
| `7.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a clarification of the string preparation the earlier sentence requires | When performing the string preparation algorithm, attributes MUST be treated as stored values. |
| `7.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the domainComponent case of the comparison the earlier sentence requires | Comparisons of domainComponent attributes MUST be performed as specified in Section 7.3. |
| `7.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | names the RFC 3490 operation of the conversion the earlier sentence requires | Specifically, conforming implementations MUST perform the conversion operation specified in Section 4 of RFC 3490, with the following clarifications: |
| `7.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a clarification of the conversion the earlier sentence requires | * in step 1, the domain name SHALL be considered a "stored string". |
| `7.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a clarification of the conversion the earlier sentence requires | That is, the AllowUnassigned flag SHALL NOT be set; |
| `7.2:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the stored-string clarification for comparison | * in step 1, the domain name SHALL be considered a "stored string". |
| `7.2:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the AllowUnassigned clarification for comparison | That is, the AllowUnassigned flag SHALL NOT be set; |
| `7.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that encodes domainComponent labels. The pki CA writes a single commonName attribute in every subject (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor) | To represent a label from an IDN in the distinguished name, the implementation MUST perform the "ToASCII" label conversion specified in Section 4.1 of RFC 3490. |
| `7.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that encodes domainComponent labels. The pki CA writes a single commonName attribute in every subject (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor) | The label SHALL be considered a "stored string". |
| `7.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that encodes domainComponent labels. The pki CA writes a single commonName attribute in every subject (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor) | That is, the AllowUnassigned flag SHALL NOT be set. |
| `7.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | To accommodate IRIs in the current structure, conforming implementations MUST map IRIs to URIs as specified in Section 3.1 of [RFC3987], with the following clarifications: |
| `7.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | Implementations MUST NOT convert the ireg-name component before performing step 2. |
| `7.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | Before URIs may be compared, conforming implementations MUST perform a combination of the syntax-based and scheme-based normalization techniques described in [RFC3987]. |
| `7.4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | Specifically, conforming implementations MUST prepare URIs for comparison as follows: |
| `7.4:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | * Step 1: Where IRIs allow the usage of IDNs, those names MUST be converted to ASCII Compatible Encoding as specified in Section 7.2 above. |
| `7.4:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | * Step 5: If recognized, the implementation MUST perform scheme- based normalization as specified in Section 5.3.3 of [RFC3987]. |
| `7.4:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies URI identities, or a relying party that compares them. Root.IssueLeafFor and GenerateWebCertWithNames write no uniformResourceIdentifier entry, and ze compares no URI (internal/component/pki/ca.go) | Conforming implementations MUST recognize and perform scheme-based normalization for the following schemes: ldap, http, https, and ftp. |
| `7.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies electronic mail identities. Root.IssueLeafFor and GenerateWebCertWithNames put hosts into dNSName and iPAddress entries only and never write an email address (internal/component/pki/ca.go) | To accommodate email addresses with internationalized domain names using the current structure, conforming implementations MUST convert the addresses into an ASCII representation. |
| `7.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CA that certifies electronic mail identities. Root.IssueLeafFor and GenerateWebCertWithNames put hosts into dNSName and iPAddress entries only and never write an email address (internal/component/pki/ca.go) | Where the host-part (the Domain of the Mailbox) contains an internationalized name, the domain name MUST be converted from an IDN to the ASCII Compatible Encoding (ACE) format as specified in Section 7.2. |
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a superior CA that issued a certificate to another CA. Ze's root is self-signed and signs leaves only (MaxPathLen 0 in internal/component/pki/ca.go generateRoot), so ze issues no certificate to a CA | If such a compromise is detected, all certificates issued to the compromised CA MUST be revoked, preventing services between its users and users of other CAs. |
| `8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.1.2.6 rule that a CA encodes its subject identically to the issuer field of what it issues | CAs MUST encode the distinguished name in the subject field of a CA certificate identically to the distinguished name in the issuer field in certificates issued by that CA. |
| `8:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the name constraints extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | In addition, name constraints for distinguished names MUST be stated identically to the encoding used in the subject field or subjectAltName extension. |
| `8:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 6.3.3 rule that the certificate and CRL paths end at the same trust anchor | At a minimum, implementations validating CRLs MUST ensure that the certification path of a certificate and the CRL issuer certification path used to validate the certificate terminate at the same trust anchor. |
| `8:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the CRL distribution points extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | CAs that include an https URI in one of these extensions MUST ensure that the server's certificate can be validated without using the information that is pointed to by the URI. |
| `A.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ASN.1 comments of the TBSCertificate definition restate the version rules of Sections 4.1.2.8 and 4.1.2.9 | Certificate ::= SEQUENCE { tbsCertificate TBSCertificate, signatureAlgorithm AlgorithmIdentifier, signature BIT STRING } TBSCertificate ::= SEQUENCE { version [0] Version DEFAULT v1, serialNumber CertificateSerialNumber, signature AlgorithmIdentifier, issuer Name, validity Validity, subject Name, subjectPublicKeyInfo SubjectPublicKeyInfo, issuerUniqueID [1] IMPLICIT UniqueIdentifier OPTIONAL, -- If present, version MUST be v2 or v3 subjectUniqueID [2] IMPLICIT UniqueIdentifier OPTIONAL, -- If present, version MUST be v2 or v3 extensions [3] Extensions OPTIONAL -- If present, version MUST be v3 -- } |
| `A.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | TBSCertList ::= SEQUENCE { version Version OPTIONAL, -- if present, MUST be v2 signature AlgorithmIdentifier, issuer Name, thisUpdate Time, nextUpdate Time OPTIONAL, revokedCertificates SEQUENCE OF SEQUENCE { userCertificate CertificateSerialNumber, revocationDate Time, crlEntryExtensions Extensions OPTIONAL -- if present, version MUST be v2 } OPTIONAL, crlExtensions [0] Extensions OPTIONAL } -- if present, version MUST be v2 |
| `A.1:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the upper bounds the comment makes mandatory are those of ITU-T X.411 Annex B for X.400 address attributes | TeletexDomainDefinedAttribute ::= SEQUENCE { type TeletexString (SIZE (1..ub-domain-defined-attribute-type-length)), value TeletexString (SIZE (1..ub-domain-defined-attribute-value-length)) } -- specifications of Upper Bounds MUST be regarded as mandatory -- from Annex B of ITU-T X.411 Reference Definition of MTS Parameter -- Upper Bounds |
| `A.2:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Support for the remaining extensions is OPTIONAL." (Section 4.2): the obligation binds a CA that issues the private key usage period extension, and the pki CA issues none (internal/component/pki/ca.go generateRoot and Root.IssueLeafFor set no such template field). The absent extension is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc5280.md | PrivateKeyUsagePeriod ::= SEQUENCE { notBefore [0] GeneralizedTime OPTIONAL, notAfter [1] GeneralizedTime OPTIONAL } -- either notBefore or notAfter MUST be present |
| `A.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | an instruction to whoever extends the ASN.1 module with further policy qualifiers; it obliges no runtime behaviour | -- Implementations that recognize additional policy qualifiers MUST -- augment the following definition for PolicyQualifierId |
| `B:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.1.2.2 rule that a CA forces the serial number non-negative | CAs MUST force the serialNumber to be a non-negative integer, that is, the sign bit in the DER encoding of the INTEGER value MUST be zero. |
| `B:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.1.2.2 rule that certificate users handle 20-octet serial numbers | Certificate users MUST be able to handle serialNumber values up to 20 octets in length. |
| `B:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.1.2.2 limit of 20 octets for a serial number | Conforming CAs MUST NOT use serialNumber values longer than 20 octets. |
| `B:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 5.2.3 rule that CRL verifiers handle 20-octet CRL numbers | CRL validators MUST be able to handle cRLNumber values up to 20 octets in length. |
| `B:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a CRL issuer. Ze issues no CRL: the pki CA (internal/component/pki/ca.go) mints certificates only, ze reads operator-supplied lists (internal/core/eap/revocation.go::parseCRLs, internal/component/pki/config.go), and x509.CreateRevocationList appears only in test fixtures | Conforming CRL issuers MUST NOT use cRLNumber values longer than 20 octets. |
| `B:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the positive form of the next sentence: an address with an at sign or underscore is never a PrintableString | Such addresses MUST be encoded using an ASN.1 type that supports them. |
| `B:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | repeats the Section 4.2 rule that a certificate with a critical extension the implementation cannot process is rejected | If an extension containing unexpected values is marked as critical, the implementation MUST reject the certificate or CRL containing the unrecognized extension. |

## Superseded

No document obsoletes RFC 5280, so its obligations are stated where they were written.
