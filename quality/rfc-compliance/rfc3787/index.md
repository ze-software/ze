# RFC 3787 - Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS)

IS-IS interoperability guidelines. Every requirement listed for RFC 3787, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 17 | 17 of 17 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IS-IS interoperability guidelines |
| Automatic checks | Included |
| Requirements | 8 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tests linked to requirements | 17 |
| Recorded audit verdicts | 4 |
| Fault test records | 17 |
| Summary | `rfc/short/rfc3787.md` |
| Requirement table | `rfc/requirements/rfc3787.md` |
| RFC text | `rfc/full/rfc3787.txt` |

## Automatic checks

Included in automatic checks: IS-IS interoperability guidelines: four MUST-level requirements over the IS-IS TLV codec and IIH origination. RFC3787-x-1 (ignore TLV 131 Inter-Domain Routing Protocol Info and TLV 133 old Authentication if received) has both polarities in TestISISIgnoreObsoleteTLVs131And133: neither is a recognized codec type constant so both fall through the opaque-unknown passthrough (retained for verbatim re-flood, never interpreted) and TLV 133 is never selected by AuthTLVIndex (only TLV 10 is auth), while a recognized TLV 129 in the same region IS still decoded (ignore is scoped, not a blanket drop). The TLV 133 clause (Section 3.2) is its own row, RFC3787-3.2-1: TestRFC3787TLV133IgnoredOnReceipt proves an LSP carrying TLV 133 is accepted and not read as authentication, and TestRFC3787TLV133NeverAuthenticates proves a configured cleartext key matching TLV 133's password still refuses the LSP as unauthenticated. RFC3787-x-2 (Section 9: IP-capable routers generate Protocols Supported TLV 129 including IP): TestISISIIHOriginationTLVs, TestRFC1195NodeProtocolsAcrossInterfaces and TestRFC1195ISHTransmitReceive prove TLV 129 with the IPv4 NLPID in the sent IIHs, LSP 0 and the ISH; TestISISHelloTLV132RequiresInterfaceAddr proves a circuit with no IPv4 address still advertises the IPv4 NLPID. Under owner ruling 2 (2026-09-30) the negative of a generate row is Ze's handling of a neighbor's PDU that lacks the item: TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected proves a neighbor Hello with no TLV 129, or a TLV 129 without 0xCC, leaves the Up adjacency resolving IPv4 to a terminal rejection (RFC 1195 Section 4.4 OSI-only assumption, Section 4.5 discard). RFC3787-10-1 (Section 10: include IP Interface Address TLV 132 in IIH PDUs) has both polarities: TestISISIIHOriginationTLVs proves the originated LAN and P2P IIH carry TLV 132, TestISISHelloTLV132RequiresInterfaceAddr proves TLV 132 is built only from a real interface address (a circuit with no IPv4 omits it), and TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop proves a neighbor Hello with no TLV 132, or one with zero entries, yields no IPv4 next hop. RFC3787-5-1 (continue narrow metrics unless all devices support wide) has both polarities: the `isis metric-style` leaf (narrow, transition, wide; default wide) selects the originated TLVs. TestRFC3787NarrowMetricStyleOriginatesNarrowTLVs proves a delivered `metric-style narrow` config makes the own LSP list its Up neighbor in TLV 2 and its connected prefixes in TLV 128 at the configured metric with no TLV 22/135, and `transition` writes both families; TestRFC3787NarrowMetricStylePseudonodeListsMembersInTLV2 proves the pseudo-node LSP lists the LAN members in TLV 2. The negative, TestRFC3787NarrowMetricStyleRefusesUnrepresentableMetric, proves a circuit metric above 63 is refused at commit under narrow or transition rather than advertised as a different narrow value. The 4-1/4-2 SHOULDs (overload bit), 8-1 MAY (default routes in L1) and the FORMAT rows are not gated.

## Published support status

**Area:** IS-IS interoperability guidelines

**Proof:** 5 of 5 MUST proven

**Evidence:** interop 1, unit 4

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC3787-x-1`](#rfc3787-x-1), [`RFC3787-3.2-1`](#rfc3787-3.2-1), [`RFC3787-5-1`](#rfc3787-5-1), [`RFC3787-x-2`](#rfc3787-x-2), [`RFC3787-10-1`](#rfc3787-10-1)

## Figures

- **MUST bar**: Ze proven 5
- **Overview bar**: Ze MUST proven 5, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 5 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc3787.json](../../../data/rfc/rfc3787.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3787-x-1` | TLV 131 is not used, and MUST be ignored if received. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L57). **positive:** `unit/verify` [`TestRFC1195UnknownTLVsFloodedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go#L24). **negative:** `unit/verify` [`TestISISIgnoreObsoleteTLVs131And133`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L60) |
| `RFC3787-3.2-1` | TLV 133 is not used, and MUST be ignored. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC3787TLV133IgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L44). **negative:** `unit/verify` [`TestRFC3787TLV133NeverAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc3787_tlv133_test.go#L69) |
| `RFC3787-4-1` | An implementation SHOULD use the Overload Bit to signal that it is not ready to accept transit traffic. An implementation SHOULD not set the Overload bit in PseudoNode LSPs that it generates, and Overload bits seen in PseudoNode LSPs SHOULD be ignored. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-4-2` | When processing LSPs received from a router which has the Overload bit set in LSP number Zero, the receiving router SHOULD treat all IP reachability advertisements as directly connected and use them in its SPF computation. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-5-1` | If not all devices in the IS-IS domain support wide metrics, narrow metrics MUST continue to be used. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3787NarrowMetricStyleOriginatesNarrowTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_metric_style_test.go#L133). **positive:** `unit/verify` [`TestRFC3787NarrowMetricStylePseudonodeListsMembersInTLV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc3787_metric_style_test.go#L45). **negative:** `unit/verify` [`TestRFC3787NarrowMetricStyleRefusesUnrepresentableMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_metric_style_test.go#L194). **positive:** `interop/nightly` [`checkISISNarrowMetric`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_isis_narrow.go#L41) |
| `RFC3787-8-1` | an implementation MAY generate default routes in Level 1. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC3787-x-2` | IP capable routers MUST generate a Protocol Supported TLV, and MUST include the IP protocol as a supported protocol. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L233). **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L168). **positive:** `unit/verify` [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L244). **positive:** `unit/verify` [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L141). **negative:** `unit/verify` [`TestRFC3787PeerWithoutIPProtocolsSupportedIsRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L58) |
| `RFC3787-10-1` | For an implementation to interoperate in a such mixed environment, it MUST include an IP Interface address (TLV 132) in its IIH PDUs. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L172). **negative:** `unit/verify` [`TestISISHelloTLV132RequiresInterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L227). **negative:** `unit/verify` [`TestRFC3787PeerWithoutIPInterfaceAddressHasNoNextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_absence_test.go#L95) |

## Gaps and untested MUSTs

RFC 3787 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

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

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3787NarrowMetricStyleRefusesUnrepresentableMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_metric_style_test.go#L194) | unit/verify | revert, verified |
| positive | [`checkISISNarrowMetric`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_isis_narrow.go#L41) | interop/nightly | revert, verified |
| positive | [`TestRFC3787NarrowMetricStylePseudonodeListsMembersInTLV2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc3787_metric_style_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC3787NarrowMetricStyleOriginatesNarrowTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3787_metric_style_test.go#L133) | unit/verify | revert, verified |

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

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 1 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 1 | reviewed | not stated |
| `10` | not stated | 1 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `12.1` | not stated | 0 | reviewed | not stated |
| `12.2` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 3787 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 3787, so its obligations are stated where they were written.
