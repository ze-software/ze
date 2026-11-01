# RFC 6482 - A Profile for Route Origin Authorizations (ROAs)

No row in the public ledger. Every requirement this repository extracted from RFC 6482, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 9 | of 10 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every count below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 9 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 9 | of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 9 gated MUSTs: they add up to 9. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | No row in the public ledger |
| Enrolment | Not enrolled (third-party) |
| Requirements | 10 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6482.md` |
| Requirement shard | `rfc/requirements/rfc6482.md` |
| RFC text | `rfc/full/rfc6482.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The RPKI cache validates the CMS-signed ROA object. Ze consumes an already-validated payload over the RTR protocol, internal/component/bgp/plugins/rpki/rtr_pdu.go.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6482.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (9):** [`RFC6482-3.1-1`](#rfc6482-3.1-1), [`RFC6482-2-1`](#rfc6482-2-1), [`RFC6482-3.3-1`](#rfc6482-3.3-1), [`RFC6482-3.3-2`](#rfc6482-3.3-2), [`RFC6482-4-1`](#rfc6482-4-1), [`RFC6482-4-2`](#rfc6482-4-2), [`RFC6482-4-3`](#rfc6482-4-3), [`RFC6482-5-1`](#rfc6482-5-1), [`RFC6482-5-2`](#rfc6482-5-2)

## Figures

- **MUST bar**: Ze correctness gap 9
- **Overview bar**: SHOULD not decided 1, Ze correctness gap 9

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 9 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 1 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc6482.json](../../../data/rfc/rfc6482.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6482-3.1-1` | The version number of the RouteOriginAttestation MUST be 0. (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no RouteOriginAttestation version. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-2-1` | This OID MUST appear both within the eContentType in the encapContentInfo object as well as the content-type signed attribute in the signerInfo object (see [RFC6488]). (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze checks no eContentType or content-type signed attribute. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-3.3-1` | This specification only supports IPv4 and IPv6. Therefore, addressFamily MUST be either 0001 or 0002. (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no ROA addressFamily: an RTR Prefix PDU names its family by PDU type. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-3.3-2` | If present, the maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the length (in bits) of an IP address in the address family (32 for IPv4 and 128 for IPv6). (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no ROA maxLength: internal/component/bgp/plugins/rpki/rtr_pdu.go checks the maxLength of an RTR Prefix PDU under RFC 8210, a different object. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-4-1` | Before a relying party can use a ROA to validate a routing announcement, the relying party MUST first validate the ROA. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no ROA before using the payload derived from it. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-4-2` | To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs none of the RFC 6488 checks or the ROA-specific step. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-4-3` | To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. o The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and each IP address prefix(es) in the ROA is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no EE certificate and checks no prefix against an IP address delegation extension. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-5-1` | Thus, the integrity of a ROA MUST be established. (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze establishes no ROA integrity. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-5-2` | Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefix(es) in the ROA match those in the certificate's address space extension. (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze verifies no ROA signature and matches no prefix against a certificate's address space extension. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| `RFC6482-3.3-3` | Additionally, a ROA MAY contain two ROAIPAddress elements, where the IP address prefix is identical in both cases. However, this is NOT RECOMMENDED as, in such a case, the ROAIPAddress with the shorter maxLength grants no additional privileges to the indicated AS and thus can be omitted without changing the meaning of the ROA. (Section 3.3) | NOT RECOMMENDED | 3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6482-3.1-1`](#rfc6482-3.1-1) The version number of the RouteOriginAttestation MUST be 0. (Section 3.1) | {gap}, no test | ze reads no RouteOriginAttestation version. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-2-1`](#rfc6482-2-1) This OID MUST appear both within the eContentType in the encapContentInfo object as well as the content-type signed attribute in the signerInfo object (see [RFC6488]). (Section 2) | {gap}, no test | ze checks no eContentType or content-type signed attribute. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-3.3-1`](#rfc6482-3.3-1) This specification only supports IPv4 and IPv6. Therefore, addressFamily MUST be either 0001 or 0002. (Section 3.3) | {gap}, no test | ze reads no ROA addressFamily: an RTR Prefix PDU names its family by PDU type. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-3.3-2`](#rfc6482-3.3-2) If present, the maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the length (in bits) of an IP address in the address family (32 for IPv4 and 128 for IPv6). (Section 3.3) | {gap}, no test | ze reads no ROA maxLength: internal/component/bgp/plugins/rpki/rtr_pdu.go checks the maxLength of an RTR Prefix PDU under RFC 8210, a different object. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-4-1`](#rfc6482-4-1) Before a relying party can use a ROA to validate a routing announcement, the relying party MUST first validate the ROA. (Section 4) | {gap}, no test | ze validates no ROA before using the payload derived from it. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-4-2`](#rfc6482-4-2) To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. (Section 4) | {gap}, no test | ze performs none of the RFC 6488 checks or the ROA-specific step. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-4-3`](#rfc6482-4-3) To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. o The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and each IP address prefix(es) in the ROA is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (Section 4) | {gap}, no test | ze holds no EE certificate and checks no prefix against an IP address delegation extension. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-5-1`](#rfc6482-5-1) Thus, the integrity of a ROA MUST be established. (Section 5) | {gap}, no test | ze establishes no ROA integrity. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |
| [`RFC6482-5-2`](#rfc6482-5-2) Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefix(es) in the ROA match those in the certificate's address space extension. (Section 5) | {gap}, no test | ze verifies no ROA signature and matches no prefix against a certificate's address space extension. ze never parses or builds the CMS-signed ROA object: it consumes Validated ROA Payloads from an RPKI cache over RTR (internal/component/bgp/plugins/rpki/rtr_pdu.go), and the cache the operator runs performs this as the relying party (the summary's Implementation is third-party). No owner decision declines relying-party validation in ze, so the row is a gap and stays outside Ze's count |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6482-3.1-1`](#rfc6482-3.1-1)

The version number of the RouteOriginAttestation MUST be 0. (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.1-1, so no unit is bound to it.

### [`RFC6482-2-1`](#rfc6482-2-1)

This OID MUST appear both within the eContentType in the encapContentInfo object as well as the content-type signed attribute in the signerInfo object (see [RFC6488]). (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-2-1, so no unit is bound to it.

### [`RFC6482-3.3-1`](#rfc6482-3.3-1)

This specification only supports IPv4 and IPv6. Therefore, addressFamily MUST be either 0001 or 0002. (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.3-1, so no unit is bound to it.

### [`RFC6482-3.3-2`](#rfc6482-3.3-2)

If present, the maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the length (in bits) of an IP address in the address family (32 for IPv4 and 128 for IPv6). (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.3-2, so no unit is bound to it.

### [`RFC6482-4-1`](#rfc6482-4-1)

Before a relying party can use a ROA to validate a routing announcement, the relying party MUST first validate the ROA. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-1, so no unit is bound to it.

### [`RFC6482-4-2`](#rfc6482-4-2)

To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-2, so no unit is bound to it.

### [`RFC6482-4-3`](#rfc6482-4-3)

To validate a ROA, the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step. o The IP address delegation extension [RFC3779] is present in the end-entity (EE) certificate (contained within the ROA), and each IP address prefix(es) in the ROA is contained within the set of IP addresses specified by the EE certificate's IP address delegation extension. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-3, so no unit is bound to it.

### [`RFC6482-5-1`](#rfc6482-5-1)

Thus, the integrity of a ROA MUST be established. (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-5-1, so no unit is bound to it.

### [`RFC6482-5-2`](#rfc6482-5-2)

Specifically, one MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefix(es) in the ROA match those in the certificate's address space extension. (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-5-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc6482.txt |
| Source fingerprint | 035b0b690b67aecd |
| Record | rfc/extraction/rfc6482.json |
| Mapped sentences | 8 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 2 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate on the status page. The lowercase 'must include Simplified BSD License text' governs reuse of the document's code components, not ROA behaviour. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Item 3 of the Introduction's list of what this document specifies. It describes the document's own contents; the obligation itself is Section 4's 'the relying party MUST perform all the validation checks specified in [RFC6488] as well as the following additional ROA-specific validation step', which site 4:2 maps. | An additional step required to validate ROAs (in addition to the validation steps specified in [RFC6488]). |

## Superseded

No document obsoletes RFC 6482, so its obligations are stated where they were written.
