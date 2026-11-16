# RFC 9582 - A Profile for Route Origin Authorizations (ROAs)

A Profile for Route Origin Authorizations (ROAs). Every requirement listed for RFC 9582, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 16 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 16 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 16 | of 22 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (third-party), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | A Profile for Route Origin Authorizations (ROAs) |
| Automatic checks | Excluded (third-party) |
| Requirements | 22 |
| Mandatory requirements (MUST level), not checked | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 16 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc9582.md` |
| Requirement table | `rfc/requirements/rfc9582.md` |
| RFC text | `rfc/full/rfc9582.txt` |

## Automatic checks

Excluded from automatic checks (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The configured relying-party cache, such as Routinator or rpki-client, validates the CMS and X.509 ROA profile. Ze's RTR client consumes the resulting validated prefix payload through `internal/component/bgp/plugins/rpki/rtr_pdu.go::parsePrefixPDU`.

## Published support status

**Area:** A Profile for Route Origin Authorizations (ROAs)

**Test verification:** 0 of 16 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 16 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **16** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (16):** [`RFC9582-3-1`](#rfc9582-3-1), [`RFC9582-4.1-1`](#rfc9582-4.1-1), [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1), [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2), [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3), [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4), [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1), [`RFC9582-5-1`](#rfc9582-5-1), [`RFC9582-5-2`](#rfc9582-5-2), [`RFC9582-5-3`](#rfc9582-5-3), [`RFC9582-5-4`](#rfc9582-5-4), [`RFC9582-5-5`](#rfc9582-5-5), [`RFC9582-5-6`](#rfc9582-5-6), [`RFC9582-5-7`](#rfc9582-5-7), [`RFC9582-6-1`](#rfc9582-6-1), [`RFC9582-6-2`](#rfc9582-6-2)

## Figures

- **MUST bar**: Not assessed 16
- **Overview bar**: Not assessed 22

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 16 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9582.json](../../../data/rfc/rfc9582.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9582-3-1`](#rfc9582-3-1)

The content-type for a ROA is defined as id-ct-routeOriginAuthz and has the numerical value 1.2.840.113549.1.9.16.1.24. This OID MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (see [RFC6488]). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-3-1, so there is no linked test.

### [`RFC9582-4.1-1`](#rfc9582-4.1-1)

The version number of the RouteOriginAttestation entry MUST be 0 (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.1-1, so there is no linked test.

### [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1)

addressFamily MUST be either 0001 or 0002 (§4.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.3.1-1, so there is no linked test.

### [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2)

IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.3.1-2, so there is no linked test.

### [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3)

There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.3.1-3, so there is no linked test.

### [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4)

Thus, the ROAIPAddressFamily structure MUST NOT appear more than twice. (§4.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.3.1-4, so there is no linked test.

### [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1)

If present, the maxLength element MUST be: * an integer greater than or equal to the length of the accompanying prefix, and * less than or equal to the maximum length (in bits) of an IP address in the applicable address family: 32 in the case of IPv4 and 128 in the case of IPv6. (§4.3.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-4.3.2.2-1, so there is no linked test.

### [`RFC9582-5-1`](#rfc9582-5-1)

Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-1, so there is no linked test.

### [`RFC9582-5-2`](#rfc9582-5-2)

To validate a ROA, the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps: (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-2, so there is no linked test.

### [`RFC9582-5-3`](#rfc9582-5-3)

* The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in [RFC3779]. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-3, so there is no linked test.

### [`RFC9582-5-4`](#rfc9582-5-4)

* The Autonomous System identifier delegation extension described in [RFC3779] is not used in ROAs and MUST NOT be present in the EE certificate. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-4, so there is no linked test.

### [`RFC9582-5-5`](#rfc9582-5-5)

If any of the above checks fail, the ROA in its entirety MUST be considered invalid (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-5, so there is no linked test.

### [`RFC9582-5-6`](#rfc9582-5-6)

* The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-6, so there is no linked test.

### [`RFC9582-5-7`](#rfc9582-5-7)

The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-5-7, so there is no linked test.

### [`RFC9582-6-1`](#rfc9582-6-1)

Thus, the integrity of a ROA MUST be established. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-6-1, so there is no linked test.

### [`RFC9582-6-2`](#rfc9582-6-2)

Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9582-6-2, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `4.3.1` | not stated | 4 | reviewed | not stated |
| `4.3.2` | not stated | 0 | reviewed | not stated |
| `4.3.2.1` | not stated | 0 | reviewed | not stated |
| `4.3.2.2` | not stated | 1 | reviewed | not stated |
| `4.3.2.3` | not stated | 0 | reviewed | not stated |
| `4.3.3` | not stated | 0 | reviewed | not stated |
| `4.3.3.1` | not stated | 0 | reviewed | not stated |
| `4.3.3.2` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 5 | reviewed | not stated |
| `6` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `7.3` | not stated | 0 | reviewed | not stated |
| `7.4` | not stated | 0 | reviewed | not stated |
| `7.5` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate on the Copyright Notice page. The lowercase 'must include Revised BSD License text' governs reuse of the document's code components, not ROA behaviour. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A bullet in the Introduction's list of what this document specifies. It describes the document's own contents; the obligation itself is Section 5's 'the Relying Party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation steps', which site 5:2 maps. | * Additional steps required to validate ROAs (in addition to the validation steps specified in [RFC6488]). |
| `7.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A field label in the IANA media-type registration template of Section 7.5. 'Required parameters: N/A' names a registry field and its value, and states no protocol obligation. | Required parameters: N/A |

## Superseded

No document obsoletes RFC 9582, so its obligations are stated where they were written.
