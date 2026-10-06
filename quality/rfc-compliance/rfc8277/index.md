# RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes

Partial. Every requirement this repository extracted from RFC 8277, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.7% | 9 of 35 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 5.7% | 2 of 35 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 35 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 35 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 81.4% | 48 of 59 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 35 | of 42 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 15 | of 35 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 42.9% | 15 of 35 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 35 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 35 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.7% | 9 of 35 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 12 | of 35 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 35 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 42 |
| Gated MUST-level | 35 |
| Not applicable, so out of scope | 15 |
| Declared gaps | 9 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 59 |
| Tagged units | 59 |
| Recorded audit verdicts | 12 |
| Discrimination records | 48 |
| Summary | `rfc/short/rfc8277.md` |
| Requirement shard | `rfc/requirements/rfc8277.md` |
| RFC text | `rfc/full/rfc8277.txt` |

## Enrolment

Enrolled: Using BGP to Bind MPLS Labels to Address Prefixes

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv4 and IPv6 labeled unicast NLRI (SAFI 4) encode and decode, label stack handling, ADD-PATH framing, route config, Adj-RIB-In label side-data, best-path comparability across differing labels, withdrawals that ignore the Compatibility field (labeled unicast and VPN, in the Adj-RIB-In and in the re-election that follows), and dataplane handoff
- requirements bound per line in [`rfc/short/rfc8277.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8277.md).


**What the ledger says remains**

MUST-level gaps are each annotated in [`rfc/short/rfc8277.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8277.md). Multiple Labels Capability (code 8) is absent from ze's capability set ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)), so every session is in the RFC 8277 Section 2 single-label mode while the encoders still accept and emit a full stack: [`RFC8277-2-1`](#rfc8277-2-1), [`RFC8277-2-3`](#rfc8277-2-3), [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2). Propagation of an already-multi-label route is unguarded for the same reason: [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2) -- the prohibition binds on every peer precisely because the capability is negotiated with none, and [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4) -- no propagation is ever blocked on label count, so the paired withdrawal has no producer either.

- **Encoding:** [`RFC8277-2.1-10`](#rfc8277-2.1-10) -- the one-octet NLRI Length is written as `byte(totalBits)` with no bound check ([`internal/component/bgp/plugins/nlri/labeled/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/types.go), [`internal/component/bgp/message/update_build_labeled.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_labeled.go)).
- **Reception:** [`RFC8277-2.2-2`](#rfc8277-2.2-2) -- the announcement's label-stack readers are S-bit-driven ([`internal/core/bgp/nlri/nlrisplit/labeled.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/labeled.go),:112), so a single-label NLRI with S clear is read past its prefix and rejected.
- **Propagation:** [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1) -- ze binds no local label, so a next-hop rewrite re-advertises the upstream label unchanged ([`internal/component/bgp/reactor/filter_delta_handlers.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter_delta_handlers.go)). [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3) separately records the absent withdrawal of an earlier fewer-label advertisement when the changed-next-hop label-stack decision declines propagation to that peer.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 26 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **35** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC8277-2.2-1`](#rfc8277-2.2-1), [`RFC8277-2.2-3`](#rfc8277-2.2-3), [`RFC8277-2.3-1`](#rfc8277-2.3-1), [`RFC8277-2.3-2`](#rfc8277-2.3-2), [`RFC8277-2.4-1`](#rfc8277-2.4-1), [`RFC8277-2.5-1`](#rfc8277-2.5-1), [`RFC8277-2.5-2`](#rfc8277-2.5-2), [`RFC8277-2.5-3`](#rfc8277-2.5-3), [`RFC8277-3.1-1`](#rfc8277-3.1-1)

**Annotated (including scoped evidence) (26):** [`RFC8277-2-1`](#rfc8277-2-1), [`RFC8277-2-2`](#rfc8277-2-2), [`RFC8277-2.1-1`](#rfc8277-2.1-1), [`RFC8277-2.1-2`](#rfc8277-2.1-2), [`RFC8277-2-3`](#rfc8277-2-3), [`RFC8277-2.1-3`](#rfc8277-2.1-3), [`RFC8277-2.1-4`](#rfc8277-2.1-4), [`RFC8277-2.1-5`](#rfc8277-2.1-5), [`RFC8277-2.1-6`](#rfc8277-2.1-6), [`RFC8277-2.1-7`](#rfc8277-2.1-7), [`RFC8277-2.1-8`](#rfc8277-2.1-8), [`RFC8277-2.1-9`](#rfc8277-2.1-9), [`RFC8277-2.1-10`](#rfc8277-2.1-10), [`RFC8277-2.1-11`](#rfc8277-2.1-11), [`RFC8277-2.1-12`](#rfc8277-2.1-12), [`RFC8277-2.1-13`](#rfc8277-2.1-13), [`RFC8277-2.1-14`](#rfc8277-2.1-14), [`RFC8277-2.2-2`](#rfc8277-2.2-2), [`RFC8277-2.1-15`](#rfc8277-2.1-15), [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1), [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2), [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3), [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4), [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1), [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2), [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8277-2-1` | Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2-2` | Further, when advertising the binding of a single label to a prefix, the BGP speaker MUST use the encoding specified in Section 2.2. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L28). **negative:** no negative test. **{single-polarity}:** all three transmitters emit the §2.2 layout unconditionally -- Length octet = 24 + prefix bits, one 3-octet label entry, prefix: LabeledUnicast.WriteTo (internal/component/bgp/plugins/nlri/labeled/types.go), UpdateBuilder.BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go) and buildVPNNLRIBytes (internal/component/bgp/message/update_build_vpn.go) -- so there is no non-conformant transmission input to reject |
| `RFC8277-2.1-1` | If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Multiple Labels Capability (code 8) producer -- the registered capability codes are 1, 2, 5, 6, 9, 64, 65, 69, 70, 73, 76 (internal/core/bgp/capability/capability.go:68-78) and `grep -rni "multiple.label\\\|CodeMultipleLabels" internal/ pkg/` matches no capability code, so no session ever reaches the Section 2.3 branch |
| `RFC8277-2.1-2` | If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same missing producer -- capability code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78), so the "capability exchanged" state never exists |
| `RFC8277-2-3` | If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2.1-3` | Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no Multiple Labels Capability -- the OPEN capability encoder writes only the codes listed at internal/core/bgp/capability/capability.go:68-78, which exclude code 8, so the obligation attached to sending it has no producer |
| `RFC8277-2.1-4` | A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no Multiple Labels Capability parser exists to length-check -- `grep -rni "multiple.label" internal/core/bgp/capability/` returns nothing and code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-5` | A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no AFI/SAFI/Count triples because it emits no capability code 8 (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-6` | A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no triple parser exists -- capability code 8 has no decoder (internal/core/bgp/capability/capability.go:68-78), so a received capability is handled by the RFC 5492 unknown-capability path rather than by a Count reader |
| `RFC8277-2.1-7` | If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no Multiple Labels Capability decoder exists to hold per-AFI/SAFI triples (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-8` | If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze stores no Multiple Labels Capability state, so no first-copy-wins rule has a producer (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-9` | If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze announces no Count because it announces no capability code 8 (internal/core/bgp/capability/capability.go:68-78), so the "more labels than announced Count" condition has no producer to evaluate it |
| `RFC8277-2.1-10` | Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| `RFC8277-2.1-11` | If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** capability code 8 is never negotiated (internal/core/bgp/capability/capability.go:68-78), so "capability lost on restart" is not a state ze can enter |
| `RFC8277-2.1-12` | Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze tracks no per-peer maximum label Count, since no capability code 8 decoder exists to supply one (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-13` | If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** neither the capability nor the Count exists in ze's session state (internal/core/bgp/capability/capability.go:68-78), so no change event can trigger the full re-exchange |
| `RFC8277-2.1-14` | "Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the guard condition is a Multiple Labels Capability or Count change, and neither is represented anywhere in ze -- `grep -rni "multiple.label" internal/component/bgp/plugins/gr/` returns nothing and code 8 is absent from internal/core/bgp/capability/capability.go:68-78 |
| `RFC8277-2.2-1` | S: This 1-bit field MUST be set to one on transmission (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L29). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L30) |
| `RFC8277-2.2-2` | S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| `RFC8277-2.2-3` | Rsrv: This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L23). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L32). **positive:** `unit/verify` [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L27). **positive:** `unit/verify` [`TestRFC8277ZeroRsrvRelayedZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L130). **negative:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L24). **negative:** `unit/verify` [`TestRFC8277RsrvClearedOnRelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L67). **negative:** `unit/verify` [`TestRFC8277RsrvClearedOnTheReceivePath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L112). **negative:** `unit/verify` [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L28) |
| `RFC8277-2.3-1` | In all labels except the last (i.e., in all labels except the one immediately preceding the prefix), the S bit MUST be 0. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L30). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L31) |
| `RFC8277-2.3-2` | In the last label, the S bit MUST be 1. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78). **positive:** `unit/verify` [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L31). **negative:** `unit/verify` [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L32) |
| `RFC8277-2.4-1` | Upon reception, the value of the Compatibility field MUST be ignored. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L27). **positive:** `unit/verify` [`TestLabeledWithdrawalEventNamesThePrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L70). **positive:** `unit/verify` [`TestLabeledWithdrawalEventNeverReachesTheLabelDecoder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L150). **positive:** `unit/verify` [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L71). **positive:** `unit/verify` [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L56). **positive:** `unit/verify` [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L97). **positive:** `unit/verify` [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L52). **positive:** `unit/verify` [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rpc_decode_nlri_context_test.go#L26). **positive:** `unit/verify` [`TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L152). **positive:** `unit/verify` [`TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L155). **positive:** `unit/verify` [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L117). **positive:** `unit/verify` [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L24). **positive:** `unit/verify` [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L26). **positive:** `unit/verify` [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L32). **positive:** `unit/verify` [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L36). **positive:** `unit/verify` [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L30). **negative:** `unit/verify` [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L28). **negative:** `unit/verify` [`TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L95). **negative:** `unit/verify` [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rpc_decode_nlri_context_test.go#L27). **negative:** `unit/verify` [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L27). **negative:** `unit/verify` [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L33). **negative:** `unit/verify` [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L37). **negative:** `unit/verify` [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L31) |
| `RFC8277-2.1-15` | If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the premise is that both speakers sent capability code 8, which ze never sends or parses (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.5-1` | If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L93). **positive:** `unit/verify` [`TestRFC8277VPNRelabelReplacesTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L70). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L94) |
| `RFC8277-2.5-2` | If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. UPDATE U1 is implicitly withdrawn. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L139). **positive:** `unit/verify` [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L196). **positive:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L34). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L140). **negative:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L35) |
| `RFC8277-2.5-3` | If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L195). **positive:** `unit/verify` [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L33). **negative:** `unit/verify` [`TestRFC8277AddPathSameIdentifierRebinds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L242) |
| `RFC8277-3.1-1` | These two routes MUST be considered to be comparable, even if they specify different labels. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L280). **positive:** `unit/verify` [`TestRFC8277AddPathRoutesOnOneSessionAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go#L44). **positive:** `unit/verify` [`TestRFC8277VPNAddPathLabelsAreOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_addpath_test.go#L20). **negative:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L281) |
| `RFC8277-3.2.1-1` | When a SAFI-4 or SAFI-128 route is propagated, if the Network Address of Next Hop field is left unchanged, the Label field(s) MUST also be left unchanged. (§3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L33). **positive:** `unit/verify` [`TestLabeledVPNPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_vpn_propagation_test.go#L29). **negative:** no negative test. **{single-polarity}:** an unchanged next hop produces no next-hop modification op (applyNextHopMod, internal/component/bgp/reactor/reactor_api_forward.go:815) and mpReachNextHopHandler copies the source attribute verbatim when no Set op reaches it (internal/component/bgp/reactor/filter_delta_handlers.go:252), so the labels are preserved by construction and there is no non-conformant propagation input to reject |
| `RFC8277-3.2.1-2` | Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-3.2.1-3` | Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no peer Count is stored because no capability code 8 decoder exists (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-3.2.1-4` | In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| `RFC8277-3.2.2-1` | If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| `RFC8277-3.2.2-2` | A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2) | MUST NOT | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-3.2.2-3` | It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the changed-next-hop label-count decision and its paired withdrawal are not implemented. internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler replaces the next hop and retains the entire NLRI without a recipient label-count decision; internal/component/bgp/message/update_build_labeled.go::BuildLabeledUnicastNLRIBytes serializes the supplied stack without such a decision. No Multiple Labels Capability count is available. The unchanged-next-hop withdrawal gap remains separately recorded in RFC8277-3.2.1-4; neither branch is counted conformant. Recording this absent capability does not authorize its implementation. |
| `RFC8277-2.2-4` | Rsrv: This 3-bit field SHOULD be set to zero on transmission (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.4-2` | Upon transmission, the Compatibility field SHOULD be set to 0x800000. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.1-16` | A BGP speaker SHOULD NOT send an UPDATE that binds more labels to a given prefix than its peer is capable of receiving, as specified in the Multiple Labels Capability sent by that peer. (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-4` | BGP speaker S1 SHOULD treat this as an indication that N1 has at least two paths to P (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-5` | S1 MAY use this fact to do load-balancing of any traffic that it has to send to P. (§2.5) | MAY | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-5-1` | In this case, the BGP speaker MAY convert the SAFI-4 route to a SAFI-1 route and then propagate the result over the session on which SAFI-4 is not enabled. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-4-1` | It is a matter of local policy whether SAFI-4 routes can be used as the basis for forwarding IP packets or whether SAFI-4 routes can only be used for forwarding MPLS packets. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8277-2-1`](#rfc8277-2-1) Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2) | {gap}, no test | ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-1`](#rfc8277-2.1-1) If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Multiple Labels Capability (code 8) producer -- the registered capability codes are 1, 2, 5, 6, 9, 64, 65, 69, 70, 73, 76 (internal/core/bgp/capability/capability.go:68-78) and `grep -rni "multiple.label\\\|CodeMultipleLabels" internal/ pkg/` matches no capability code, so no session ever reaches the Section 2.3 branch |
| [`RFC8277-2.1-2`](#rfc8277-2.1-2) If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: same missing producer -- capability code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78), so the "capability exchanged" state never exists |
| [`RFC8277-2-3`](#rfc8277-2-3) If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1) | {gap}, no test | with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-3`](#rfc8277-2.1-3) Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no Multiple Labels Capability -- the OPEN capability encoder writes only the codes listed at internal/core/bgp/capability/capability.go:68-78, which exclude code 8, so the obligation attached to sending it has no producer |
| [`RFC8277-2.1-4`](#rfc8277-2.1-4) A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no Multiple Labels Capability parser exists to length-check -- `grep -rni "multiple.label" internal/core/bgp/capability/` returns nothing and code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-5`](#rfc8277-2.1-5) A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no AFI/SAFI/Count triples because it emits no capability code 8 (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-6`](#rfc8277-2.1-6) A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no triple parser exists -- capability code 8 has no decoder (internal/core/bgp/capability/capability.go:68-78), so a received capability is handled by the RFC 5492 unknown-capability path rather than by a Count reader |
| [`RFC8277-2.1-7`](#rfc8277-2.1-7) If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no Multiple Labels Capability decoder exists to hold per-AFI/SAFI triples (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-8`](#rfc8277-2.1-8) If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze stores no Multiple Labels Capability state, so no first-copy-wins rule has a producer (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-9`](#rfc8277-2.1-9) If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze announces no Count because it announces no capability code 8 (internal/core/bgp/capability/capability.go:68-78), so the "more labels than announced Count" condition has no producer to evaluate it |
| [`RFC8277-2.1-10`](#rfc8277-2.1-10) Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1) | {gap}, no test | the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| [`RFC8277-2.1-11`](#rfc8277-2.1-11) If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: capability code 8 is never negotiated (internal/core/bgp/capability/capability.go:68-78), so "capability lost on restart" is not a state ze can enter |
| [`RFC8277-2.1-12`](#rfc8277-2.1-12) Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze tracks no per-peer maximum label Count, since no capability code 8 decoder exists to supply one (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-13`](#rfc8277-2.1-13) If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: neither the capability nor the Count exists in ze's session state (internal/core/bgp/capability/capability.go:68-78), so no change event can trigger the full re-exchange |
| [`RFC8277-2.1-14`](#rfc8277-2.1-14) "Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the guard condition is a Multiple Labels Capability or Count change, and neither is represented anywhere in ze -- `grep -rni "multiple.label" internal/component/bgp/plugins/gr/` returns nothing and code 8 is absent from internal/core/bgp/capability/capability.go:68-78 |
| [`RFC8277-2.2-2`](#rfc8277-2.2-2) S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2) | {gap}, no test | the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| [`RFC8277-2.1-15`](#rfc8277-2.1-15) If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the premise is that both speakers sent capability code 8, which ze never sends or parses (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2) Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1) | {gap}, no test | the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3) Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no peer Count is stored because no capability code 8 decoder exists (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4) In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1) | {gap}, no test | the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1) If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2) | {gap}, no test | ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2) A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2) | {gap}, no test | no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3) It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2) | {gap}, no test | the changed-next-hop label-count decision and its paired withdrawal are not implemented. internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler replaces the next hop and retains the entire NLRI without a recipient label-count decision; internal/component/bgp/message/update_build_labeled.go::BuildLabeledUnicastNLRIBytes serializes the supplied stack without such a decision. No Multiple Labels Capability count is available. The unchanged-next-hop withdrawal gap remains separately recorded in RFC8277-3.2.1-4; neither branch is counted conformant. Recording this absent capability does not authorize its implementation. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8277-2-1`](#rfc8277-2-1)

Unless this Capability is sent on a given BGP session by both of that session's BGP speakers, a SAFI-4 or SAFI-128 UPDATE message sent on that session from either speaker MUST bind a prefix to only a single label (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2-1, so no unit is bound to it.

### [`RFC8277-2-2`](#rfc8277-2-2)

Further, when advertising the binding of a single label to a prefix, the BGP speaker MUST use the encoding specified in Section 2.2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a single-label binding transmitted in any layout other than the Section 2.2 one. All three transmitters now covered with whole-NLRI literals: LabeledUnicast.WriteTo by TestLabeledSingleLabelSection22Encoding (HEAD, require.Equal on the whole encoding), and UpdateBuilder.BuildLabeledUnicastNLRIBytes (plain and ADD-PATH) and buildVPNNLRIBytes by TestRFC8277BuilderLabelLayout (20 000641 0a, 00000007 20 000641 0a, 60 000641 <RD> 0a; recorded revert of WriteLabelValues; judge overlay dropping the S line of WriteLabelValues reds every case). {single-polarity: positive} is valid: a one-label stack has one layout and Length is computed from the input, so no input reaches a non-Section-2.2 encoding; the reason now names all three transmitters (correction paragraph in rfc/corrections/rfc8277.md). Residual: the HEAD WriteTo unit has no discrimination record of its own.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33) | unit/verify | unproven |

### [`RFC8277-2.1-1`](#rfc8277-2.1-1)

If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-1, so no unit is bound to it.

### [`RFC8277-2.1-2`](#rfc8277-2.1-2)

If (a) a BGP speaker has sent the Multiple Labels Capability in its BGP OPEN message for a particular BGP session, (b) it has received the Multiple Labels Capability in its peer's BGP OPEN message for that session, and (c) both Capabilities specify AFI/SAFI x/y, then when using an UPDATE of AFI x and SAFI y to advertise the binding of a label or sequence of labels to a given prefix, the BGP speaker MUST use the encoding of Section 2.3. This encoding MUST be used even if only one label is being bound to a given prefix. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-2, so no unit is bound to it.

### [`RFC8277-2-3`](#rfc8277-2-3)

If a BGP speaker has not sent the Multiple Labels Capability in its BGP OPEN message on a particular BGP session, or if it has not received the Multiple Labels Capability in the BGP OPEN message from its peer on that BGP session, that BGP speaker MUST NOT send on that session any UPDATE message that binds more than one MPLS label to any given prefix. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2-3, so no unit is bound to it.

### [`RFC8277-2.1-3`](#rfc8277-2.1-3)

Any implementation that sends a Multiple Labels Capability MUST be able to support at least two labels in the NLRI. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-3, so no unit is bound to it.

### [`RFC8277-2.1-4`](#rfc8277-2.1-4)

A Multiple Labels Capability whose length is not a multiple of four MUST be considered to be malformed. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-4, so no unit is bound to it.

### [`RFC8277-2.1-5`](#rfc8277-2.1-5)

A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-5, so no unit is bound to it.

### [`RFC8277-2.1-6`](#rfc8277-2.1-6)

A triple of the form <AFI=x, SAFI=y, Count=0> or <AFI=x, SAFI=y, Count=1> MUST NOT be sent. If such a triple is received, it MUST be ignored. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-6, so no unit is bound to it.

### [`RFC8277-2.1-7`](#rfc8277-2.1-7)

If the Capability contains more than one triple with a given AFI/ SAFI, all but the first MUST be ignored. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-7, so no unit is bound to it.

### [`RFC8277-2.1-8`](#rfc8277-2.1-8)

If a BGP OPEN message contains multiple copies of the Multiple Labels Capability, only the first copy is significant; subsequent copies MUST be ignored. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-8, so no unit is bound to it.

### [`RFC8277-2.1-9`](#rfc8277-2.1-9)

If a BGP speaker receives an UPDATE that binds more labels to a given prefix than the number of labels the BGP speaker is prepared to receive (as announced in its Multiple Labels Capability), the BGP speaker MUST apply the "treat-as-withdraw" strategy of [RFC7606] to that UPDATE. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-9, so no unit is bound to it.

### [`RFC8277-2.1-10`](#rfc8277-2.1-10)

Notwithstanding the number of labels that a BGP speaker has claimed to be able to receive, its peer MUST NOT attempt to send more labels than can be properly encoded in the NLRI field of the MP_REACH_NLRI attribute. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-10, so no unit is bound to it.

### [`RFC8277-2.1-11`](#rfc8277-2.1-11)

If the Multiple Labels Capability for a given AFI/SAFI was exchanged on the failed session but has not been exchanged on the restarted session, then any prefixes advertised in that AFI/SAFI with multiple labels MUST be explicitly withdrawn. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-11, so no unit is bound to it.

### [`RFC8277-2.1-12`](#rfc8277-2.1-12)

Similarly, if the maximum label count (specified in the Capability for a given AFI/SAFI) is reduced, any prefixes advertised with more labels than are valid for the current session MUST be explicitly withdrawn. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-12, so no unit is bound to it.

### [`RFC8277-2.1-13`](#rfc8277-2.1-13)

If either of these conditions hold, the complete set of routes for the given AFI/SAFI MUST be exchanged. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-13, so no unit is bound to it.

### [`RFC8277-2.1-14`](#rfc8277-2.1-14)

"Accelerated Routing Convergence for BGP Graceful Restart" [Enhanced-GR] describes another procedure that allows the routes learned over a given BGP session to be maintained when the session fails and then restarts. These procedures MUST NOT be applied if either of the following conditions hold: o The Multiple Labels Capability for a given AFI/SAFI had been exchanged prior to the restart but has not been exchanged on the restarted session. o The Multiple Labels Capability for a given AFI/SAFI had been exchanged with a given Count prior to the restart but have been exchanged with a smaller count on the restarted session. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-14, so no unit is bound to it.

### [`RFC8277-2.2-1`](#rfc8277-2.2-1)

S: This 1-bit field MUST be set to one on transmission (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a single label entry transmitted with S=0. Positive: TestRFC8277BuilderLabelLayout pins the one-label entry 000641 through both WriteLabelValues builders (labeled plain/ADD-PATH, VPN) as whole-NLRI literals, plus HEAD TestLabeledSingleLabelSection22Encoding for WriteTo. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten hands WriteTo a lone entry 0x000640 with S=0, the relayed-entry input, and asserts 20 000641 0a. Judge overlays: dropping 'entry |= 1' in WriteLabelStack reds the negative (200006400a); dropping the S line of WriteLabelValues reds all five builder cases. The {single-polarity} marker is dropped (correction paragraph), records on both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34) | unit/verify | unproven |

### [`RFC8277-2.2-2`](#rfc8277-2.2-2)

S: This 1-bit field MUST be set to one on transmission and MUST be ignored on reception. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.2-2, so no unit is bound to it.

### [`RFC8277-2.2-3`](#rfc8277-2.2-3)

Rsrv: This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c29 judge), owner ruling 8(d). Reception (MUST ignore) unchanged from c28: TestLabeledRsrvIgnoredOnReceive and TestRFC8277RsrvIgnoredOnReceiveLabelStack. Transmission (SHOULD zero) now enforced for both originated and relayed stacks. Originated: TestRFC8277BuilderLabelLayout (WriteLabelValues, Rsrv 000). Relayed: clearLabelRsrv runs in publishBase, the ingest step every announcement passes before the RIB and both forward rails see it; SAFI 4 and 128 only, read-only scan first, copy only on a hit. Negatives TestRFC8277RsrvClearedOnRelay (Rsrv 111/101/001/110 to 000 over one label, a three-label stack then a second NLRI, ADD-PATH path id, SAFI 128 RD; framing and the received buffer unchanged) and TestRFC8277RsrvClearedOnTheReceivePath (through enforceRFC7606, no RFC 7606 action). Positive TestRFC8277ZeroRsrvRelayedZeroCopy asserts the same WireUpdate for a zero-Rsrv stack and a unicast NLRI. No bypass: the only non-publishBase exits of enforceRFC7606 are treat-as-withdraw (withdrawals carry the Section 2.4 Compatibility field, not a label stack) and session reset; originated stacks come from LabelEntriesFor (Rsrv 0). EVPN (SAFI 70) untouched. Three observed-red records (revert clearLabelRsrv).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277RsrvClearedOnRelay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L67) | unit/verify | revert, verified |
| negative | [`TestRFC8277RsrvClearedOnTheReceivePath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L112) | unit/verify | revert, verified |
| negative | [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L24) | unit/verify | unproven |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC8277ZeroRsrvRelayedZeroCopy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_label_rsrv_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestRFC8277RsrvIgnoredOnReceiveLabelStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_rsrv_stack_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L23) | unit/verify | unproven |

### [`RFC8277-2.3-1`](#rfc8277-2.3-1)

In all labels except the last (i.e., in all labels except the one immediately preceding the prefix), the S bit MUST be 0. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: a non-last label entry transmitted with S=1. Positive: TestRFC8277BuilderLabelLayout pins 50 000640 000c80 0012c1 0a and 78 000640 000c81 <RD> 0a through WriteLabelValues; HEAD TestLabeledLabelStackSBitPlacement covers WriteTo built from values. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten hands WriteTo entries 0x000641, 0x000c81, 0x0012c0 (S=1 mid-stack) and asserts the S bits are cleared. Judge overlay dropping 'entry &^= 1' in WriteLabelStack reds the negative only (50000641000c810012c10a) while the HEAD placement test stays green, so the new unit is what discriminates the relay path. Marker dropped, records on both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77) | unit/verify | unproven |

### [`RFC8277-2.3-2`](#rfc8277-2.3-2)

In the last label, the S bit MUST be 1. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c28 judge). Forbidden: the last label entry transmitted with S=0. Positive: the builder stacks in TestRFC8277BuilderLabelLayout end 0012c1 and 000c81 (WriteLabelValues), HEAD TestLabeledLabelStackSBitPlacement for WriteTo. Genuine negative (R1(b)): TestRFC8277RelayedStackSBitRewritten gives the last entry 0x0012c0 (S=0) and the lone entry 0x000640, and asserts S=1 is transmitted. Judge overlay dropping 'entry |= 1' reds both subcases; dropping the WriteLabelValues S line reds both builder stacks. The negative shares its stack buffer with 2.3-1, but each break is isolated to its own octet and the whole NLRI is compared. Marker dropped, records on both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277RelayedStackSBitRewritten`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_sbit_relay_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC8277BuilderLabelLayout`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8277_label_layout_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78) | unit/verify | unproven |

### [`RFC8277-2.4-1`](#rfc8277-2.4-1)

Upon reception, the value of the Compatibility field MUST be ignored. (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent source rejudgment of all 17 current tagged functions and 23 polarity covers. RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field MUST be ignored." Read the full withdrawal/session/prefix/AFI-SAFI/ADD-PATH context. Exact codec and JSON assertions now cover labeled and VPN IPv4/IPv6, recommended, zero, S-set and other Compatibility values, adjacent identities and negotiated identifiers including zero, with no label output. The real SDK transport checks both decoder directions and preserves labels for announcements. RIB tests assert IPv4 labeled route/label deletion, ADD-PATH survivor labels, VPN removal, original announced withdrawal bytes and remaining-PE promotion. New VPN RR/RS tests assert exact survivor inventories across RD/prefix/path-id controls, plus native peer-down commands and parsed survivor bytes. Producers inspected: keyLabeled/keyVPN; ParseWithdrawnNLRIs/wrapNLRI; appendNLRIJSONValue; registered codec handlers; labeled DecodeNLRIHex/decodeLabeledNLRI; VPN parseVPN; SendDecodeNLRI/SDK DecodeNLRI/OnDecodeNLRI; removeLabeled and opaque RouteKey; checkRouteBestChange; RR walkVPNNLRIs and RS appendOpaqueRecords/recordKey, with both peer-down senders. The prior VPN decoder and opaque-inventory findings are addressed in these current branches and must not be repeated as extant defects. Keep weak, not a whole-stack upgrade: the tagged RIB and labeled RR/RS withdrawal populations do not cover all family/ADD-PATH reception paths, peer-down tests stop at command/RPC boundaries, and real session-to-RIB-to-independent-peer/VPP acceptance remains pending. The promotion carrier does not assert an exact replacement action. Nonrecommended Compatibility is conforming receiver input, not malformed data. Stored native producer-halt records establish reachability only; no execution, freshness validation or semantic mutation was performed by this audit. No partial annotation is present, and no untested-path defect is asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalEventIgnoresOtherCompatibilityValues`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L95) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rpc_decode_nlri_context_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalJSONConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_vpn_consumer_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalEventNamesThePrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalEventNeverReachesTheLabelDecoder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/format/rfc8277_withdraw_event_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC8277LabeledWithdrawIgnoresCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_labeled_withdraw_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawIgnoresCompatibilityValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawPromotesTheOtherPE`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_withdraw_best_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestVPNReflectorPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalLeavesExactReflectorInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_vpn_withdrawal_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalLeavesTheWithdrawalMap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rr/rfc8277_withdrawal_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestVPNRouteServerPeerDownEmitsOnlyRetainedNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalLeavesExactRouteServerInventory`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_vpn_withdrawal_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalLeavesTheRouteServerSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rs/rfc8277_withdrawal_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestLabeledWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_labeled_consumer_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestVPNWithdrawalCodecConsumersIgnoreCompatibility`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc8277_vpn_consumer_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRPCDecodeNLRIRealSDKConsumers`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/ipc/rpc_decode_nlri_context_test.go#L26) | unit/verify | revert, verified |

### [`RFC8277-2.1-15`](#rfc8277-2.1-15)

If both BGP speakers of a given BGP session have sent the Multiple Labels Capability, but AFI/SAFI x/y has not been specified in both Capabilities, then UPDATEs of AFI/SAFI x/y on that session MUST use the encoding of Section 2.2, and such UPDATEs can only bind one label to a prefix. (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-15, so no unit is bound to it.

### [`RFC8277-2.5-1`](#rfc8277-2.5-1)

If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8277 Section 2.5: "If [RFC7911] is not being used, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. That is, the UPDATE U1 is implicitly withdrawn and is replaced by UPDATE U2." Read the full same-session, same-prefix, same-next-hop premise and both tagged carriers. TestLabeledImplicitWithdrawalNoAddPath positively requires exactly one received route and labels [200] after replacing [100]; its negative sends another prefix, requires two routes and the original prefix still bound to [200]. TestRFC8277VPNRelabelReplacesTheRoute sends SAFI-128 with fixed peer, RD, prefix, next hop and attributes, changing only label 100 to 200; it requires one stored route, lookup by the old labeled NLRI, an iteration returning only the new NLRI, and a second best-change naming that new NLRI. The post-lint vpnv4NLRI has removed only the constant /8 argument: length remains 96 and label/RD/prefix bytes remain unchanged. RouteKey strips label identity, insertOpaqueNoOp refuses the old-wire shortcut on relabel, and insertOpaque/setRouteNLRI stores the latest wire NLRI. Distinct same-prefix/different-prefix controls isolate implicit replacement and current assertions would fail on old-label retention or duplicate bindings. Existing enforced judgment is preserved after current source rejudgment; renew the existing VPN RouteKey discrimination record natively. This is no claim of generic VPN RS/RR withdrawal correctness; RFC8277-2.4-1 remains weak.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L94) | unit/verify | unproven |
| positive | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L93) | unit/verify | unproven |
| positive | [`TestRFC8277VPNRelabelReplacesTheRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_test.go#L70) | unit/verify | revert, verified |

### [`RFC8277-2.5-2`](#rfc8277-2.5-2)

If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1. UPDATE U1 is implicitly withdrawn. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent post-edit judgment by AddPathProofReview, not the test author. RFC 8277 Section 2.5: "If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1." TestRFC8277AddPathSameNextHopKeepsIndependentLabels holds peer, prefix, next hop and MED fixed; a reused path 9 replaces [200] by exactly [300], leaves exactly two routes and path 7 at [100]. Its negative boundary uses a different identifier, retaining both routes and both labels. Existing TestLabeledImplicitWithdrawalAddPath and TestRFC8277AddPathLabelsBoundPerPath retain route-entry and binding isolation coverage. pathSet.setLabels binds only the named entry; the parent observed the new test fail with [200] instead of [300] when that producer retained the old handle, and pass in the complete RIB race run. Native records separately observe producer disabling; no generic best-path compliance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L140) | unit/verify | unproven |
| positive | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L139) | unit/verify | unproven |
| positive | [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L196) | unit/verify | revert, verified |

### [`RFC8277-2.5-3`](#rfc8277-2.5-3)

If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent post-edit judgment by AddPathProofReview, not the test author. RFC 8277 Section 2.5: "If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1." The new positive TestRFC8277AddPathSameNextHopKeepsIndependentLabels holds the same-next-hop premise that the earlier different-next-hop test did not isolate: path 9 binds [200] while path 7 keeps [100], including across relabel and targeted withdrawal. TestRFC8277AddPathSameIdentifierRebinds supplies the negative boundary: the SAME path 7 leaves one route with [200] alone. Both reach received UPDATE ingestion and per-path label storage. Parent observed the new positive fail with path 7 reading [200] when setLabels bound every update to the first entry; normal complete RIB race run passed. This closes both the formerly absent negative tag and the same-next-hop proof gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8277AddPathSameIdentifierRebinds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L242) | unit/verify | revert, verified |
| positive | [`TestRFC8277AddPathSameNextHopKeepsIndependentLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestRFC8277AddPathLabelsBoundPerPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L195) | unit/verify | revert, verified |

### [`RFC8277-3.1-1`](#rfc8277-3.1-1)

These two routes MUST be considered to be comparable, even if they specify different labels. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment of all three current tagged functions and four polarity covers. RFC 8277 Section 3.1: "These two routes MUST be considered to be comparable, even if they specify different labels." Read the full different-session or same-session/different-ADD-PATH-identifier premise and "The same applies to SAFI-128 routes." TestLabeledRoutesWithDifferentLabelsAreComparable ingests two iBGP peers, requires exactly two candidates and one stored best, elects peer A/MED10 against the smaller-address fallback, then swaps labels and requires the same winner, MED10, one stored best and the winner's new label200. This is a distinct negative label-invariance control. TestRFC8277AddPathRoutesOnOneSessionAreComparable ingests paths7/9 and gathers both from either key, requiring path9/MED10 against path-id fallback. TestRFC8277VPNAddPathLabelsAreOneElection gathers both VPN paths from either label-bearing key, requires path7/MED10 and exact winning framed NLRI, then checks relabel count and path-specific Compatibility withdrawal. newOpaqueAddPathPeer establishes iBGP metadata; that VPN winner aligns with path-id fallback and is not independently credited as a MED-bypass oracle. Producers inspected: label-free CIDR/native RouteKey identities, gatherCandidatesLocked, gatherPrefixCandidatesLocked, gatherKeyCandidatesLocked, checkRouteBestChange and SelectBest/selectBestCandidates. These establish shared election and label-independent comparability, not every RFC4271 criterion or whole-set MED, send-path propagation, live peer interoperability or VPP acceptance. Stored producer-panic records establish reachability only and were not replayed or freshness-verified. No test or gate was executed by this audit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L281) | unit/verify | revert, verified |
| positive | [`TestRFC8277AddPathRoutesOnOneSessionAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L280) | unit/verify | revert, verified |
| positive | [`TestRFC8277VPNAddPathLabelsAreOneElection`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_vpn_route_key_addpath_test.go#L20) | unit/verify | revert, verified |

### [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1)

When a SAFI-4 or SAFI-128 route is propagated, if the Network Address of Next Hop field is left unchanged, the Label field(s) MUST also be left unchanged. (§3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Quote covers SAFI-4 and SAFI-128. TestLabeledPropagationUnchangedNextHopKeepsLabels pins SAFI 4 and TestLabeledVPNPropagationUnchangedNextHopKeepsLabels pins SAFI 128: next-hop-unchanged emits no op and mpReachNextHopHandler plans the MP_REACH verbatim, with the Next Hop, label entry, RD and prefix asserted byte for byte. The {single-polarity: positive} annotation holds: the RFC imposes nothing on a changed next hop. Observed-red record on mpReachNextHopHandler.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L33) | unit/verify | unproven |
| positive | [`TestLabeledVPNPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_vpn_propagation_test.go#L29) | unit/verify | revert, verified |

### [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2)

Note that a given route MUST NOT be propagated to a given peer if the route's NLRI has multiple labels, but the Multiple Labels Capability was not negotiated with the peer. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-2, so no unit is bound to it.

### [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3)

Similarly, a given route MUST NOT be propagated to a given peer if the route's NLRI has more labels than the peer has announced (through its Multiple Labels Capability) that it can handle. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-3, so no unit is bound to it.

### [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4)

In either case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to the peer, that route MUST be withdrawn from that peer using the procedure specified in Section 2.4. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-4, so no unit is bound to it.

### [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1)

If the Network Address of Next Hop field is changed before a SAFI-4 or SAFI-128 route is propagated, the Label field(s) of the propagated route MUST contain the label(s) that is (are) bound to the prefix at the new next hop. (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.2-1, so no unit is bound to it.

### [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2)

A BGP speaker MUST NOT send multiple labels to a peer with which it has not exchanged the Multiple Labels Capability (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.2-2, so no unit is bound to it.

### [`RFC8277-3.2.2-3`](#rfc8277-3.2.2-3)

It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4. (§3.2.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. RFC 8277 §3.2.2 requires a Section 2.4 withdrawal of the earlier same-AFI/SAFI/prefix fewer-label route when the speaker decides not to propagate the replacement stack to a peer that cannot handle it. This is a distinct trigger from §3.2.1. mpReachNextHopHandler changes the next hop and copies the NLRI without a label-count decision, and BuildLabeledUnicastNLRIBytes encodes every supplied label. Multiple Labels Capability negotiation/count handling remains absent. The existing TestLabeledPropagationChangedNextHopReusesReceivedLabelGap observes unchanged labels with a changed next hop, not the required withdrawal. Record the missing behavior explicitly; absence does not establish vacuous conformance or authorize development.

No test carries RFC8277-3.2.2-3, so no unit is bound to it.

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `2.1` | not stated | 17 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `2.3` | not stated | 3 | walked | not stated |
| `2.4` | not stated | 2 | walked | not stated |
| `2.5` | not stated | 3 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 4 | walked | not stated |
| `3.2.2` | not stated | 3 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Section 2 overview bullet announcing the Section 2.3 encoding rule that Section 2.1 states in full; site 2.1:13 carries it. The bullet's other half, "MAY bind a prefix to a sequence of more than one label", is the permission the same row's condition already grants. | o If this Capability is sent by both BGP speakers on a given session, an UPDATE message on that session, from either speaker, MUST use the encoding of Section 2.3 and MAY bind a prefix to a sequence of more than one label. |
| `2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.3 repeats the Section 2.2 Rsrv sentence word for word for each stack entry. RFC8277-2.2-3 retains the full receive/transmit sentence; the dated Correction 2026-10-06 identifies its existing multi-label receive, origination and relay tests. RFC8277-2.2-4 repeats only its transmit SHOULD. The rows cite Section 2.2; this site records the Section 2.3 duplicate without claiming otherwise. | This 3-bit field SHOULD be set to zero on transmission and MUST be ignored on reception. |
| `2.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 2.4 summary sentence restates the reception obligation site 2.4:1 states on its own ("Upon reception, the value of the Compatibility field MUST be ignored."), which maps RFC8277-2.4-1. Its RECOMMENDED half is row RFC8277-2.4-2 and is advisory. | In order to ensure backwards compatibility, it is RECOMMENDED by this document that the Compatibility field be set to 0x800000, but it is REQUIRED that it be ignored upon reception. |

## Superseded

No document obsoletes RFC 8277, so its obligations are stated where they were written.
