# RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes

BGP Labeled Unicast. Every requirement listed for RFC 8277, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 35 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 35 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 35 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 35 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 57 | 57 of 69 tests linked to requirements, 0 without a fault test and 2 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 24 | of 35 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 12 | of 35 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 2 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 35 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 35 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 35 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 35 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 35 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 35 | of 42 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 35 mandatory requirements: they add up to 35. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Labeled Unicast |
| Automatic checks | Included |
| Requirements | 42 |
| Mandatory requirements (MUST level) | 35 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 24 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 69 |
| Tests linked to requirements | 69 |
| Recorded audit verdicts | 12 |
| Fault test records | 59 |
| Summary | `rfc/short/rfc8277.md` |
| Requirement table | `rfc/requirements/rfc8277.md` |
| RFC text | `rfc/full/rfc8277.txt` |

## Automatic checks

Included in automatic checks: Using BGP to Bind MPLS Labels to Address Prefixes

## Published support status

**Area:** BGP Labeled Unicast

**Test verification:** 10 of 35 MUST proven

**Evidence:** unit 10

**Unmet:** RFC8277-2-1, RFC8277-2-3, RFC8277-2.1-1, RFC8277-2.1-10, RFC8277-2.1-11, RFC8277-2.1-12, RFC8277-2.1-13, RFC8277-2.1-14, RFC8277-2.1-15, RFC8277-2.1-2, RFC8277-2.1-3, RFC8277-2.1-4, RFC8277-2.1-5, RFC8277-2.1-6, RFC8277-2.1-7, RFC8277-2.1-8, RFC8277-2.1-9, RFC8277-2.2-2, RFC8277-3.2.1-2, RFC8277-3.2.1-3, RFC8277-3.2.1-4, RFC8277-3.2.2-1, RFC8277-3.2.2-2, RFC8277-3.2.2-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 26 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **35** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC8277-2.2-1`](#rfc8277-2.2-1), [`RFC8277-2.2-3`](#rfc8277-2.2-3), [`RFC8277-2.3-1`](#rfc8277-2.3-1), [`RFC8277-2.3-2`](#rfc8277-2.3-2), [`RFC8277-2.4-1`](#rfc8277-2.4-1), [`RFC8277-2.5-1`](#rfc8277-2.5-1), [`RFC8277-2.5-2`](#rfc8277-2.5-2), [`RFC8277-2.5-3`](#rfc8277-2.5-3), [`RFC8277-3.1-1`](#rfc8277-3.1-1)

**Annotated (including scoped evidence) (26):** [`RFC8277-2-1`](#rfc8277-2-1), [`RFC8277-2-2`](#rfc8277-2-2), [`RFC8277-2.1-1`](#rfc8277-2.1-1), [`RFC8277-2.1-2`](#rfc8277-2.1-2), [`RFC8277-2-3`](#rfc8277-2-3), [`RFC8277-2.1-3`](#rfc8277-2.1-3), [`RFC8277-2.1-4`](#rfc8277-2.1-4), [`RFC8277-2.1-5`](#rfc8277-2.1-5), [`RFC8277-2.1-6`](#rfc8277-2.1-6), [`RFC8277-2.1-7`](#rfc8277-2.1-7), [`RFC8277-2.1-8`](#rfc8277-2.1-8), [`RFC8277-2.1-9`](#rfc8277-2.1-9), [`RFC8277-2.1-10`](#rfc8277-2.1-10), [`RFC8277-2.1-11`](#rfc8277-2.1-11), [`RFC8277-2.1-12`](#rfc8277-2.1-12), [`RFC8277-2.1-13`](#rfc8277-2.1-13), [`RFC8277-2.1-14`](#rfc8277-2.1-14), [`RFC8277-2.2-2`](#rfc8277-2.2-2), [`RFC8277-2.1-15`](#rfc8277-2.1-15), [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1), [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2), [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3), [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4), [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1), [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2), [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3)

## Figures

- **MUST bar**: Ze verified 10, Tests awaiting verification 1, Not built 24
- **Overview bar**: Ze MUST verified 10, SHOULD verified 1, Tests awaiting verification 1, Not assessed 6, Not built 24

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 35 |
| MUST rows kept (the MUST bar) | 35 |
| Ze verified | 10 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 24 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 7 |
| SHOULD verified | 1 |
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

Source: [data/rfc/rfc8277.json](../../../data/rfc/rfc8277.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8277-2-1` | Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2-2` | Further, when advertising the binding of a single label to a prefix, the BGP speaker MUST use the encoding specified in Section 2.2. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L28). **negative:** no negative test. **{single-polarity}:** all three transmitters emit the §2.2 layout unconditionally -- Length octet = 24 + prefix bits, one 3-octet label entry, prefix: LabeledUnicast.WriteTo (internal/component/bgp/plugins/nlri/labeled/types.go), UpdateBuilder.BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go) and buildVPNNLRIBytes (internal/component/bgp/message/update_build_vpn.go) -- so there is no non-conformant transmission input to reject |
| `RFC8277-2.1-1` | If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no session can meet the three conditions, so ze never selects the Section 2.3 encoding through a capability exchange. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-2` | If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no session can meet the three conditions, so ze never selects the Section 2.3 encoding through a capability exchange, for one label or several. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2-3` | If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2.1-3` | Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no Multiple Labels Capability, so the two-label support owed by its sender is owed when the capability lands. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-4` | A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no parser that could judge a received Multiple Labels Capability malformed by its length. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-5` | A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze writes no AFI/SAFI/Count triple. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-6` | A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes no triple, so no Count of 0 or 1 is read and ignored by a Count reader. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-7` | If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no per-AFI/SAFI triple state, so no first triple is kept over a later one. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-8` | If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze stores no Multiple Labels Capability, so no first-copy rule exists. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-9` | If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze announces no Count, so no UPDATE is judged against one and treat-as-withdraw is never applied for an excess label count. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-10` | Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| `RFC8277-2.1-11` | If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks no Multiple Labels Capability across a restart, so no multi-label prefix is withdrawn when it is lost. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-12` | Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks no per-peer maximum label count, so no prefix is withdrawn when the count falls. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-13` | If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tracks neither condition, so no full re-exchange is triggered. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.1-14` | "Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code reads a Multiple Labels Capability or Count change, the graceful restart plugin (internal/component/bgp/plugins/gr) included, so no restart procedure is guarded by one. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.2-1` | S: This 1-bit field MUST be set to one on transmission (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L29). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L30) |
| `RFC8277-2.2-2` | S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| `RFC8277-2.2-3` | Rsrv: This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L23). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L32). **positive:** `unit/verify` [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L27). **positive:** `unit/verify` [`TestRFC8277ZeroRsrvRelayedZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L130). **negative:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L24). **negative:** `unit/verify` [`TestRFC8277RsrvClearedOnRelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L67). **negative:** `unit/verify` [`TestRFC8277RsrvClearedOnTheReceivePath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L112). **negative:** `unit/verify` [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L28) |
| `RFC8277-2.3-1` | In all labels except the last (i.e., in all labels except the one immediately preceding the prefix), the S bit MUST be 0. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L30). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L31) |
| `RFC8277-2.3-2` | In the last label, the S bit MUST be 1. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L31). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L32) |
| `RFC8277-2.4-1` | Upon reception, the value of the Compatibility field MUST be ignored. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L27). **positive:** `unit/verify` [`TestLabeledWithdrawalEventNamesThePrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L71). **positive:** `unit/verify` [`TestLabeledWithdrawalEventNeverReachesTheLabelDecoder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L151). **positive:** `unit/verify` [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L76). **positive:** `unit/verify` [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L61). **positive:** `unit/verify` [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L102). **positive:** `unit/verify` [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L54). **positive:** `unit/verify` [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rfc8277_rpc_decode_nlri_context_test.go#L26). **positive:** `unit/verify` [`TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L152). **positive:** `unit/verify` [`TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L155). **positive:** `unit/verify` [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L120). **positive:** `unit/verify` [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L25). **positive:** `unit/verify` [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L26). **positive:** `unit/verify` [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L33). **positive:** `unit/verify` [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L36). **positive:** `unit/verify` [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L30). **negative:** `unit/verify` [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L28). **negative:** `unit/verify` [`TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L96). **negative:** `unit/verify` [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L77). **negative:** `unit/verify` [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L62). **negative:** `unit/verify` [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L103). **negative:** `unit/verify` [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L55). **negative:** `unit/verify` [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rfc8277_rpc_decode_nlri_context_test.go#L27). **negative:** `unit/verify` [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L121). **negative:** `unit/verify` [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L26). **negative:** `unit/verify` [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L27). **negative:** `unit/verify` [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L34). **negative:** `unit/verify` [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L37). **negative:** `unit/verify` [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L31) |
| `RFC8277-2.1-15` | If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the premise is both speakers having sent the capability, which ze never sends, so ze never applies the per-AFI/SAFI fallback to Section 2.2 on that ground. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-2.5-1` | If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L96). **positive:** `unit/verify` [`TestRFC8277VPNRelabelReplacesTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L72). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L97) |
| `RFC8277-2.5-2` | If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. UPDATE U1 is implicitly withdrawn. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L142). **positive:** `unit/verify` [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L199). **positive:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L34). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L143). **negative:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L35) |
| `RFC8277-2.5-3` | If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L198). **positive:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L33). **negative:** `unit/verify` [`TestRFC8277AddPathSameIdentifierRebinds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L245) |
| `RFC8277-3.1-1` | These two routes MUST be considered to be comparable, even if they specify different labels. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestAddPathOpaqueMixedModeMeetInOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/addpath_opaque_best_per_route_test.go#L117). **positive:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L283). **positive:** `unit/verify` [`TestRFC8277AddPathRoutesOnOneSessionAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go#L47). **positive:** `unit/verify` [`TestRFC8277VPNAddPathLabelsAreOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_addpath_test.go#L20). **negative:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L284) |
| `RFC8277-3.2.1-1` | When a SAFI-4 or SAFI-128 route is propagated, if the Network Address of Next Hop field is left unchanged, the Label field(s) MUST also be left unchanged. (§3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L34). **positive:** `unit/verify` [`TestLabeledVPNPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_vpn_propagation_test.go#L30). **negative:** no negative test. **{single-polarity}:** an unchanged next hop produces no next-hop modification op (internal/component/bgp/reactor/peer_forward_facts.go::precomputeNextHop and internal/component/bgp/reactor/peer_forward_facts.go::applyFactsNextHop) and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler copies the source attribute verbatim when no Set op reaches it, so the labels are preserved by construction and there is no non-conformant propagation input to reject |
| `RFC8277-3.2.1-2` | Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-3.2.1-3` | Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze stores no peer Count, so no propagation is suppressed for a route carrying more labels than the peer announced. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| `RFC8277-3.2.1-4` | In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| `RFC8277-3.2.2-1` | If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| `RFC8277-3.2.2-2` | A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2) | MUST NOT | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-3.2.2-3` | It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the changed-next-hop label-count decision and its paired withdrawal are not implemented. internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler replaces the next hop and retains the entire NLRI without a recipient label-count decision; internal/component/bgp/message/update_build_labeled.go::BuildLabeledUnicastNLRIBytes serializes the supplied stack without such a decision. No Multiple Labels Capability count is available. The unchanged-next-hop withdrawal gap remains separately recorded in RFC8277-3.2.1-4; neither branch is counted conformant. Recording this absent capability does not authorize its implementation. |
| `RFC8277-2.2-4` | Rsrv: This 3-bit field SHOULD be set to zero on transmission (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.4-2` | Upon transmission, the Compatibility field SHOULD be set to 0x800000. (§2.4) | SHOULD | 2.4 | **positive:** `unit/verify` [`TestWithdrawalCompatibilityEveryWriter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L139). **positive:** `unit/verify` [`TestWithdrawalCompatibilityOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L43). **negative:** `unit/verify` [`TestWithdrawalCompatibilityOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L44) |
| `RFC8277-2.1-16` | A BGP speaker SHOULD NOT send an UPDATE that binds more labels to a given prefix than its peer is capable of receiving, as specified in the Multiple Labels Capability sent by that peer. (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-4` | BGP speaker S1 SHOULD treat this as an indication that N1 has at least two paths to P (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-5` | S1 MAY use this fact to do load-balancing of any traffic that it has to send to P. (§2.5) | MAY | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-5-1` | In this case, the BGP speaker MAY convert the SAFI-4 route to a SAFI-1 route and then propagate the result over the session on which SAFI-4 is not enabled. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-4-1` | It is a matter of local policy whether SAFI-4 routes can be used as the basis for forwarding IP packets or whether SAFI-4 routes can only be used for forwarding MPLS packets. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8277-2-1`](#rfc8277-2-1) Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2) | {gap}, no test | ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-1`](#rfc8277-2.1-1) If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1) | {gap}, no test | no session can meet the three conditions, so ze never selects the Section 2.3 encoding through a capability exchange. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-2`](#rfc8277-2.1-2) If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1) | {gap}, no test | no session can meet the three conditions, so ze never selects the Section 2.3 encoding through a capability exchange, for one label or several. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2-3`](#rfc8277-2-3) If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1) | {gap}, no test | with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-3`](#rfc8277-2.1-3) Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1) | {gap}, no test | ze sends no Multiple Labels Capability, so the two-label support owed by its sender is owed when the capability lands. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-4`](#rfc8277-2.1-4) A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1) | {gap}, no test | ze holds no parser that could judge a received Multiple Labels Capability malformed by its length. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-5`](#rfc8277-2.1-5) A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1) | {gap}, no test | ze writes no AFI/SAFI/Count triple. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-6`](#rfc8277-2.1-6) A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1) | {gap}, no test | ze decodes no triple, so no Count of 0 or 1 is read and ignored by a Count reader. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-7`](#rfc8277-2.1-7) If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1) | {gap}, no test | ze holds no per-AFI/SAFI triple state, so no first triple is kept over a later one. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-8`](#rfc8277-2.1-8) If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1) | {gap}, no test | ze stores no Multiple Labels Capability, so no first-copy rule exists. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-9`](#rfc8277-2.1-9) If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1) | {gap}, no test | ze announces no Count, so no UPDATE is judged against one and treat-as-withdraw is never applied for an excess label count. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-10`](#rfc8277-2.1-10) Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1) | {gap}, no test | the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| [`RFC8277-2.1-11`](#rfc8277-2.1-11) If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1) | {gap}, no test | ze tracks no Multiple Labels Capability across a restart, so no multi-label prefix is withdrawn when it is lost. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-12`](#rfc8277-2.1-12) Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1) | {gap}, no test | ze tracks no per-peer maximum label count, so no prefix is withdrawn when the count falls. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-13`](#rfc8277-2.1-13) If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1) | {gap}, no test | ze tracks neither condition, so no full re-exchange is triggered. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.1-14`](#rfc8277-2.1-14) "Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1) | {gap}, no test | no code reads a Multiple Labels Capability or Count change, the graceful restart plugin (internal/component/bgp/plugins/gr) included, so no restart procedure is guarded by one. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-2.2-2`](#rfc8277-2.2-2) S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2) | {gap}, no test | the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| [`RFC8277-2.1-15`](#rfc8277-2.1-15) If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1) | {gap}, no test | the premise is both speakers having sent the capability, which ze never sends, so ze never applies the per-AFI/SAFI fallback to Section 2.2 on that ground. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2) Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1) | {gap}, no test | the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3) Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1) | {gap}, no test | ze stores no peer Count, so no propagation is suppressed for a route carrying more labels than the peer announced. ze implements no Multiple Labels Capability: internal/core/bgp/capability/capability.go declares no capability code 8, so ze never sends it and a received one meets the unknown-capability path. No owner decision declines the capability, so this is owed with it |
| [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4) In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1) | {gap}, no test | the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1) If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2) | {gap}, no test | ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2) A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2) | {gap}, no test | no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3) It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2) | {gap}, no test | the changed-next-hop label-count decision and its paired withdrawal are not implemented. internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler replaces the next hop and retains the entire NLRI without a recipient label-count decision; internal/component/bgp/message/update_build_labeled.go::BuildLabeledUnicastNLRIBytes serializes the supplied stack without such a decision. No Multiple Labels Capability count is available. The unchanged-next-hop withdrawal gap remains separately recorded in RFC8277-3.2.1-4; neither branch is counted conformant. Recording this absent capability does not authorize its implementation. |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8277-2-1`](#rfc8277-2-1)

Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2-1, so there is no linked test.

### [`RFC8277-2-2`](#rfc8277-2-2)

Further, when advertising the binding of a single label to a prefix, the BGP speaker MUST use the encoding specified in Section 2.2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a single-label binding transmitted in any layout other than the Section 2.2 one. All three transmitters now covered with whole-NLRI literals: LabeledUnicast.WriteTo by TestLabeledSingleLabelSection22Encoding (HEAD, require.Equal on the whole encoding), and UpdateBuilder.BuildLabeledUnicastNLRIBytes (plain and ADD-PATH) and buildVPNNLRIBytes by TestRFC8277BuilderLabelLayout (20 000641 0a, 00000007 20 000641 0a, 60 000641 <RD> 0a; recorded revert of WriteLabelValues; judge overlay dropping the S line of WriteLabelValues reds every case). {single-polarity: positive} is valid: a one-label stack has one layout and Length is computed from the input, so no input reaches a non-Section-2.2 encoding; the reason now names all three transmitters (correction paragraph in rfc/corrections/rfc8277.md). Residual: the HEAD WriteTo unit has no discrimination record of its own.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33) | unit/verify | not verified |

### [`RFC8277-2.1-1`](#rfc8277-2.1-1)

If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-1, so there is no linked test.

### [`RFC8277-2.1-2`](#rfc8277-2.1-2)

If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-2, so there is no linked test.

### [`RFC8277-2-3`](#rfc8277-2-3)

If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2-3, so there is no linked test.

### [`RFC8277-2.1-3`](#rfc8277-2.1-3)

Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-3, so there is no linked test.

### [`RFC8277-2.1-4`](#rfc8277-2.1-4)

A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-4, so there is no linked test.

### [`RFC8277-2.1-5`](#rfc8277-2.1-5)

A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-5, so there is no linked test.

### [`RFC8277-2.1-6`](#rfc8277-2.1-6)

A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-6, so there is no linked test.

### [`RFC8277-2.1-7`](#rfc8277-2.1-7)

If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-7, so there is no linked test.

### [`RFC8277-2.1-8`](#rfc8277-2.1-8)

If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-8, so there is no linked test.

### [`RFC8277-2.1-9`](#rfc8277-2.1-9)

If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-9, so there is no linked test.

### [`RFC8277-2.1-10`](#rfc8277-2.1-10)

Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-10, so there is no linked test.

### [`RFC8277-2.1-11`](#rfc8277-2.1-11)

If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-11, so there is no linked test.

### [`RFC8277-2.1-12`](#rfc8277-2.1-12)

Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-12, so there is no linked test.

### [`RFC8277-2.1-13`](#rfc8277-2.1-13)

If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-13, so there is no linked test.

### [`RFC8277-2.1-14`](#rfc8277-2.1-14)

"Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-14, so there is no linked test.

### [`RFC8277-2.2-1`](#rfc8277-2.2-1)

S: This 1-bit field MUST be set to one on transmission (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a single label entry transmitted with S=0. Positive: TestRFC8277BuilderLabelLayout pins the one-label entry 000641 through both WriteLabelValues builders (labeled plain/ADD-PATH, VPN) as whole-NLRI literals, plus HEAD TestLabeledSingleLabelSection22Encoding for WriteTo. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten hands WriteTo a lone entry 0x000640 with S=0, the relayed-entry input, and asserts 20 000641 0a. Judge overlays: dropping 'entry |= 1' in WriteLabelStack reds the negative (200006400a); dropping the S line of WriteLabelValues reds all five builder cases. The {single-polarity} marker is dropped (correction paragraph), records on both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34) | unit/verify | not verified |

### [`RFC8277-2.2-2`](#rfc8277-2.2-2)

S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.2-2, so there is no linked test.

### [`RFC8277-2.2-3`](#rfc8277-2.2-3)

Rsrv: This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c29 judge), owner ruling 8(d). Reception (MUST ignore) unchanged from c28: TestLabeledRsrvIgnoredOnReceive and TestRFC8277RsrvIgnoredOnReceiveLabelStack. Transmission (SHOULD zero) now enforced for both originated and relayed stacks. Originated: TestRFC8277BuilderLabelLayout (WriteLabelValues, Rsrv 000). Relayed: clearLabelRsrv runs in publishBase, the ingest step every announcement passes before the RIB and both forward rails see it; SAFI 4 and 128 only, read-only scan first, copy only on a hit. Negatives TestRFC8277RsrvClearedOnRelay (Rsrv 111/101/001/110 to 000 over one label, a three-label stack then a second NLRI, ADD-PATH path id, SAFI 128 RD; framing and the received buffer unchanged) and TestRFC8277RsrvClearedOnTheReceivePath (through enforceRFC7606, no RFC 7606 action). Positive TestRFC8277ZeroRsrvRelayedZeroCopy asserts the same WireUpdate for a zero-Rsrv stack and a unicast NLRI. No bypass: the only non-publishBase exits of enforceRFC7606 are treat-as-withdraw (withdrawals carry the Section 2.4 Compatibility field, not a label stack) and session reset; originated stacks come from LabelEntriesFor (Rsrv 0). EVPN (SAFI 70) untouched. Three observed-red records (revert clearLabelRsrv).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277RsrvClearedOnRelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L67) | unit/verify | revert, verified |
| negative | [`TestRFC8277RsrvClearedOnTheReceivePath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L24) | unit/verify | not verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC8277ZeroRsrvRelayedZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L23) | unit/verify | not verified |

### [`RFC8277-2.3-1`](#rfc8277-2.3-1)

In all labels except the last (i.e., in all labels except the one immediately preceding the prefix), the S bit MUST be 0. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a non-last label entry transmitted with S=1. Positive: TestRFC8277BuilderLabelLayout pins 50 000640 000c80 0012c1 0a and 78 000640 000c81 <RD> 0a through WriteLabelValues; HEAD TestLabeledLabelStackSBitPlacement covers WriteTo built from values. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten hands WriteTo entries 0x000641, 0x000c81, 0x0012c0 (S=1 mid-stack) and asserts the S bits are cleared. Judge overlay dropping 'entry &^= 1' in WriteLabelStack reds the negative only (50000641000c810012c10a) while the HEAD placement test stays green, so the new unit is what discriminates the relay path. Marker dropped, records on both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77) | unit/verify | not verified |

### [`RFC8277-2.3-2`](#rfc8277-2.3-2)

In the last label, the S bit MUST be 1. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: the last label entry transmitted with S=0. Positive: the builder stacks in TestRFC8277BuilderLabelLayout end 0012c1 and 000c81 (WriteLabelValues), HEAD TestLabeledLabelStackSBitPlacement for WriteTo. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten gives the last entry 0x0012c0 (S=0) and the lone entry 0x000640, and asserts S=1 is transmitted. Judge overlay dropping 'entry |= 1' reds both subcases; dropping the WriteLabelValues S line reds both builder stacks. The negative shares its stack buffer with 2.3-1, but each break is isolated to its own octet and the whole NLRI is compared. Marker dropped, records on both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78) | unit/verify | not verified |

### [`RFC8277-2.4-1`](#rfc8277-2.4-1)

Upon reception, the value of the Compatibility field MUST be ignored. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc8277): "Upon reception, the value of the Compatibility field MUST be ignored." a0589dd1d1 changed only the stub bridge.SetUpdateRouteSel callback's signature in TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes to accept the new trailing uint64 (source policy epoch), which the stub ignores. No input or assertion changed. Verdict unchanged: enforced. Earlier judgements are in this file's git history. Independent current-source rejudgment for minimal forwarding-feasibility fixture prerequisites: all 29 existing tests/units keys and all five code subjects were read and are retained. Labeled compatibility matrices still cover IPv4/IPv6, with/without ADD-PATH, zero/17 identifiers, prefix/RD siblings, exact native survivors and replacement identity; VPN promotion retains both its AFI/path matrices and original two compatibility loops. The changed fixture only supplies connected terminal routes for the exact advertised next hops, leaving every original announcement, withdrawal and assertion intact. The JSON/codec/SDK transport, route-server and reflector populations retain their separate native identity assertions. Verdict remains enforced. Existing revert records demonstrate producer reachability only; renewal must preserve their routes/producers and does not establish a normal-return semantic mutation or destination TCP proof. No new execution or red is claimed by this pending judgment.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L34) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L77) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rfc8277_rpc_decode_nlri_context_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalEventNamesThePrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalEventNeverReachesTheLabelDecoder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rfc8277_rpc_decode_nlri_context_test.go#L26) | unit/verify | revert, verified |

### [`RFC8277-2.1-15`](#rfc8277-2.1-15)

If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-2.1-15, so there is no linked test.

### [`RFC8277-2.5-1`](#rfc8277-2.5-1)

If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8277 Section 2.5: "If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2." Read the full same-session, same-prefix, same-next-hop premise and both tagged carriers. TestLabeledImplicitWithdrawalNoAddPath positively requires exactly one received route and labels [200] after replacing [100]; its negative sends another prefix, requires two routes and the original prefix still bound to [200]. TestRFC8277VPNRelabelReplacesTheRoute sends SAFI-128 with fixed peer, RD, prefix, next hop and attributes, changing only label 100 to 200; it requires one stored route, lookup by the old labeled NLRI, an iteration returning only the new NLRI, and a second best-change naming that new NLRI. The post-lint vpnv4NLRI has removed only the constant /8 argument: length remains 96 and label/RD/prefix bytes remain unchanged. RouteKey strips label identity, insertOpaqueNoOp refuses the old-wire shortcut on relabel, and insertOpaque/setRouteNLRI stores the latest wire NLRI. Distinct same-prefix/different-prefix controls isolate implicit replacement and current assertions would fail on old-label retention or duplicate bindings. Existing enforced judgment is preserved after current source rejudgment; renew the existing VPN RouteKey discrimination record natively. This is no claim of generic VPN RS/RR withdrawal correctness; RFC8277-2.4-1 remains weak. Independent current-source rejudgment retains all three existing tests/units keys and the enforced verdict. The SAFI-4 same-session/prefix/next-hop label-100 to label-200 replacement and different-prefix control remain unchanged. The VPN carrier adds only the connected 10.0.0.1 prerequisite; it still requires one stored route, lookup through old-label identity, iteration carrying only the new label, and the second published best change naming the relabeled NLRI. RouteKey continues to omit label identity, while received storage and publication preserve the latest native route. The old note prefix saying RFC8277-2.4-1 remains weak is historical: that separate file currently records enforced, and this judgment does not alter it. Renew the existing RouteKey revert tuple with its reachability-only limitation. No normal-return mutation or fresh observation is claimed here.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L97) | unit/verify | not verified |
| positive | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L96) | unit/verify | not verified |
| positive | [`TestRFC8277VPNRelabelReplacesTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L72) | unit/verify | revert, verified |

### [`RFC8277-2.5-2`](#rfc8277-2.5-2)

If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. UPDATE U1 is implicitly withdrawn. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/rib/rfc8277_test.go::TestLabeledImplicitWithdrawalAddPath, internal/component/bgp/plugins/rib/rfc8277_test.go::TestLabeledImplicitWithdrawalAddPath#2, internal/component/bgp/plugins/rib/rfc8277_test.go::TestRFC8277AddPathLabelsBoundPerPath moved. Independent post-edit judgment by AddPathProofReview, not the test author. RFC 8277 Section 2.5: "If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1." TestRFC8277AddPathSameNextHopKeepsIndependentLabels holds peer, prefix, next hop and MED fixed; a reused path 9 replaces [200] by exactly [300], leaves exactly two routes and path 7 at [100]. Its negative boundary uses a different identifier, retaining both routes and both labels. Existing TestLabeledImplicitWithdrawalAddPath and TestRFC8277AddPathLabelsBoundPerPath retain route-entry and binding isolation coverage. pathSet.setLabels binds only the named entry; the parent observed the new test fail with [200] instead of [300] when that producer retained the old handle, and pass in the complete RIB race run. Native records separately observe producer disabling; no generic best-path compliance is inferred.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L143) | unit/verify | not verified |
| positive | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L142) | unit/verify | not verified |
| positive | [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L199) | unit/verify | revert, verified |

### [`RFC8277-2.5-3`](#rfc8277-2.5-3)

If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/rib/rfc8277_test.go::TestRFC8277AddPathLabelsBoundPerPath, internal/component/bgp/plugins/rib/rfc8277_test.go::TestRFC8277AddPathSameIdentifierRebinds moved. Independent post-edit judgment by AddPathProofReview, not the test author. RFC 8277 Section 2.5: "If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1." The new positive TestRFC8277AddPathSameNextHopKeepsIndependentLabels holds the same-next-hop premise that the earlier different-next-hop test did not isolate: path 9 binds [200] while path 7 keeps [100], including across relabel and targeted withdrawal. TestRFC8277AddPathSameIdentifierRebinds supplies the negative boundary: the SAME path 7 leaves one route with [200] alone. Both reach received UPDATE ingestion and per-path label storage. Parent observed the new positive fail with path 7 reading [200] when setLabels bound every update to the first entry; normal complete RIB race run passed. This closes both the formerly absent negative tag and the same-next-hop proof gap.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8277AddPathSameIdentifierRebinds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L245) | unit/verify | revert, verified |
| positive | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L198) | unit/verify | revert, verified |

### [`RFC8277-3.1-1`](#rfc8277-3.1-1)

These two routes MUST be considered to be comparable, even if they specify different labels. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudgment of all four current tagged functions and five polarity covers against the complete RFC 8277 Section 3.1. The RFC states: These two routes MUST be considered to be comparable, even if they specify different labels. Its premise includes different BGP sessions or one ADD-PATH session with different path identifiers, and it states: The same applies to SAFI-128 routes. Read the current structured-receive/storage fixtures and the real CIDR/opaque route-identity, retained candidate aggregation, admission, best-path selection and stored-winner producers. TestLabeledRoutesWithDifferentLabelsAreComparable provides two iBGP session identities, two real independent connected host routes in a private Loc-RIB and the resolver over that same table, so neither received next hop depends on the advertised /8. Its unchanged original inputs require exactly two candidates, peer A/MED10 against the smaller-peer-address fallback and one best record. Its separate negative swaps only labels and requires the same peer, MED10, one best record and the winner's own new label200. The current positive semantic break skips MED elimination and fails the initial winner; the current negative semantic break incorrectly derives preference from the label and passes the initial arrangement before failing only the post-swap peer/MED assertions. TestRFC8277AddPathRoutesOnOneSessionAreComparable uses the same genuine reachability prerequisite, preserves labels100/200 and IDs7/9 with MED20/10, requires two stored paths and two candidates whichever ID-bearing key triggers the lookup, and elects path9/MED10 against path-ID fallback. Its current semantic label-preference break produces path7/MED20 from both keys after a clean pass. TestRFC8277VPNAddPathLabelsAreOneElection separately requires both differently labeled VPN paths for one RD/prefix from either ADD-PATH framing, path7/MED10 with its exact own label-bearing framed NLRI, replacement on relabel and path-specific Compatibility withdrawal. Its winner aligns with the lower-path-ID fallback, so it is not independently credited as a MED-bypass discriminator. Its freshly observed producer-panic record establishes reachability of the opaque gather, not an independent semantic label/selection fault. TestAddPathOpaqueMixedModeMeetInOneElection now supplies the distinct VPN cross-session aggregation boundary: plain and ADD-PATH peers for the same RD/prefix must both appear from both trigger framings, the plain LOCAL_PREF500 route wins with its own unframed NLRI, both triggers reach the same election and exactly one best record remains. Its current native semantic break stops opaque aggregation after the first contributing peer, before retaining another peer's paths; the clean unit passes and the mutated VPN sibling fails both exact candidate counts, observing one instead of two. This failure is independent of map order. The EVPN sibling is outside this RFC claim. Together the opaque same-session label-identity assertions and cross-session aggregation assertions cover the two SAFI-128 comparability premises without treating CIDR aggregation as evidence for the distinct opaque loop. These are bounded in-process comparison and identity proofs, not live VPN peer interoperability, all RFC4271 ranking criteria, send-path propagation, kernel/MPLS forwarding, VPP acceptance or full RFC8277 support. No test, expectation or original vector was removed or weakened.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L284) | unit/verify | mutant, verified |
| positive | [`TestAddPathOpaqueMixedModeMeetInOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/addpath_opaque_best_per_route_test.go#L117) | unit/verify | mutant, verified |
| positive | [`TestRFC8277AddPathRoutesOnOneSessionAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go#L47) | unit/verify | mutant, verified |
| positive | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L283) | unit/verify | mutant, verified |
| positive | [`TestRFC8277VPNAddPathLabelsAreOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_addpath_test.go#L20) | unit/verify | revert, verified |

### [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1)

When a SAFI-4 or SAFI-128 route is propagated, if the Network Address of Next Hop field is left unchanged, the Label field(s) MUST also be left unchanged. (§3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudge after replacing the test-only applyNextHopMod helper with the live precomputeNextHop and applyFactsNextHop producers. RFC 8277 Section 3.2.1 says: When a SAFI-4 or SAFI-128 route is propagated, if the Network Address of Next Hop field is left unchanged, the Label field(s) MUST also be left unchanged. TestLabeledPropagationUnchangedNextHopKeepsLabels retains its SAFI-4 source bytes, empty modification-set assertion, complete MP_REACH byte equality, label/prefix equality and S-bit assertion. TestLabeledVPNPropagationUnchangedNextHopKeepsLabels retains its SAFI-128 source bytes, empty modification-set assertion, complete attribute equality, twelve-octet VPN next hop, label, Route Distinguisher and prefix assertions. precomputeNextHop selects nhModeNone for NextHopUnchanged, applyFactsNextHop records no operation for that mode, and mpReachNextHopHandler takes its unchanged-source keepOrDrop branch when neither a next-hop Set nor an NLRI replacement exists. These assertions would detect loss or alteration of the preserved source attribute; they do not merely assert successful construction. The positive-only annotation still describes this conditional preservation obligation correctly and now cites the live producers without obsolete line numbers. Changed-next-hop behavior belongs to Section 3.2.2 and is not claimed by these tests. This rejudge is by source inspection, not a claim that the newly owed mutant observations or an interoperability scenario ran. Retain the enforced verdict with no scope expansion; the main thread must refresh the changed units' native discrimination observations before committing them.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L34) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLabeledVPNPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_vpn_propagation_test.go#L30) | unit/verify | mutant, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2)

Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-3.2.1-2, so there is no linked test.

### [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3)

Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-3.2.1-3, so there is no linked test.

### [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4)

In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-3.2.1-4, so there is no linked test.

### [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1)

If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-3.2.2-1, so there is no linked test.

### [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2)

A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8277-3.2.2-2, so there is no linked test.

### [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3)

It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent AC12 rejudgment on 2026-11-16 against RFC8277 Sections3.2.1 and3.2.2, including the fewer-label previous-route withdrawal condition: mpReachNextHopHandler now delegates width admission to the shared next-hop profile but still retains the entire NLRI when only replacing a next hop. BuildLabeledUnicastNLRIBytes serializes all supplied labels. The capability Code declarations still omit8 and no negotiated Multiple Labels Count is available to this egress decision. There is no changed-next-hop recipient label-count refusal and therefore no associated withdrawal of an earlier fewer-label route. Opaque route identity/implicit-replacement support does not implement this propagation-conditioned withdrawal. No carrier tags this row. Retain unimplemented and the explicit gap; no partial or whole credit and no automatic scope authorization follow. No run or proof replay performed.

No test carries RFC8277-3.2.2-3, so there is no linked test.

### [`RFC8277-2.4-2`](#rfc8277-2.4-2)

Upon transmission, the Compatibility field SHOULD be set to 0x800000. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestWithdrawalCompatibilityOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L44) | unit/verify | mutant, verified |
| positive | [`TestWithdrawalCompatibilityEveryWriter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestWithdrawalCompatibilityOnSend`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_withdrawal_compatibility_test.go#L43) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8277.txt |
| Source fingerprint | 570bb26257004cc5 |
| Record | rfc/extraction/rfc8277.json |
| Mapped sentences | 34 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `2.1` | not stated | 17 | reviewed | not stated |
| `2.2` | not stated | 2 | reviewed | not stated |
| `2.3` | not stated | 3 | reviewed | not stated |
| `2.4` | not stated | 2 | reviewed | not stated |
| `2.5` | not stated | 3 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 1 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.2.1` | not stated | 4 | reviewed | not stated |
| `3.2.2` | not stated | 3 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Section 2 overview bullet announcing the Section 2.3 encoding rule that Section 2.1 states in full; site 2.1:13 carries it. The bullet's other half, "MAY bind a prefix to a sequence of more than one label", is the permission the same row's condition already grants. | o If this Capability is sent by both BGP speakers on a given session, an UPDATE message on that session, from either speaker, MUST use the encoding of Section 2.3 and MAY bind a prefix to a sequence of more than one label. |
| `2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.3 repeats the Section 2.2 Rsrv sentence word for word for each stack entry. RFC8277-2.2-3 retains the full receive/transmit sentence; the dated Correction 2026-10-06 identifies its existing multi-label receive, origination and relay tests. RFC8277-2.2-4 repeats only its transmit SHOULD. The rows cite Section 2.2; this site records the Section 2.3 duplicate without claiming otherwise. | This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. |
| `2.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 2.4 summary sentence restates the reception obligation site 2.4:1 states on its own ("Upon reception, the value of the Compatibility field MUST be ignored."), which maps RFC8277-2.4-1. Its RECOMMENDED half is row RFC8277-2.4-2 and is advisory. | In order to ensure backwards compatibility, it is RECOMMENDED by this document that the Compatibility field be set to 0x800000, but it is REQUIRED that it be ignored upon reception. |

## Superseded

No document obsoletes RFC 8277, so its obligations are stated where they were written.
