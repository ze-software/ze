# RFC 9582 - A Profile for Route Origin Authorizations (ROAs)

Unsupported. Every requirement this repository extracted from RFC 9582, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 23.1% | 6 of 26 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3.8% | 1 of 26 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 26 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 26 | of 35 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 3 | of 26 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.5% | 3 of 26 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 26 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 26 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 61.5% | 16 of 26 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 26 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 35 |
| Gated MUST-level | 26 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 16 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9582.md` |
| Requirement shard | `rfc/requirements/rfc9582.md` |
| RFC text | `rfc/full/rfc9582.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The RPKI cache validates the CMS and X.509 profile. Ze consumes an already-validated payload over the RTR protocol, internal/component/bgp/plugins/rpki/rtr_pdu.go.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered**

Nothing. RFC 9582 profiles the ROA signed object itself: a CMS envelope carrying a `RouteOriginAttestation`, an EE certificate bearing an RFC 3779 IP address delegation extension, and a certification path to a trust anchor. No ROA object exists anywhere in the ze process. `parsePrefixPDU` reads an already-validated payload off an RTR PDU at fixed byte offsets, so the decode, the extension reading and the path validation this RFC specifies all happen in the RPKI cache ze dials. <!-- source: [`internal/component/bgp/plugins/rpki/rtr_pdu.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu.go) -- parsePrefixPDU -->

**What the ledger says remains**

Every obligation, and implementing them means a relying-party validator: a CMS/DER decoder (RFC 5652 and RFC 6488), an RFC 3779 extension reader, X.509 path validation to a trust anchor, and an RRDP or rsync fetcher. That is what Routinator and rpki-client are, and ze delegates it deliberately. **This row read `Supported` until 2026-09-01, and the claim was never about this RFC.** [`rfc/short/rfc9582.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9582.md) still declares thirteen ids describing ASPA PDUs and RTR version negotiation, which are `draft-ietf-sidrops-8210bis`: RFC 9582 has no Section 5.12 and its Section 7 is IANA Considerations, so each id cites a section its own document does not have. The obligations are real and ze meets them; only the attribution is wrong. Correcting it is [`plan/pre-release/spec-rfc-requirement-reattribution.md`](https://github.com/ze-software/ze/blob/main/plan/pre-release/spec-rfc-requirement-reattribution.md), which the ledger's own ratchets refuse until they learn to let proof follow an obligation to another document.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 16 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **26** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC9582-5.12-1`](#rfc9582-5.12-1), [`RFC9582-5.12-2`](#rfc9582-5.12-2), [`RFC9582-5.12-3`](#rfc9582-5.12-3), [`RFC9582-5.12-6`](#rfc9582-5.12-6), [`RFC9582-5.12-7`](#rfc9582-5.12-7), [`RFC9582-7-2`](#rfc9582-7-2)

**Annotated instead of tested (4):** [`RFC9582-5.12-4`](#rfc9582-5.12-4), [`RFC9582-5.12-5`](#rfc9582-5.12-5), [`RFC9582-7-1`](#rfc9582-7-1), [`RFC9582-7-3`](#rfc9582-7-3)

**No test and no annotation (16):** [`RFC9582-3-1`](#rfc9582-3-1), [`RFC9582-4.1-1`](#rfc9582-4.1-1), [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1), [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2), [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3), [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4), [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1), [`RFC9582-5-1`](#rfc9582-5-1), [`RFC9582-5-2`](#rfc9582-5-2), [`RFC9582-5-3`](#rfc9582-5-3), [`RFC9582-5-4`](#rfc9582-5-4), [`RFC9582-5-5`](#rfc9582-5-5), [`RFC9582-5-6`](#rfc9582-5-6), [`RFC9582-5-7`](#rfc9582-5-7), [`RFC9582-6-1`](#rfc9582-6-1), [`RFC9582-6-2`](#rfc9582-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9582-5.12-1` | Provider AS set in ASPA PDU MUST contain at least one provider ASN (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L223). **negative:** `unit/verify` [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L273) |
| `RFC9582-5.12-2` | Customer AS MUST NOT appear in its own provider set (§5.12) | MUST NOT | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L224). **negative:** `unit/verify` [`TestParseASPAPDUSelfRef`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L319) |
| `RFC9582-5.12-3` | Provider ASNs MUST be in ascending order within the PDU (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L225). **negative:** `unit/verify` [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L340) |
| `RFC9582-5.12-4` | Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a cache-side emission/deduplication guarantee; ze is the RTR router that only consumes ASPA PDUs (no ASPA PDU writer exists, only query writers) and never enforces or emits this pairing (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, :103) |
| `RFC9582-5.12-5` | Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12) | MUST | 5.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** exact-(Customer-AS,AFI) withdraw matching binds the cache's emission and a per-AFI record model; ze consumes withdraws keyed on Customer-AS alone (the §5.12 router option to ignore AFI) and maintains no AFI dimension to match (internal/component/bgp/plugins/rpki/rtr_session.go:283, aspa_cache.go:114) |
| `RFC9582-5.12-6` | Router MUST ignore ASPA PDUs with unknown AFI values (§5.12) | MUST | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L226). **negative:** `unit/verify` [`TestParseASPAPDUUnknownAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L305) |
| `RFC9582-5.12-7` | Customer AS 0 is reserved, MUST NOT appear (§5.12) | MUST NOT | 5.12 | **positive:** `unit/verify` [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L227). **negative:** `unit/verify` [`TestParseASPAPDUReservedCustomerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L362) |
| `RFC9582-7-1` | Router starting a v2 session MUST send query with version=2 (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L21). **negative:** no negative test. **{single-polarity}:** ze constructs every session at rtrVersionMax and writes that version unconditionally into the initial query, so the emitted version byte is observable but there is no malformed input that yields a wrong-version query to test negatively |
| `RFC9582-7-2` | On Unsupported Protocol Version error, router MUST downgrade or disconnect (§7) | MUST | 7 | **positive:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L50). **negative:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L64) |
| `RFC9582-7-3` | Cache receiving a version it does not support MUST send error code 4 (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the RTR cache/server role; ze runs only an RTR client that dials out and reads error reports, with no listener and no error-report writer, so it never receives queries or sends error code 4 (internal/component/bgp/plugins/rpki/rtr_session.go:125) |
| `RFC9582-7-4` | Router SHOULD start at highest supported version (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5.12-8` | Provider ASNs SHOULD be sorted ascending; cache MUST sort, router SHOULD verify (§5.12) | SHOULD | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5.12-9` | Router MAY ignore AFI field and apply ASPA to all address families (§5.12) | MAY | 5.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-3-1` | The ROA content-type OID 1.2.840.113549.1.9.16.1.24 MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.1-1` | The version number of the RouteOriginAttestation entry MUST be 0 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-1` | addressFamily MUST be either 0001 or 0002 (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-2` | IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1) | MUST NOT | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-3` | There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.1-4` | The ROAIPAddressFamily structure MUST NOT appear more than twice (§4.3.1) | MUST NOT | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-1` | If present, the maxLength element MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the maximum length in bits of an IP address in the applicable address family: 32 for IPv4 and 128 for IPv6 (§4.3.2.2) | MUST | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-1` | Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-2` | To validate a ROA, the Relying Party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation steps of Section 5 (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-3` | The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in RFC 3779 (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-4` | The Autonomous System identifier delegation extension described in RFC 3779 is not used in ROAs and MUST NOT be present in the EE certificate (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-5` | If any of the Section 5 checks fail, the ROA in its entirety MUST be considered invalid (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-6` | The IP address delegation extension is present in the EE certificate and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by that extension (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-7` | The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-6-1` | The integrity of a ROA MUST be established (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-6-2` | One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-2` | The maxLength element SHOULD NOT be encoded if the maximum length is equal to the prefix length (§4.3.2.2) | SHOULD NOT | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.2-3` | Certification Authorities SHOULD anticipate that future Relying Parties will become increasingly stringent in considering the presence of superfluous maxLength elements an encoding error (§4.3.2.2) | SHOULD | 4.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.3-1` | The canonicalization process described in Section 4.3.3 SHOULD be used to ensure that information elements are unique with respect to one another and sorted in ascending order (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.3-2` | Certification Authorities SHOULD anticipate that future Relying Parties will impose a strict requirement for the ipAddrBlocks field to be in canonical form (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-5-8` | If any of the Section 5 checks fail, an error SHOULD be logged (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9582-4.3.2.3-1` | A ROA MAY contain two ROAIPAddress elements where the IP address prefix is identical in both cases, but this is NOT RECOMMENDED because the element with the shorter maxLength grants no additional privileges (§4.3.2.3) | NOT RECOMMENDED | 4.3.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9582-5.12-4`](#rfc9582-5.12-4) Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12) | no test | no test carries this requirement id; annotated {not-applicable}: this is a cache-side emission/deduplication guarantee; ze is the RTR router that only consumes ASPA PDUs (no ASPA PDU writer exists, only query writers) and never enforces or emits this pairing (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, :103) |
| [`RFC9582-5.12-5`](#rfc9582-5.12-5) Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12) | no test | no test carries this requirement id; annotated {not-applicable}: exact-(Customer-AS,AFI) withdraw matching binds the cache's emission and a per-AFI record model; ze consumes withdraws keyed on Customer-AS alone (the §5.12 router option to ignore AFI) and maintains no AFI dimension to match (internal/component/bgp/plugins/rpki/rtr_session.go:283, aspa_cache.go:114) |
| [`RFC9582-7-3`](#rfc9582-7-3) Cache receiving a version it does not support MUST send error code 4 (§7) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the RTR cache/server role; ze runs only an RTR client that dials out and reads error reports, with no listener and no error-report writer, so it never receives queries or sends error code 4 (internal/component/bgp/plugins/rpki/rtr_session.go:125) |
| [`RFC9582-3-1`](#rfc9582-3-1) The ROA content-type OID 1.2.840.113549.1.9.16.1.24 MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (§3) | no test | no test carries this requirement id |
| [`RFC9582-4.1-1`](#rfc9582-4.1-1) The version number of the RouteOriginAttestation entry MUST be 0 (§4.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-1`](#rfc9582-4.3.1-1) addressFamily MUST be either 0001 or 0002 (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-2`](#rfc9582-4.3.1-2) IPv4 prefixes MUST NOT appear as IPv4-mapped IPv6 addresses (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-3`](#rfc9582-4.3.1-3) There MUST be only one instance of ROAIPAddressFamily per unique AFI in the ROA (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.1-4`](#rfc9582-4.3.1-4) The ROAIPAddressFamily structure MUST NOT appear more than twice (§4.3.1) | no test | no test carries this requirement id |
| [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1) If present, the maxLength element MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the maximum length in bits of an IP address in the applicable address family: 32 for IPv4 and 128 for IPv6 (§4.3.2.2) | no test | no test carries this requirement id |
| [`RFC9582-5-1`](#rfc9582-5-1) Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-2`](#rfc9582-5-2) To validate a ROA, the Relying Party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation steps of Section 5 (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-3`](#rfc9582-5-3) The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in RFC 3779 (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-4`](#rfc9582-5-4) The Autonomous System identifier delegation extension described in RFC 3779 is not used in ROAs and MUST NOT be present in the EE certificate (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-5`](#rfc9582-5-5) If any of the Section 5 checks fail, the ROA in its entirety MUST be considered invalid (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-6`](#rfc9582-5-6) The IP address delegation extension is present in the EE certificate and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by that extension (§5) | no test | no test carries this requirement id |
| [`RFC9582-5-7`](#rfc9582-5-7) The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5) | no test | no test carries this requirement id |
| [`RFC9582-6-1`](#rfc9582-6-1) The integrity of a ROA MUST be established (§6) | no test | no test carries this requirement id |
| [`RFC9582-6-2`](#rfc9582-6-2) One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension (§6) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9582-5.12-1`](#rfc9582-5.12-1)

Provider AS set in ASPA PDU MUST contain at least one provider ASN (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L273) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L223) | unit/verify | unproven |

### [`RFC9582-5.12-2`](#rfc9582-5.12-2)

Customer AS MUST NOT appear in its own provider set (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUSelfRef`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L319) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L224) | unit/verify | unproven |

### [`RFC9582-5.12-3`](#rfc9582-5.12-3)

Provider ASNs MUST be in ascending order within the PDU (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnsorted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L340) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L225) | unit/verify | unproven |

### [`RFC9582-5.12-4`](#rfc9582-5.12-4)

Cache MUST ensure one ASPA PDU per (Customer-AS, AFI) pair (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5.12-4, so no unit is bound to it.

### [`RFC9582-5.12-5`](#rfc9582-5.12-5)

Withdraw ASPA MUST match exact (Customer-AS, AFI) pair (§5.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5.12-5, so no unit is bound to it.

### [`RFC9582-5.12-6`](#rfc9582-5.12-6)

Router MUST ignore ASPA PDUs with unknown AFI values (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUUnknownAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L305) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L226) | unit/verify | unproven |

### [`RFC9582-5.12-7`](#rfc9582-5.12-7)

Customer AS 0 is reserved, MUST NOT appear (§5.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseASPAPDUReservedCustomerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L362) | unit/verify | unproven |
| positive | [`TestParseASPAPDU`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L227) | unit/verify | unproven |

### [`RFC9582-7-1`](#rfc9582-7-1)

Router starting a v2 session MUST send query with version=2 (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRTRSessionStartsAtV2`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L21) | unit/verify | unproven |

### [`RFC9582-7-2`](#rfc9582-7-2)

On Unsupported Protocol Version error, router MUST downgrade or disconnect (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L64) | unit/verify | unproven |
| positive | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L50) | unit/verify | unproven |

### [`RFC9582-7-3`](#rfc9582-7-3)

Cache receiving a version it does not support MUST send error code 4 (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-7-3, so no unit is bound to it.

### [`RFC9582-3-1`](#rfc9582-3-1)

The ROA content-type OID 1.2.840.113549.1.9.16.1.24 MUST appear within both the eContentType in the encapContentInfo object and the content-type signed attribute in the signerInfo object (§3)

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

The ROAIPAddressFamily structure MUST NOT appear more than twice (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.1-4, so no unit is bound to it.

### [`RFC9582-4.3.2.2-1`](#rfc9582-4.3.2.2-1)

If present, the maxLength element MUST be an integer greater than or equal to the length of the accompanying prefix, and less than or equal to the maximum length in bits of an IP address in the applicable address family: 32 for IPv4 and 128 for IPv6 (§4.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-4.3.2.2-1, so no unit is bound to it.

### [`RFC9582-5-1`](#rfc9582-5-1)

Before a Relying Party can use a ROA to validate a routing announcement, the Relying Party MUST first validate the ROA (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-1, so no unit is bound to it.

### [`RFC9582-5-2`](#rfc9582-5-2)

To validate a ROA, the Relying Party MUST perform all the validation checks specified in RFC 6488 as well as the ROA-specific validation steps of Section 5 (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-2, so no unit is bound to it.

### [`RFC9582-5-3`](#rfc9582-5-3)

The EE certificate's IP address delegation extension MUST NOT contain "inherit" elements as described in RFC 3779 (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-3, so no unit is bound to it.

### [`RFC9582-5-4`](#rfc9582-5-4)

The Autonomous System identifier delegation extension described in RFC 3779 is not used in ROAs and MUST NOT be present in the EE certificate (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-4, so no unit is bound to it.

### [`RFC9582-5-5`](#rfc9582-5-5)

If any of the Section 5 checks fail, the ROA in its entirety MUST be considered invalid (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-5, so no unit is bound to it.

### [`RFC9582-5-6`](#rfc9582-5-6)

The IP address delegation extension is present in the EE certificate and every IP address prefix in the ROA payload is contained within the set of IP addresses specified by that extension (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-6, so no unit is bound to it.

### [`RFC9582-5-7`](#rfc9582-5-7)

The ROA content fully conforms with all requirements specified in Sections 3 and 4 (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-5-7, so no unit is bound to it.

### [`RFC9582-6-1`](#rfc9582-6-1)

The integrity of a ROA MUST be established (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9582-6-1, so no unit is bound to it.

### [`RFC9582-6-2`](#rfc9582-6-2)

One MUST verify the signature on the ROA using an X.509 certificate issued under this PKI and check that the prefix or prefixes in the ROA are contained within those in the certificate's IP address delegation extension (§6)

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
