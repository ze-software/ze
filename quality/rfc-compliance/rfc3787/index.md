# RFC 3787 - Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS)

Partial. Every requirement this repository extracted from RFC 3787, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 4 of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 5 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 13 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 1 of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 5 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 8 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 4 |
| Discrimination records | 13 |
| Summary | `rfc/short/rfc3787.md` |
| Requirement shard | `rfc/requirements/rfc3787.md` |
| RFC text | `rfc/full/rfc3787.txt` |

## Enrolment

Enrolled: IS-IS interoperability guidelines: four MUST-level requirements over the IS-IS TLV codec and IIH origination. RFC3787-x-1 (ignore TLV 131 Inter-Domain Routing Protocol Info and TLV 133 old Authentication if received) has both polarities in TestISISIgnoreObsoleteTLVs131And133: neither is a recognized codec type constant so both fall through the opaque-unknown passthrough (retained for verbatim re-flood, never interpreted) and TLV 133 is never selected by AuthTLVIndex (only TLV 10 is auth), while a recognized TLV 129 in the same region IS still decoded (ignore is scoped, not a blanket drop). The TLV 133 clause (Section 3.2) is its own row, RFC3787-3.2-1: TestRFC3787TLV133IgnoredOnReceipt proves an LSP carrying TLV 133 is accepted and not read as authentication, and TestRFC3787TLV133NeverAuthenticates proves a configured cleartext key matching TLV 133's password still refuses the LSP as unauthenticated. RFC3787-x-2 (Section 9: IP-capable routers generate Protocols Supported TLV 129 including IP): TestISISIIHOriginationTLVs, TestRFC1195NodeProtocolsAcrossInterfaces and TestRFC1195ISHTransmitReceive prove TLV 129 with the IPv4 NLPID in the sent IIHs, LSP 0 and the ISH; TestISISHelloTLV132RequiresInterfaceAddr proves a circuit with no IPv4 address still advertises the IPv4 NLPID. Under owner ruling 2 (2026-09-30) the negative of a generate row is Ze's handling of a neighbor's PDU that lacks the item: TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected proves a neighbor Hello with no TLV 129, or a TLV 129 without 0xCC, leaves the Up adjacency resolving IPv4 to a terminal rejection (RFC 1195 Section 4.4 OSI-only assumption, Section 4.5 discard). RFC3787-10-1 (Section 10: include IP Interface Address TLV 132 in IIH PDUs) has both polarities: TestISISIIHOriginationTLVs proves the originated LAN and P2P IIH carry TLV 132, TestISISHelloTLV132RequiresInterfaceAddr proves TLV 132 is built only from a real interface address (a circuit with no IPv4 omits it), and TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop proves a neighbor Hello with no TLV 132, or one with zero entries, yields no IPv4 next hop. RFC3787-5-1 (continue narrow metrics unless all devices support wide) is {gap}: Ze originates ONLY wide metrics (TLV 22/135) and never the narrow TLV 2 by umbrella decision, so it does not fall back to narrow-metric origination for a mixed domain; disclosed in the docs/features/rfc-status.md RFC 3787 row (Partial). The 4-1/4-2 SHOULDs (overload bit), 8-1 MAY (default routes in L1) and the FORMAT rows are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Obsolete TLV 131/133 are ignored on receipt (no codec decoder, opaque passthrough, TLV 133 never treated as auth)
- the originated IIH carries the Protocols Supported TLV (129, IPv4 NLPID) and IP Interface Address TLV (132) for mixed-environment interoperability. Tests bound per requirement in [`rfc/requirements/rfc3787.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc3787.md).


**What the ledger says remains**

One MUST gap, gated in [`rfc/short/rfc3787.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3787.md): Ze originates ONLY wide metrics (TLV 22/135) and never the narrow TLV 2, so it does not fall back to narrow-metric origination for a mixed narrow/wide domain -- it requires every device to be wide-capable. Ze still DECODES a legacy neighbor's narrow TLV 2.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC3787-x-1`](#rfc3787-x-1), [`RFC3787-3.2-1`](#rfc3787-3.2-1), [`RFC3787-x-2`](#rfc3787-x-2), [`RFC3787-10-1`](#rfc3787-10-1)

**Annotated (including scoped evidence) (1):** [`RFC3787-5-1`](#rfc3787-5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3787-x-1` | TLV 131 is not used, and MUST be ignored if received. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L57). **positive:** `unit/verify` [`TestRFC1195UnknownTLVsFloodedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go#L24). **negative:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L60) |
| `RFC3787-3.2-1` | TLV 133 is not used, and MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC3787TLV133IgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L44). **negative:** `unit/verify` [`TestRFC3787TLV133NeverAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L69) |
| `RFC3787-4-1` | An implementation SHOULD use the Overload Bit to signal that it is not ready to accept transit traffic. An implementation SHOULD not set the Overload bit in PseudoNode LSPs that it generates, and Overload bits seen in PseudoNode LSPs SHOULD be ignored. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-4-2` | When processing LSPs received from a router which has the Overload bit set in LSP number Zero, the receiving router SHOULD treat all IP reachability advertisements as directly connected and use them in its SPF computation. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-5-1` | If not all devices in the IS-IS domain support wide metrics, narrow metrics MUST continue to be used. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze originates ONLY wide metrics by umbrella decision -- it never emits the narrow TLV 2 (internal/plugins/isis/packet/tlv_neighbours.go:59-60 "Ze never originates TLV 2"; internal/plugins/isis/types/metric.go:15 "Only wide metrics are originated by Ze"). Ze DECODES a legacy neighbor's narrow TLV 2 (decode-only) so it can parse mixed-domain LSPs, but it does not fall back to narrow-metric ORIGINATION, so a narrow-only router cannot interpret Ze's advertisements. Ze therefore requires every device in the domain to support wide metrics rather than continuing with narrow until the whole domain is wide-capable. Disclosed in docs/features/rfc-status.md |
| `RFC3787-8-1` | an implementation MAY generate default routes in Level 1. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-x-2` | IP capable routers MUST generate a Protocol Supported TLV, and MUST include the IP protocol as a supported protocol. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L233). **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L168). **positive:** `unit/verify` [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L244). **positive:** `unit/verify` [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L141). **negative:** `unit/verify` [`TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L58) |
| `RFC3787-10-1` | For an implementation to interoperate in a such mixed environment, it MUST include an IP Interface address (TLV 132) in its IIH PDUs. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L172). **negative:** `unit/verify` [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L227). **negative:** `unit/verify` [`TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L95) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3787-5-1`](#rfc3787-5-1) If not all devices in the IS-IS domain support wide metrics, narrow metrics MUST continue to be used. (§5) | {gap}, no test | Ze originates ONLY wide metrics by umbrella decision -- it never emits the narrow TLV 2 (internal/plugins/isis/packet/tlv_neighbours.go:59-60 "Ze never originates TLV 2"; internal/plugins/isis/types/metric.go:15 "Only wide metrics are originated by Ze"). Ze DECODES a legacy neighbor's narrow TLV 2 (decode-only) so it can parse mixed-domain LSPs, but it does not fall back to narrow-metric ORIGINATION, so a narrow-only router cannot interpret Ze's advertisements. Ze therefore requires every device in the domain to support wide metrics rather than continuing with narrow until the whole domain is wide-capable. Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3787-x-1`](#rfc3787-x-1)

TLV 131 is not used, and MUST be ignored if received. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Receive path now driven: lsdb TestRFC1195UnknownTLVsFloodedUnchanged stores an LSP carrying TLV 131 via Flooder.ReceiveLSP (not refused) and floods it byte-identical with 131 present, so a receiver that refused, stripped or rewrote on 131 goes red. Packet TestISISIgnoreObsoleteTLVs131And133: DecodeTLVs returns no error and keeps 131 opaque (positive); the negative is the scope control, a recognized TLV 129 in the same region is still decoded to its NLPID, so a decoder that dropped the PDU content on meeting 131 goes red. Tag prose narrowed to 131 (sec 3.1). TLV 131 is an L2 LSP TLV, so no IIH input is owed. The 'recognized' constant loop cannot go red on any producer change and carries no weight here. Records: + ReceiveLSP, +/- DecodeTLVs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC1195UnknownTLVsFloodedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L57) | unit/verify | revert, verified |

### [`RFC3787-3.2-1`](#rfc3787-3.2-1)

TLV 133 is not used, and MUST be ignored. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30. Positive TestRFC3787TLV133IgnoredOnReceipt: an L2 LSP whose first TLV is TLV 133 (auth-type 1 + cleartext password) decodes, keeps TLV 133 as an opaque span, AuthTLVIndex returns -1 and VerifyPDU with no keys accepts it. Negative TestRFC3787TLV133NeverAuthenticates: under a cleartext key equal to TLV 133's password, VerifyPDU returns ErrAuthMissing; a receiver that honored TLV 133 as authentication would authenticate the LSP and go red. Recorded red on packet/tlv_opaque.go::AuthTLVIndex (both).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3787TLV133NeverAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC3787TLV133IgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L44) | unit/verify | revert, verified |

### [`RFC3787-5-1`](#rfc3787-5-1)

If not all devices in the IS-IS domain support wide metrics, narrow metrics MUST continue to be used. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3787-5-1, so no unit is bound to it.

### [`RFC3787-x-2`](#rfc3787-x-2)

IP capable routers MUST generate a Protocol Supported TLV, and MUST include the IP protocol as a supported protocol. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) under OWNER RULING 2: for a generate row the positive proves what Ze generates and the negative proves Ze's handling of a peer PDU that lacks it. RFC 3787 section 9 names that handling itself: 'A router that does not include the Protocols Supported TLV may be assumed to be a pure OSI router'. Positive: circuit TestISISIIHOriginationTLVs (TLV 129 in sent LAN and P2P IIH), root TestRFC1195NodeProtocolsAcrossInterfaces (0xCC in every IIH and LSP 0), root TestRFC1195ISHTransmitReceive (ISH TLV 129 = 0xCC), circuit TestISISHelloTLV132RequiresInterfaceAddr (retagged positive, D-15: no IPv4 address still advertises 0xCC); records + on protocolsSupportedTLV. Negative: root TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected dispatches a peer P2P IIH through the engine dispatcher with no TLV 129, TLV 129 CLNP-only, and TLV 129 IPv6-only, each with TLV 132 = 192.0.2.2 present, and requires the Up adjacency to resolve IPv4 to Unsupported with no address (RFC 1195 section 4.5 discard); read against circuit/ish.go receivedProtocols (absent -> ProtocolCLNP) and spf_wiring.go ResolveNextHop (no IPv4 bit -> Unsupported). Record - revert receivedProtocols; the author's semantic overlay (absent -> CLNP|IPv4) went red on no_TLV_129 with next hop 192.0.2.2. The CLNP-only and IPv6-only cases isolate the NLPID check from the presence check. Orphan x-2 negative record on TestISISHelloTLV132RequiresInterfaceAddr pruned (tag is now positive).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L141) | unit/verify | revert, verified |

### [`RFC3787-10-1`](#rfc3787-10-1)

For an implementation to interoperate in a such mixed environment, it MUST include an IP Interface address (TLV 132) in its IIH PDUs. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) under OWNER RULING 2. Positive circuit TestISISIIHOriginationTLVs requires exactly one TLV 132 whose value is the configured c0 00 02 01 in the sent LAN and P2P IIH (record + revert ipv4InterfaceAddrTLV). Negatives: circuit TestISISHelloTLV132RequiresInterfaceAddr (generation side: a circuit with no IPv4 omits TLV 132 rather than fabricating one; record - revert ipv4InterfaceAddrTLV) and NEW root TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop (handling of absence: a peer IIH advertising 0xCC with no TLV 132, or TLV 132 with zero entries, still forms an Up adjacency, as RFC 3787 section 10 and RFC 1195 section 4.4 form adjacencies 'independent of the IP interface addresses', and ResolveNextHop answers not-found, inventing neither gateway nor rejection; record - revert spf_wiring.go ResolveNextHop, author's overlay returning an address-less on-link hop went red on both subtests). Read against ResolveNextHop: row.IPv4 empty -> continue -> (NextHop{}, false).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L227) | unit/verify | revert, verified |
| negative | [`TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L172) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3787.txt |
| Source fingerprint | b5160bf591e297a3 |
| Record | rfc/extraction/rfc3787.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 3787 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 3787, so its obligations are stated where they were written.
