# RFC 9582 - A Profile for Route Origin Authorizations (ROAs)

Unsupported. Every requirement this repository extracted from RFC 9582, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 16 | of 22 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 16 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Unsupported |
| Enrolment | Not enrolled (third-party) |
| Requirements | 22 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 16 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9582.md` |
| Requirement shard | `rfc/requirements/rfc9582.md` |
| RFC text | `rfc/full/rfc9582.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The configured relying-party cache, such as Routinator or rpki-client, validates the CMS and X.509 ROA profile. Ze's RTR client consumes the resulting validated prefix payload through `internal/component/bgp/plugins/rpki/rtr_pdu.go::parsePrefixPDU`.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered**

Ze consumes validated ROA payloads through RTR. The signed ROA object, EE certificate extensions, signature verification and certification-path validation belong to the configured relying-party cache. `parsePrefixPDU` decodes only the RTR prefix, maximum length and origin ASN.

**What the ledger says remains:**

The RFC 9582 signed-object profile is performed by the relying-party cache. Historical claims of RTR v2 support under this RFC number were misattributed; current RTR requirements are described in `draft-ietf-sidrops-8210bis`.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 16 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (16):** [`RFC9582-3-1`](#rfc9582-3-1), [`RFC9582-4.1-1`](#rfc9582-4.1-1), [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1), [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2), [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3), [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4), [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1), [`RFC9582-5-1`](#rfc9582-5-1), [`RFC9582-5-2`](#rfc9582-5-2), [`RFC9582-5-3`](#rfc9582-5-3), [`RFC9582-5-4`](#rfc9582-5-4), [`RFC9582-5-5`](#rfc9582-5-5), [`RFC9582-5-6`](#rfc9582-5-6), [`RFC9582-5-7`](#rfc9582-5-7), [`RFC9582-6-1`](#rfc9582-6-1), [`RFC9582-6-2`](#rfc9582-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9582-3-1` | The content-type for a ROA is defined as id-ct-routeOriginAuthz and has the numerical value 1.2.840.113549.1.9.16.1.24. This OID MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (see [RFC6488]). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.1-1` | The version number of the RouteOriginAttestation entry MUST be 0 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-1` | addressFamily MUST be either 0001 or 0002 (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-2` | IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1) | MUST NOT | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-3` | There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-4` | Thus, the ROAIPAddressFamily structure MUST NOT appear more than twice. (§4.3.1) | MUST NOT | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-1` | If present, the maxLength element MUST be: * an integer greater than or equal to the length of the accompanying prefix, and * less than or equal to the maximum length (in bits) of an IP address in the applicable address family: 32 in the case of IPv4 and 128 in the case of IPv6. (§4.3.2.2) | MUST | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-1` | Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-2` | To validate a ROA, the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps: (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-3` | * The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in [RFC3779]. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-4` | * The Autonomous System identifier delegation extension described in [RFC3779] is not used in ROAs and MUST NOT be present in the EE certificate. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-5` | If any of the above checks fail, the ROA in its entirety MUST be considered invalid (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-6` | * The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-7` | The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-6-1` | Thus, the integrity of a ROA MUST be established. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-6-2` | Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-2` | The maxLength element SHOULD NOT be encoded if the maximum length is equal to the prefix length (§4.3.2.2) | SHOULD NOT | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-3` | Certification Authorities SHOULD anticipate that future Relying Parties will become increasingly stringent in considering the presence of superfluous maxLength elements an encoding error (§4.3.2.2) | SHOULD | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.3-1` | In order to produce and verify this canonical form, the process described in this section SHOULD be used to ensure that information elements are unique with respect to one another and sorted in ascending order. (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.3-2` | Certification Authorities SHOULD anticipate that future Relying Parties will impose a strict requirement for the ipAddrBlocks field to be in this canonical form. (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-8` | If any of the above checks fail, the ROA in its entirety MUST be considered invalid and an error SHOULD be logged. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.3-1` | Additionally, a ROA MAY contain two ROAIPAddress elements, where the IP address prefix is identical in both cases. However, this is NOT RECOMMENDED, because in such a case, the ROAIPAddress element with the shorter maxLength grants no additional privileges to the indicated AS and thus can be omitted without changing the meaning of the ROA. (§4.3.2.3) | NOT RECOMMENDED | 4.3.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9582-3-1`](#rfc9582-3-1) The content-type for a ROA is defined as id-ct-routeOriginAuthz and has the numerical value 1.2.840.113549.1.9.16.1.24. This OID MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (see [RFC6488]). (§3) | no test | no test carries this requirement id |
| [`RFC9582-4.1-1`](#rfc9582-4.1-1) The version number of the RouteOriginAttestation entry MUST be 0 (§4.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1) addressFamily MUST be either 0001 or 0002 (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2) IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3) There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4) Thus, the ROAIPAddressFamily structure MUST NOT appear more than twice. (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1) If present, the maxLength element MUST be: * an integer greater than or equal to the length of the accompanying prefix, and * less than or equal to the maximum length (in bits) of an IP address in the applicable address family: 32 in the case of IPv4 and 128 in the case of IPv6. (§4.3.2.2) | no test | no test carries this requirement id |
| [`RFC9582-5-1`](#rfc9582-5-1) Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-2`](#rfc9582-5-2) To validate a ROA, the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps: (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-3`](#rfc9582-5-3) * The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in [RFC3779]. (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-4`](#rfc9582-5-4) * The Autonomous System identifier delegation extension described in [RFC3779] is not used in ROAs and MUST NOT be present in the EE certificate. (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-5`](#rfc9582-5-5) If any of the above checks fail, the ROA in its entirety MUST be considered invalid (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-6`](#rfc9582-5-6) * The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-7`](#rfc9582-5-7) The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5) | no test | no test carries this requirement id |
| [`RFC9582-6-1`](#rfc9582-6-1) Thus, the integrity of a ROA MUST be established. (§6) | no test | no test carries this requirement id |
| [`RFC9582-6-2`](#rfc9582-6-2) Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension. (§6) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9582-3-1`](#rfc9582-3-1)

The content-type for a ROA is defined as id-ct-routeOriginAuthz and has the numerical value 1.2.840.113549.1.9.16.1.24. This OID MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (see [RFC6488]). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-3-1, so no unit is bound to it.

### [`RFC9582-4.1-1`](#rfc9582-4.1-1)

The version number of the RouteOriginAttestation entry MUST be 0 (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.1-1, so no unit is bound to it.

### [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1)

addressFamily MUST be either 0001 or 0002 (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.1-1, so no unit is bound to it.

### [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2)

IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.1-2, so no unit is bound to it.

### [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3)

There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.1-3, so no unit is bound to it.

### [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4)

Thus, the ROAIPAddressFamily structure MUST NOT appear more than twice. (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.1-4, so no unit is bound to it.

### [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1)

If present, the maxLength element MUST be: * an integer greater than or equal to the length of the accompanying prefix, and * less than or equal to the maximum length (in bits) of an IP address in the applicable address family: 32 in the case of IPv4 and 128 in the case of IPv6. (§4.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.2.2-1, so no unit is bound to it.

### [`RFC9582-5-1`](#rfc9582-5-1)

Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-1, so no unit is bound to it.

### [`RFC9582-5-2`](#rfc9582-5-2)

To validate a ROA, the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps: (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-2, so no unit is bound to it.

### [`RFC9582-5-3`](#rfc9582-5-3)

* The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in [RFC3779]. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-3, so no unit is bound to it.

### [`RFC9582-5-4`](#rfc9582-5-4)

* The Autonomous System identifier delegation extension described in [RFC3779] is not used in ROAs and MUST NOT be present in the EE certificate. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-4, so no unit is bound to it.

### [`RFC9582-5-5`](#rfc9582-5-5)

If any of the above checks fail, the ROA in its entirety MUST be considered invalid (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-5, so no unit is bound to it.

### [`RFC9582-5-6`](#rfc9582-5-6)

* The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-6, so no unit is bound to it.

### [`RFC9582-5-7`](#rfc9582-5-7)

The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-7, so no unit is bound to it.

### [`RFC9582-6-1`](#rfc9582-6-1)

Thus, the integrity of a ROA MUST be established. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-6-1, so no unit is bound to it.

### [`RFC9582-6-2`](#rfc9582-6-2)

Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-6-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9582.txt |
| Source fingerprint | c7cf938bf211d0aa |
| Record | rfc/extraction/rfc9582.json |
| Mapped sentences | 14 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 4 | walked | not stated |
| `4.3.2` | not stated | 0 | walked | not stated |
| `4.3.2.1` | not stated | 0 | walked | not stated |
| `4.3.2.2` | not stated | 1 | walked | not stated |
| `4.3.2.3` | not stated | 0 | walked | not stated |
| `4.3.3` | not stated | 0 | walked | not stated |
| `4.3.3.1` | not stated | 0 | walked | not stated |
| `4.3.3.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 5 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate on the Copyright Notice page. The lowercase 'must include Revised BSD License text' governs reuse of the document's code components, not ROA behaviour. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A bullet in the Introduction's list of what this document specifies. It describes the document's own contents; the obligation itself is Section 5's 'the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps', which site 5:2 maps. | * Additional steps required to validate ROAs (in addition to the validation steps specified in [RFC6488]). |
| `7.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A field label in the IANA media-type registration template of Section 7.5. 'Required parameters: N/A' names a registry field and its value, and states no protocol obligation. | Required parameters: N/A |

## Superseded

No document obsoletes RFC 9582, so its obligations are stated where they were written.
