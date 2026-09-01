# RFC 5301 - Dynamic Hostname Exchange Mechanism for IS-IS

Supported. Every requirement this repository extracted from RFC 5301, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 7 of 7 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 7 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 7 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 7 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 7 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 7 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5301.md` |
| Requirement shard | `rfc/requirements/rfc5301.md` |
| RFC text | `rfc/full/rfc5301.txt` |

## Enrolment

Enrolled: Dynamic Hostname Exchange Mechanism for IS-IS (TLV 137): seven MUST-level requirements read from the indicative prose of section 3, all met and test-bound with positive+negative tags. 3-4 (TLV type 137), 3-5 (the length octet is the total length of the value), 3-6 (a value of 1 to 255 bytes) and 3-8 (the string is not null-terminated) are proven over the real origination path in internal/plugins/isis/lsdb (encode_test.go builds the node's own LSP and reads the framed octets out of Entry.Raw), and 3-4 also carries interop evidence: test/interop/scenarios/isis-p2p-frr renders Ze's name in FRR's IS-IS database, which FRR can only do by decoding type 137. 3-7 (the value is encoded in 7-bit ASCII) and 3-9 (the content is a domain name per RFC 2181) are enforced at the config boundary by ISISHostnameValidator (internal/component/config/validators.go), which refuses any octet outside 0x20..0x7e and any break of the RFC 2181 section 11 label lengths; the value is REFUSED rather than sanitised at emit, so the name an operator configures and the name a peer reads are the same string. 3-10 (the IDNA duty of a user-interface that permits Unicode) is met under Reading A, recorded in rfc/short/rfc5301.md: the configuring interface refuses Unicode, so the sentence's antecedent is false and no ToASCII conversion is owed. The refusal is observable in both polarities rather than annotated, and no row carries {gap} or {not-applicable}. test/isis/isis-hostname-ascii.ci proves the boundary through `ze config validate`. Enrolled 2026-08-10.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Dynamic hostname TLV 137 origination and decode ([`internal/plugins/isis/packet/tlv_core.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core.go) `writeHostnameTLV`, [`internal/plugins/isis/lsdb/encode.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode.go) `hostnameTLV`). The advertised value is refused at the config boundary unless it is printable 7-bit ASCII and its labels satisfy RFC 2181 section 11. `ISISHostnameValidator` ([`internal/component/config/validators.go`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators.go)) enforces that, reached through the `ze:validate` extension on the hostname leaf in [`internal/plugins/isis/yang/ze-isis-conf.yang`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/yang/ze-isis-conf.yang). Nothing sanitizes or converts the value on the way to the wire. The name an operator configures is the name a peer reads. `sanitizeHostname` ([`internal/plugins/isis/show.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/show.go)) drops every octet outside 0x20..0x7e from a RECEIVED value before display, which keeps a peer's malformed advertisement out of the CLI. Obligations extracted 2026-07-30 and bound per line in [`rfc/short/rfc5301.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5301.md). The walk is recorded in [`rfc/extraction/rfc5301.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc5301.json).

**What the ledger says remains**

Enrolled 2026-08-10. All seven gated obligations of section 3 are proven in both polarities, and no row carries `{gap}` or `{not-applicable}`. The IDNA sentence of section 3 is conditional on a user-interface that permits Unicode characters. Ze's refuses it. The antecedent is false, so no ToASCII conversion is owed. That reading is recorded in [`rfc/short/rfc5301.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5301.md) beside the requirement. The receive path stays lenient on purpose. RFC 5301 section 4 lets a receiver ignore or install the mapping. Rejecting a peer's LSP over its hostname octets would be a denial-of-service lever.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5301-3-4`](#rfc5301-3-4), [`RFC5301-3-5`](#rfc5301-3-5), [`RFC5301-3-6`](#rfc5301-3-6), [`RFC5301-3-7`](#rfc5301-3-7), [`RFC5301-3-8`](#rfc5301-3-8), [`RFC5301-3-9`](#rfc5301-3-9), [`RFC5301-3-10`](#rfc5301-3-10)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5301-3-4` | The Dynamic hostname TLV is defined here as TLV type 137 (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L110). **negative:** `unit/verify` [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L166). **positive:** `interop/nightly` [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1135) |
| `RFC5301-3-5` | Length - total length of the value field (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L114). **negative:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L125) |
| `RFC5301-3-6` | Value - a string of 1 to 255 bytes (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L118). **negative:** `unit/verify` [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L162). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L100). **positive:** `interop/nightly` [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1139) |
| `RFC5301-3-7` | The Value field is encoded in 7-bit ASCII (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L149). **positive:** `unit/verify` [`TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`](https://github.com/ze-software/ze/blob/main/internal/component/config/cli/cmd_validate_startup_agreement_test.go#L43). **negative:** `unit/verify` [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L153). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L55). **positive:** `functional/verify` [`isis-hostname-startup-refused.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-startup-refused.ci#L16) |
| `RFC5301-3-8` | The string is not null-terminated (Section 3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L121). **negative:** `unit/verify` [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L80) |
| `RFC5301-3-9` | The content of this value is a domain name, see [RFC2181] (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L196). **negative:** `unit/verify` [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L201). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L73) |
| `RFC5301-3-10` | If a user-interface for configuring or displaying this field permits Unicode characters, that user-interface is responsible for applying the ToASCII and/or ToUnicode algorithm as described in [RFC3490] to achieve the correct format for transmission or display (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestISISHostnameUnicodeRefusedNotConverted`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L260). **negative:** `unit/verify` [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L72). **positive:** `functional/verify` [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L56) |
| `RFC5301-3-1` | The use of FQDN or a subset of it is strongly recommended (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-3` | If this TLV is present in a pseudonode LSP, then it SHOULD NOT be interpreted as the DNS hostname of the router (Section 3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-1` | If a system receives a mapping for a name or system ID that is different from the mapping in the local cache, an implementation SHOULD replace the existing mapping with the latest information (Section 4) | SHOULD | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-2` | This TLV may be present in any fragment of a non-pseudonode LSP (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-3-11` | The Dynamic hostname TLV is optional (Section 3) | OPTIONAL | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-2` | When originating an LSP, a router may decide to include this TLV in its LSP (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-3` | Upon receipt of an LSP with the Dynamic hostname TLV, a router may decide to ignore this TLV, or to install the symbolic name and system ID in its hostname mapping table for the IS-IS network (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |
| `RFC5301-4-4` | A router may also optionally insert this TLV in its pseudonode LSP for the association of a symbolic name to a local LAN (Section 4) | MAY | 4 - Implementation | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5301 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5301-3-4`](#rfc5301-3-4)

The Dynamic hostname TLV is defined here as TLV type 137 (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L166) | unit/verify | unproven |
| positive | [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1135) | interop/nightly | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L110) | unit/verify | unproven |

### [`RFC5301-3-5`](#rfc5301-3-5)

Length - total length of the value field (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L125) | unit/verify | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L114) | unit/verify | unproven |

### [`RFC5301-3-6`](#rfc5301-3-6)

Value - a string of 1 to 255 bytes (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameEmptyOmitsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L162) | unit/verify | unproven |
| positive | [`checkISISDynamicHostname`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L1139) | interop/nightly | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L118) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L100) | functional/verify | unproven |

### [`RFC5301-3-7`](#rfc5301-3-7)

The Value field is encoded in 7-bit ASCII (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L153) | unit/verify | unproven |
| positive | [`TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`](https://github.com/ze-software/ze/blob/main/internal/component/config/cli/cmd_validate_startup_agreement_test.go#L43) | unit/verify | unproven |
| positive | [`TestISISHostnameValidatorCharset`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L149) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L55) | functional/verify | unproven |
| positive | [`isis-hostname-startup-refused.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-startup-refused.ci#L16) | functional/verify | unproven |

### [`RFC5301-3-8`](#rfc5301-3-8)

The string is not null-terminated (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L80) | unit/verify | unproven |
| positive | [`TestISISHostnameTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L121) | unit/verify | unproven |

### [`RFC5301-3-9`](#rfc5301-3-9)

The content of this value is a domain name, see [RFC2181] (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L201) | unit/verify | unproven |
| positive | [`TestISISHostnameValidatorLabels`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L196) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L73) | functional/verify | unproven |

### [`RFC5301-3-10`](#rfc5301-3-10)

If a user-interface for configuring or displaying this field permits Unicode characters, that user-interface is responsible for applying the ToASCII and/or ToUnicode algorithm as described in [RFC3490] to achieve the correct format for transmission or display (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHostnameTLVIsPrintableASCII`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/encode_test.go#L72) | unit/verify | unproven |
| positive | [`TestISISHostnameUnicodeRefusedNotConverted`](https://github.com/ze-software/ze/blob/main/internal/component/config/validators_isis_test.go#L260) | unit/verify | unproven |
| positive | [`isis-hostname-ascii.ci`](https://github.com/ze-software/ze/blob/main/test/isis/isis-hostname-ascii.ci#L56) | functional/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 |
| Signed off | 2026-07-30 |
| Register | prose |
| Source | rfc/full/rfc5301.txt |
| Source fingerprint | d1ee0177891d12fe |
| Record | rfc/extraction/rfc5301.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract states what the document defines and directs nobody. |
| `1` | Introduction | 1 | walked | Introduction. Motivates a name-to-systemID mapping and describes why static tables do not scale. Its one prose site is the burden static configuration imposes on an operator, excluded below. |
| `1.1` | not stated | 0 | walked | Specification of Requirements: the RFC 2119 key-words paragraph. Tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. It is also the whole of the document's capitalised-keyword content. |
| `2` | Possible Solutions | 0 | walked | Possible Solutions. Compares static configuration and DNS against in-protocol advertisement, then states that this document defines a new TLV. Rationale only. |
| `3` | not stated | 0 | walked | Dynamic Hostname TLV: the definitional section, and the source of every gated requirement this summary declares. Written wholly in indicative prose, so the site scan finds nothing here and each requirement is listed as unsourced against this section rather than mapped to a site. |
| `4` | Implementation | 0 | walked | Implementation. Four sentences, all permissive or advisory: a router MAY include the TLV, MAY ignore it on receipt, MAY insert it in a pseudonode LSP, and SHOULD replace a differing cached mapping. Captured as RFC5301-4-1 through RFC5301-4-4. |
| `5` | Security Considerations | 0 | walked | Security Considerations. Warns that a compromised router can inject false mapping information and that the mapping must be treated with suspicion during an incident. An operational caution, not a directive on the protocol machinery. |
| `6` | Acknowledgments, crediting RFC 2763 and Henk Smit | 0 | skipped (acknowledgements) | Acknowledgments, crediting RFC 2763 and Henk Smit. |
| `7` | IANA Considerations | 1 | walked | IANA Considerations. Records that TLV 137 was already allocated by RFC 2763 and that IANA need do nothing. Walked because the prose scan attributes one site here; it is excluded below. |
| `8` | not stated | 1 | walked | Informative References, followed by the authors' addresses and the copyright statement the prose scan attributes its last site to. Walked for that site rather than skipped. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the maintenance burden of the STATIC mapping tables this document exists to replace. The 'must' binds a network operator keeping a hand-written table in step, not an IS-IS implementation. | These tables need to contain the names and system IDs of all routers in the network, and must be modified each time an addition, deletion, or change occurs. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA instruction, and a negative one: it states that no registry action is needed because RFC 2763 already allocated TLV 137. It binds no speaker. | As such, no new actions are required on the part of IANA. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The standard IETF copyright boilerplate. It addresses parties holding patents, not an IS-IS implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 5301, so its obligations are stated where they were written.
