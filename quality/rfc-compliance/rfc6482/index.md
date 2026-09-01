# RFC 6482 - A Profile for Route Origin Authorizations (ROAs)

No row in the public ledger. Every requirement this repository extracted from RFC 6482, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 0 |
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

Enrolled: A Profile for Route Origin Authorizations (ROAs): nine MUST-level requirements, all {not-applicable}. ze is neither a ROA producer nor a relying-party validator: it consumes Validated ROA Payloads (prefix, maxLength, origin AS) over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go) and never parses or constructs the RFC 6482 CMS-signed ROA object. The gated MUSTs (ROA version, CMS content-type OID, addressFamily, maxLength constraint, and the relying-party validation duties -- validate before use, RFC 6488 checks, EE-certificate delegation containment, integrity, X.509 signature verification) all govern producing or validating the ROA object, done by the RPKI cache upstream. The nearest analog (maxLength sanity) ze enforces on the RTR PDU wire per RFC 8210, not on a ROA object.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6482.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (9):** [`RFC6482-3.1-1`](#rfc6482-3.1-1), [`RFC6482-2-1`](#rfc6482-2-1), [`RFC6482-3.3-1`](#rfc6482-3.3-1), [`RFC6482-3.3-2`](#rfc6482-3.3-2), [`RFC6482-4-1`](#rfc6482-4-1), [`RFC6482-4-2`](#rfc6482-4-2), [`RFC6482-4-3`](#rfc6482-4-3), [`RFC6482-5-1`](#rfc6482-5-1), [`RFC6482-5-2`](#rfc6482-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6482-3.1-1` | Version number of the RouteOriginAttestation MUST be 0 (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object, so it never inspects the ROA object's version field |
| `RFC6482-2-1` | The ROA content-type OID MUST appear both within the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; there is no CMS/ASN.1 ROA decode in the RPKI path, so no eContentType/content-type OID is ever checked |
| `RFC6482-3.3-1` | addressFamily MUST be either 0001 (IPv4) or 0002 (IPv6) (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; ze receives per-address-family RTR Prefix PDUs (IPv4 Type 4 / IPv6 Type 6), not ROA ipAddrBlocks with an addressFamily field |
| `RFC6482-3.3-2` | If present, maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the address family maximum (32 for IPv4, 128 for IPv6) (Section 3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; the equivalent maxLength sanity (maxLen within prefixLen..AF-max) is enforced on the RTR PDU wire format per RFC 8210 at internal/component/bgp/plugins/rpki/rtr_pdu.go:125, not on a ROA object |
| `RFC6482-4-1` | The relying party MUST first validate the ROA before using it (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; validating the ROA before use is a relying-party duty performed by the RPKI cache, and ze consumes only the already-validated VRPs |
| `RFC6482-4-2` | The relying party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation step (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; the RFC 6488 and ROA-specific validation checks are performed by the relying-party cache, not by ze |
| `RFC6482-4-3` | Each IP address prefix in the ROA MUST be contained within the set of IP addresses specified by the EE certificate's IP address delegation extension (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; checking each prefix against the EE certificate's IP delegation extension requires the ROA CMS and EE certificate that ze never handles |
| `RFC6482-5-1` | The integrity of a ROA MUST be established (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; establishing ROA integrity is a relying-party/cache responsibility |
| `RFC6482-5-2` | One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefixes in the ROA match those in the certificate's address space extension (Section 5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; verifying the ROA's X.509 signature and address-space match is a relying-party/cache responsibility ze does not perform |
| `RFC6482-3.3-3` | A ROA MAY contain two ROAIPAddress elements with identical IP address prefix, but this is NOT RECOMMENDED as the shorter maxLength grants no additional privileges (Section 3.3) | NOT RECOMMENDED | 3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6482-3.1-1`](#rfc6482-3.1-1) Version number of the RouteOriginAttestation MUST be 0 (Section 3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object, so it never inspects the ROA object's version field |
| [`RFC6482-2-1`](#rfc6482-2-1) The ROA content-type OID MUST appear both within the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (Section 2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; there is no CMS/ASN.1 ROA decode in the RPKI path, so no eContentType/content-type OID is ever checked |
| [`RFC6482-3.3-1`](#rfc6482-3.3-1) addressFamily MUST be either 0001 (IPv4) or 0002 (IPv6) (Section 3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; ze receives per-address-family RTR Prefix PDUs (IPv4 Type 4 / IPv6 Type 6), not ROA ipAddrBlocks with an addressFamily field |
| [`RFC6482-3.3-2`](#rfc6482-3.3-2) If present, maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the address family maximum (32 for IPv4, 128 for IPv6) (Section 3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; the equivalent maxLength sanity (maxLen within prefixLen..AF-max) is enforced on the RTR PDU wire format per RFC 8210 at internal/component/bgp/plugins/rpki/rtr_pdu.go:125, not on a ROA object |
| [`RFC6482-4-1`](#rfc6482-4-1) The relying party MUST first validate the ROA before using it (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; validating the ROA before use is a relying-party duty performed by the RPKI cache, and ze consumes only the already-validated VRPs |
| [`RFC6482-4-2`](#rfc6482-4-2) The relying party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation step (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; the RFC 6488 and ROA-specific validation checks are performed by the relying-party cache, not by ze |
| [`RFC6482-4-3`](#rfc6482-4-3) Each IP address prefix in the ROA MUST be contained within the set of IP addresses specified by the EE certificate's IP address delegation extension (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; checking each prefix against the EE certificate's IP delegation extension requires the ROA CMS and EE certificate that ze never handles |
| [`RFC6482-5-1`](#rfc6482-5-1) The integrity of a ROA MUST be established (Section 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; establishing ROA integrity is a relying-party/cache responsibility |
| [`RFC6482-5-2`](#rfc6482-5-2) One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefixes in the ROA match those in the certificate's address space extension (Section 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze is neither a ROA producer nor a relying-party validator; it consumes Validated ROA Payloads over the RTR protocol (RFC 8210; internal/component/bgp/plugins/rpki/rtr_pdu.go parsePrefixPDU builds VRPs) and never parses or constructs the RFC 6482 CMS-signed ROA object; verifying the ROA's X.509 signature and address-space match is a relying-party/cache responsibility ze does not perform |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6482-3.1-1`](#rfc6482-3.1-1)

Version number of the RouteOriginAttestation MUST be 0 (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.1-1, so no unit is bound to it.

### [`RFC6482-2-1`](#rfc6482-2-1)

The ROA content-type OID MUST appear both within the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-2-1, so no unit is bound to it.

### [`RFC6482-3.3-1`](#rfc6482-3.3-1)

addressFamily MUST be either 0001 (IPv4) or 0002 (IPv6) (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.3-1, so no unit is bound to it.

### [`RFC6482-3.3-2`](#rfc6482-3.3-2)

If present, maxLength MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the address family maximum (32 for IPv4, 128 for IPv6) (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-3.3-2, so no unit is bound to it.

### [`RFC6482-4-1`](#rfc6482-4-1)

The relying party MUST first validate the ROA before using it (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-1, so no unit is bound to it.

### [`RFC6482-4-2`](#rfc6482-4-2)

The relying party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation step (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-2, so no unit is bound to it.

### [`RFC6482-4-3`](#rfc6482-4-3)

Each IP address prefix in the ROA MUST be contained within the set of IP addresses specified by the EE certificate's IP address delegation extension (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-4-3, so no unit is bound to it.

### [`RFC6482-5-1`](#rfc6482-5-1)

The integrity of a ROA MUST be established (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-5-1, so no unit is bound to it.

### [`RFC6482-5-2`](#rfc6482-5-2)

One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI, and check that the prefixes in the ROA match those in the certificate's address space extension (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6482-5-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6482, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6482, so its obligations are stated where they were written.
