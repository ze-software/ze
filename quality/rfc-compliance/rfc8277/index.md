# RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes

Partial. Every requirement this repository extracted from RFC 8277, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 21.1% | 4 of 19 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 26.3% | 5 of 19 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 19 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 34 | of 41 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 15 | of 34 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 52.6% | 10 of 19 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 19 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 41 |
| Gated MUST-level | 34 |
| Obligations that bind Ze | 19 |
| Not applicable, so out of scope | 15 |
| Declared gaps | 10 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8277.md` |
| Requirement shard | `rfc/requirements/rfc8277.md` |
| RFC text | `rfc/full/rfc8277.txt` |

## Enrolment

Enrolled: Using BGP to Bind MPLS Labels to Address Prefixes

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv4 and IPv6 labeled unicast NLRI (SAFI 4) encode and decode, label stack handling, ADD-PATH framing, route config, Adj-RIB-In label side-data, best-path comparability across differing labels, and dataplane handoff
- requirements bound per line in [`rfc/short/rfc8277.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8277.md).


**What the ledger says remains**

Ten MUST-level gaps, each annotated in [`rfc/short/rfc8277.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8277.md). Multiple Labels Capability (code 8) is absent from ze's capability set ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)), so every session is in the RFC 8277 Section 2 single-label mode while the encoders still accept and emit a full stack: [`RFC8277-2-1`](#rfc8277-2-1), [`RFC8277-2-3`](#rfc8277-2-3), [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2). Propagation of an already-multi-label route is unguarded for the same reason: [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2) -- the prohibition binds on every peer precisely because the capability is negotiated with none, and [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4) -- no propagation is ever blocked on label count, so the paired withdrawal has no producer either.

- **Encoding:** [`RFC8277-2.1-10`](#rfc8277-2.1-10) -- the one-octet NLRI Length is written as `byte(totalBits)` with no bound check ([`internal/component/bgp/plugins/nlri/labeled/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/types.go), [`internal/component/bgp/message/update_build_labeled.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_labeled.go)).
- **Reception:** [`RFC8277-2.2-2`](#rfc8277-2.2-2) and [`RFC8277-2.4-1`](#rfc8277-2.4-1) -- the label-stack readers are S-bit-driven ([`internal/core/bgp/nlri/nlrisplit/labeled.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/labeled.go),:112), so a single-label NLRI with S clear, and a Section 2.4 withdrawal carrying the RECOMMENDED Compatibility value 0x800000, are both read past their prefix and rejected.
- **State:** [`RFC8277-2.5-3`](#rfc8277-2.5-3) -- label side-data is keyed on the prefix alone ([`internal/component/bgp/plugins/rib/storage/peerrib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/peerrib.go), familyrib.go), so a second ADD-PATH path overwrites the first path's label binding.
- **Propagation:** [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1) -- ze binds no local label, so a next-hop rewrite re-advertises the upstream label unchanged ([`internal/component/bgp/reactor/filter_delta_handlers.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/filter_delta_handlers.go)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 30 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **34** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC8277-2.2-3`](#rfc8277-2.2-3), [`RFC8277-2.5-1`](#rfc8277-2.5-1), [`RFC8277-2.5-2`](#rfc8277-2.5-2), [`RFC8277-3.1-1`](#rfc8277-3.1-1)

**Annotated instead of tested (30):** [`RFC8277-2-1`](#rfc8277-2-1), [`RFC8277-2-2`](#rfc8277-2-2), [`RFC8277-2.1-1`](#rfc8277-2.1-1), [`RFC8277-2.1-2`](#rfc8277-2.1-2), [`RFC8277-2-3`](#rfc8277-2-3), [`RFC8277-2.1-3`](#rfc8277-2.1-3), [`RFC8277-2.1-4`](#rfc8277-2.1-4), [`RFC8277-2.1-5`](#rfc8277-2.1-5), [`RFC8277-2.1-6`](#rfc8277-2.1-6), [`RFC8277-2.1-7`](#rfc8277-2.1-7), [`RFC8277-2.1-8`](#rfc8277-2.1-8), [`RFC8277-2.1-9`](#rfc8277-2.1-9), [`RFC8277-2.1-10`](#rfc8277-2.1-10), [`RFC8277-2.1-11`](#rfc8277-2.1-11), [`RFC8277-2.1-12`](#rfc8277-2.1-12), [`RFC8277-2.1-13`](#rfc8277-2.1-13), [`RFC8277-2.1-14`](#rfc8277-2.1-14), [`RFC8277-2.2-1`](#rfc8277-2.2-1), [`RFC8277-2.2-2`](#rfc8277-2.2-2), [`RFC8277-2.3-1`](#rfc8277-2.3-1), [`RFC8277-2.3-2`](#rfc8277-2.3-2), [`RFC8277-2.4-1`](#rfc8277-2.4-1), [`RFC8277-2.1-15`](#rfc8277-2.1-15), [`RFC8277-2.5-3`](#rfc8277-2.5-3), [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1), [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2), [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3), [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4), [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1), [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8277-2-1` | Without Multiple Labels Capability exchanged, bind a prefix to only a single label (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2-2` | Without Multiple Labels Capability exchanged, use the encoding of Section 2.2 (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33). **negative:** no negative test. **{single-polarity}:** LabeledUnicast.WriteTo emits the §2.2 layout unconditionally -- Length octet = 24 + prefix bits, one 3-octet label entry, prefix (internal/component/bgp/plugins/nlri/labeled/types.go:125) -- so there is no non-conformant transmission input to reject |
| `RFC8277-2.1-1` | When Multiple Labels Capability exchanged for an AFI/SAFI, use the encoding of Section 2.3 (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Multiple Labels Capability (code 8) producer -- the registered capability codes are 1, 2, 5, 6, 9, 64, 65, 69, 70, 73, 76 (internal/core/bgp/capability/capability.go:68-78) and `grep -rni "multiple.label\\\|CodeMultipleLabels" internal/ pkg/` matches no capability code, so no session ever reaches the Section 2.3 branch |
| `RFC8277-2.1-2` | When Multiple Labels Capability exchanged, use Section 2.3 encoding even for a single label (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** same missing producer -- capability code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78), so the "capability exchanged" state never exists |
| `RFC8277-2-3` | Send UPDATE binding more than one label without having both sent and received Multiple Labels Capability (§2, §2.1) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| `RFC8277-2.1-3` | Support at least two labels in NLRI when sending Multiple Labels Capability (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no Multiple Labels Capability -- the OPEN capability encoder writes only the codes listed at internal/core/bgp/capability/capability.go:68-78, which exclude code 8, so the obligation attached to sending it has no producer |
| `RFC8277-2.1-4` | Capability length must be a multiple of 4; otherwise consider malformed (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no Multiple Labels Capability parser exists to length-check -- `grep -rni "multiple.label" internal/core/bgp/capability/` returns nothing and code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-5` | Send a triple with Count=0 or Count=1 (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no AFI/SAFI/Count triples because it emits no capability code 8 (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-6` | Ignore received triples with Count=0 or Count=1 (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no triple parser exists -- capability code 8 has no decoder (internal/core/bgp/capability/capability.go:68-78), so a received capability is handled by the RFC 5492 unknown-capability path rather than by a Count reader |
| `RFC8277-2.1-7` | Ignore all but the first triple for a given AFI/SAFI in the capability (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no Multiple Labels Capability decoder exists to hold per-AFI/SAFI triples (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-8` | Ignore subsequent copies if multiple copies of Multiple Labels Capability in OPEN (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze stores no Multiple Labels Capability state, so no first-copy-wins rule has a producer (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-9` | Apply treat-as-withdraw per RFC 7606 when UPDATE binds more labels than receiver's announced Count (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze announces no Count because it announces no capability code 8 (internal/core/bgp/capability/capability.go:68-78), so the "more labels than announced Count" condition has no producer to evaluate it |
| `RFC8277-2.1-10` | Attempt to send more labels than can be properly encoded in NLRI field of MP_REACH_NLRI (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| `RFC8277-2.1-11` | Explicitly withdraw prefixes advertised with multiple labels when capability lost on session restart (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** capability code 8 is never negotiated (internal/core/bgp/capability/capability.go:68-78), so "capability lost on restart" is not a state ze can enter |
| `RFC8277-2.1-12` | Explicitly withdraw prefixes advertised with more labels than valid when maximum label count reduced (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze tracks no per-peer maximum label Count, since no capability code 8 decoder exists to supply one (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.1-13` | Exchange complete set of routes for given AFI/SAFI when capability or count changes (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** neither the capability nor the Count exists in ze's session state (internal/core/bgp/capability/capability.go:68-78), so no change event can trigger the full re-exchange |
| `RFC8277-2.1-14` | Apply Enhanced Graceful Restart procedures when capability lost or count reduced on restart (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the guard condition is a Multiple Labels Capability or Count change, and neither is represented anywhere in ze -- `grep -rni "multiple.label" internal/component/bgp/plugins/gr/` returns nothing and code 8 is absent from internal/core/bgp/capability/capability.go:68-78 |
| `RFC8277-2.2-1` | S bit must be set to 1 on transmission in single-label encoding (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34). **negative:** no negative test. **{single-polarity}:** WriteLabelStack sets the bottom-of-stack bit on the final entry unconditionally, so a one-label stack always transmits S=1 and no code path can emit S=0 to reject (internal/core/bgp/nlri/helpers.go:67) |
| `RFC8277-2.2-2` | S bit must be ignored on reception in single-label encoding (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| `RFC8277-2.2-3` | Rsrv field must be ignored on reception (§2.2, §2.3) | MUST | 2.2 | **positive:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L21). **negative:** `unit/verify` [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L22) |
| `RFC8277-2.3-1` | In multiple-label encoding, S bit must be 0 in all labels except the last (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77). **negative:** no negative test. **{single-polarity}:** WriteLabelStack sets the S bit only when i == len(labels)-1, leaving every earlier entry at S=0 by construction, so there is no transmission input that could carry a mid-stack S bit to reject (internal/core/bgp/nlri/helpers.go:67) |
| `RFC8277-2.3-2` | In multiple-label encoding, S bit must be 1 in the last label (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78). **negative:** no negative test. **{single-polarity}:** the same encoder always terminates the stack with S=1 and offers no way to emit a stack without a bottom-of-stack entry (internal/core/bgp/nlri/helpers.go:67) |
| `RFC8277-2.4-1` | Compatibility field in withdrawal must be ignored on reception (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a §2.4 withdrawal is [Length][Compatibility(3)][Prefix], but ze reads those three octets as a label stack entry and stops on the S bit -- ExtractLabels (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so the RECOMMENDED value 0x800000, whose S bit is clear, makes the reader run past the NLRI and the withdrawal is dropped by removeLabeled's error return (internal/component/bgp/plugins/rib/rib_structured.go:565) |
| `RFC8277-2.1-15` | If both speakers sent Multiple Labels Capability but a given AFI/SAFI was not specified in both, UPDATEs for that AFI/SAFI MUST use the encoding of Section 2.2 (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the premise is that both speakers sent capability code 8, which ze never sends or parses (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-2.5-1` | Without ADD-PATH, UPDATE U2 for same prefix/nexthop MUST be interpreted as implicit withdrawal of U1 (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L93). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L94) |
| `RFC8277-2.5-2` | With ADD-PATH and same Path Identifier, UPDATE U2 MUST be interpreted as implicit withdrawal of U1 (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L139). **negative:** `unit/verify` [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L140) |
| `RFC8277-2.5-3` | With ADD-PATH and different Path Identifiers, U2 MUST be interpreted as a new binding; U2 MUST NOT be interpreted as withdrawing U1 (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** route entries are keyed on (path-id, prefix) but the MPLS label side-data is keyed on the prefix alone -- SetLabelsIfRouteExists discards the path-id returned by parseNLRIKey (internal/component/bgp/plugins/rib/storage/peerrib.go:320) and FamilyRIB.SetLabels stores one handle per prefix (internal/component/bgp/plugins/rib/storage/familyrib.go:569) -- so U2 for a second Path Identifier overwrites U1's label binding, and RemoveLabels for either path deletes the shared entry (internal/component/bgp/plugins/rib/storage/peerrib.go:341) |
| `RFC8277-3.1-1` | Routes with different labels for the same prefix must be considered comparable for best-path selection (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L226). **negative:** `unit/verify` [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L227) |
| `RFC8277-3.2.1-1` | When propagating with unchanged Next Hop, Label field(s) must also be left unchanged (§3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L33). **negative:** no negative test. **{single-polarity}:** an unchanged next hop produces no next-hop modification op (applyNextHopMod, internal/component/bgp/reactor/reactor_api_forward.go:815) and mpReachNextHopHandler copies the source attribute verbatim when no Set op reaches it (internal/component/bgp/reactor/filter_delta_handlers.go:252), so the labels are preserved by construction and there is no non-conformant propagation input to reject |
| `RFC8277-3.2.1-2` | Propagate route to peer if NLRI has multiple labels but Multiple Labels Capability not negotiated with that peer (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-3.2.1-3` | Propagate route to peer if NLRI has more labels than peer's announced Count (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no peer Count is stored because no capability code 8 decoder exists (internal/core/bgp/capability/capability.go:68-78) |
| `RFC8277-3.2.1-4` | Withdraw previous route from peer if propagation blocked due to label count (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| `RFC8277-3.2.2-1` | When propagating with changed Next Hop, Label field(s) must contain labels bound to prefix at the new next hop (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| `RFC8277-3.2.2-2` | Send multiple labels to peer without having exchanged Multiple Labels Capability (§3.2.2) | MUST NOT | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| `RFC8277-2.2-4` | Rsrv field should be set to zero on transmission (§2.2, §2.3) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.4-2` | Compatibility field in withdrawal should be set to 0x800000 on transmission (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.1-16` | Send more labels to a peer than peer's Count in Multiple Labels Capability (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-4` | Treat same next-hop from different sessions as multiple paths (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-2.5-5` | Load-balance between multiple paths with different labels for same prefix and next-hop (§2.5) | MAY | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-5-1` | Convert SAFI-4 to SAFI-1 for propagation to non-SAFI-4 peers (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8277-4-1` | Use SAFI-4 routes for IP forwarding (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8277-2-1`](#rfc8277-2-1) Without Multiple Labels Capability exchanged, bind a prefix to only a single label (§2) | {gap}, no test | ze never exchanges the Multiple Labels Capability, so every session is in the single-label mode of §2, yet the operator-facing encoder accepts an unbounded stack: parseLabeledNLRI appends one label per `label` token (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:325) and BuildLabeledUnicastNLRIBytes encodes len(p.Labels) entries with no capability check (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-1`](#rfc8277-2.1-1) When Multiple Labels Capability exchanged for an AFI/SAFI, use the encoding of Section 2.3 (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Multiple Labels Capability (code 8) producer -- the registered capability codes are 1, 2, 5, 6, 9, 64, 65, 69, 70, 73, 76 (internal/core/bgp/capability/capability.go:68-78) and `grep -rni "multiple.label\\\|CodeMultipleLabels" internal/ pkg/` matches no capability code, so no session ever reaches the Section 2.3 branch |
| [`RFC8277-2.1-2`](#rfc8277-2.1-2) When Multiple Labels Capability exchanged, use Section 2.3 encoding even for a single label (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: same missing producer -- capability code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78), so the "capability exchanged" state never exists |
| [`RFC8277-2-3`](#rfc8277-2-3) Send UPDATE binding more than one label without having both sent and received Multiple Labels Capability (§2, §2.1) | {gap}, no test | with no capability code 8 producer (internal/core/bgp/capability/capability.go:68-78) the precondition is never met, and yet WriteLabelStack emits every label it is given, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61), reached from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257) |
| [`RFC8277-2.1-3`](#rfc8277-2.1-3) Support at least two labels in NLRI when sending Multiple Labels Capability (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no Multiple Labels Capability -- the OPEN capability encoder writes only the codes listed at internal/core/bgp/capability/capability.go:68-78, which exclude code 8, so the obligation attached to sending it has no producer |
| [`RFC8277-2.1-4`](#rfc8277-2.1-4) Capability length must be a multiple of 4; otherwise consider malformed (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no Multiple Labels Capability parser exists to length-check -- `grep -rni "multiple.label" internal/core/bgp/capability/` returns nothing and code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-5`](#rfc8277-2.1-5) Send a triple with Count=0 or Count=1 (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no AFI/SAFI/Count triples because it emits no capability code 8 (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-6`](#rfc8277-2.1-6) Ignore received triples with Count=0 or Count=1 (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no triple parser exists -- capability code 8 has no decoder (internal/core/bgp/capability/capability.go:68-78), so a received capability is handled by the RFC 5492 unknown-capability path rather than by a Count reader |
| [`RFC8277-2.1-7`](#rfc8277-2.1-7) Ignore all but the first triple for a given AFI/SAFI in the capability (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no Multiple Labels Capability decoder exists to hold per-AFI/SAFI triples (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-8`](#rfc8277-2.1-8) Ignore subsequent copies if multiple copies of Multiple Labels Capability in OPEN (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze stores no Multiple Labels Capability state, so no first-copy-wins rule has a producer (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-9`](#rfc8277-2.1-9) Apply treat-as-withdraw per RFC 7606 when UPDATE binds more labels than receiver's announced Count (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze announces no Count because it announces no capability code 8 (internal/core/bgp/capability/capability.go:68-78), so the "more labels than announced Count" condition has no producer to evaluate it |
| [`RFC8277-2.1-10`](#rfc8277-2.1-10) Attempt to send more labels than can be properly encoded in NLRI field of MP_REACH_NLRI (§2.1) | {gap}, no test | the NLRI Length field is one octet and both encoders narrow the computed bit count without a bound check -- LabeledUnicast.WriteTo writes buf[pos] = byte(totalBits) (internal/component/bgp/plugins/nlri/labeled/types.go:135) and BuildLabeledUnicastNLRIBytes writes buf[0] = byte(totalBits) (internal/component/bgp/message/update_build_labeled.go:277) -- so a stack whose labels plus prefix exceed 255 bits is emitted with a wrapped Length octet instead of being refused |
| [`RFC8277-2.1-11`](#rfc8277-2.1-11) Explicitly withdraw prefixes advertised with multiple labels when capability lost on session restart (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: capability code 8 is never negotiated (internal/core/bgp/capability/capability.go:68-78), so "capability lost on restart" is not a state ze can enter |
| [`RFC8277-2.1-12`](#rfc8277-2.1-12) Explicitly withdraw prefixes advertised with more labels than valid when maximum label count reduced (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze tracks no per-peer maximum label Count, since no capability code 8 decoder exists to supply one (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.1-13`](#rfc8277-2.1-13) Exchange complete set of routes for given AFI/SAFI when capability or count changes (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: neither the capability nor the Count exists in ze's session state (internal/core/bgp/capability/capability.go:68-78), so no change event can trigger the full re-exchange |
| [`RFC8277-2.1-14`](#rfc8277-2.1-14) Apply Enhanced Graceful Restart procedures when capability lost or count reduced on restart (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the guard condition is a Multiple Labels Capability or Count change, and neither is represented anywhere in ze -- `grep -rni "multiple.label" internal/component/bgp/plugins/gr/` returns nothing and code 8 is absent from internal/core/bgp/capability/capability.go:68-78 |
| [`RFC8277-2.2-2`](#rfc8277-2.2-2) S bit must be ignored on reception in single-label encoding (§2.2) | {gap}, no test | the receive path is S-driven rather than Length-driven -- ExtractLabels keeps consuming 3-octet entries until it sees S=1 (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled frames the NLRI the same way (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so a conformant single-label NLRI whose S bit is clear is read past its prefix and rejected as a truncated label stack |
| [`RFC8277-2.4-1`](#rfc8277-2.4-1) Compatibility field in withdrawal must be ignored on reception (§2.4) | {gap}, no test | a §2.4 withdrawal is [Length][Compatibility(3)][Prefix], but ze reads those three octets as a label stack entry and stops on the S bit -- ExtractLabels (internal/core/bgp/nlri/nlrisplit/labeled.go:112) and SplitLabeled (internal/core/bgp/nlri/nlrisplit/labeled.go:63) -- so the RECOMMENDED value 0x800000, whose S bit is clear, makes the reader run past the NLRI and the withdrawal is dropped by removeLabeled's error return (internal/component/bgp/plugins/rib/rib_structured.go:565) |
| [`RFC8277-2.1-15`](#rfc8277-2.1-15) If both speakers sent Multiple Labels Capability but a given AFI/SAFI was not specified in both, UPDATEs for that AFI/SAFI MUST use the encoding of Section 2.2 (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the premise is that both speakers sent capability code 8, which ze never sends or parses (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-2.5-3`](#rfc8277-2.5-3) With ADD-PATH and different Path Identifiers, U2 MUST be interpreted as a new binding; U2 MUST NOT be interpreted as withdrawing U1 (§2.5) | {gap}, no test | route entries are keyed on (path-id, prefix) but the MPLS label side-data is keyed on the prefix alone -- SetLabelsIfRouteExists discards the path-id returned by parseNLRIKey (internal/component/bgp/plugins/rib/storage/peerrib.go:320) and FamilyRIB.SetLabels stores one handle per prefix (internal/component/bgp/plugins/rib/storage/familyrib.go:569) -- so U2 for a second Path Identifier overwrites U1's label binding, and RemoveLabels for either path deletes the shared entry (internal/component/bgp/plugins/rib/storage/peerrib.go:341) |
| [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2) Propagate route to peer if NLRI has multiple labels but Multiple Labels Capability not negotiated with that peer (§3.2.1) | {gap}, no test | the condition is that the capability is NOT negotiated, and ze negotiates it with nobody -- code 8 is absent from the Code constants (internal/core/bgp/capability/capability.go:68-78) -- so the prohibition binds on every peer, and ze propagates anyway: WriteLabelStack emits every label it is handed, S clear on all but the last (internal/core/bgp/nlri/helpers.go:61,:67) from BuildLabeledUnicastNLRIBytes (internal/component/bgp/message/update_build_labeled.go:257), and mpReachNextHopHandler copies a received multi-label NLRI verbatim while patching only the next hop (internal/component/bgp/reactor/filter_delta_handlers.go:241) |
| [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3) Propagate route to peer if NLRI has more labels than peer's announced Count (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: no peer Count is stored because no capability code 8 decoder exists (internal/core/bgp/capability/capability.go:68-78) |
| [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4) Withdraw previous route from peer if propagation blocked due to label count (§3.2.1, §3.2.2) | {gap}, no test | the trigger never fires only because ze fails RFC8277-3.2.1-2 -- no propagation is ever blocked on label count, since the egress path applies no label-count limit at all (internal/component/bgp/message/update_build_labeled.go:257, internal/component/bgp/reactor/filter_delta_handlers.go:241). Vacuity produced by ze's own violation is not a reason the obligation does not apply: implementing the §3.2.1 block requires this withdrawal alongside it, and no withdrawal producer keyed on label count exists |
| [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1) When propagating with changed Next Hop, Label field(s) must contain labels bound to prefix at the new next hop (§3.2.2) | {gap}, no test | ze binds no local MPLS label for labeled unicast, and a next-hop change rewrites only the next-hop field -- mpReachNextHopHandler patches the next hop in place and copies the AFI/SAFI header, reserved octet and NLRI unchanged (internal/component/bgp/reactor/filter_delta_handlers.go:241) -- so the upstream label is re-advertised alongside a next hop that never bound it |
| [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2) Send multiple labels to peer without having exchanged Multiple Labels Capability (§3.2.2) | {gap}, no test | no capability code 8 is ever exchanged (internal/core/bgp/capability/capability.go:68-78), and the egress path applies no label-count limit -- BuildLabeledUnicastNLRIBytes encodes every label in LabeledUnicastParams.Labels (internal/component/bgp/message/update_build_labeled.go:257) and the forwarding path copies a received multi-label NLRI verbatim (internal/component/bgp/reactor/filter_delta_handlers.go:241) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8277-2-1`](#rfc8277-2-1)

Without Multiple Labels Capability exchanged, bind a prefix to only a single label (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2-1, so no unit is bound to it.

### [`RFC8277-2-2`](#rfc8277-2-2)

Without Multiple Labels Capability exchanged, use the encoding of Section 2.2 (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L33) | unit/verify | unproven |

### [`RFC8277-2.1-1`](#rfc8277-2.1-1)

When Multiple Labels Capability exchanged for an AFI/SAFI, use the encoding of Section 2.3 (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-1, so no unit is bound to it.

### [`RFC8277-2.1-2`](#rfc8277-2.1-2)

When Multiple Labels Capability exchanged, use Section 2.3 encoding even for a single label (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-2, so no unit is bound to it.

### [`RFC8277-2-3`](#rfc8277-2-3)

Send UPDATE binding more than one label without having both sent and received Multiple Labels Capability (§2, §2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2-3, so no unit is bound to it.

### [`RFC8277-2.1-3`](#rfc8277-2.1-3)

Support at least two labels in NLRI when sending Multiple Labels Capability (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-3, so no unit is bound to it.

### [`RFC8277-2.1-4`](#rfc8277-2.1-4)

Capability length must be a multiple of 4; otherwise consider malformed (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-4, so no unit is bound to it.

### [`RFC8277-2.1-5`](#rfc8277-2.1-5)

Send a triple with Count=0 or Count=1 (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-5, so no unit is bound to it.

### [`RFC8277-2.1-6`](#rfc8277-2.1-6)

Ignore received triples with Count=0 or Count=1 (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-6, so no unit is bound to it.

### [`RFC8277-2.1-7`](#rfc8277-2.1-7)

Ignore all but the first triple for a given AFI/SAFI in the capability (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-7, so no unit is bound to it.

### [`RFC8277-2.1-8`](#rfc8277-2.1-8)

Ignore subsequent copies if multiple copies of Multiple Labels Capability in OPEN (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-8, so no unit is bound to it.

### [`RFC8277-2.1-9`](#rfc8277-2.1-9)

Apply treat-as-withdraw per RFC 7606 when UPDATE binds more labels than receiver's announced Count (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-9, so no unit is bound to it.

### [`RFC8277-2.1-10`](#rfc8277-2.1-10)

Attempt to send more labels than can be properly encoded in NLRI field of MP_REACH_NLRI (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-10, so no unit is bound to it.

### [`RFC8277-2.1-11`](#rfc8277-2.1-11)

Explicitly withdraw prefixes advertised with multiple labels when capability lost on session restart (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-11, so no unit is bound to it.

### [`RFC8277-2.1-12`](#rfc8277-2.1-12)

Explicitly withdraw prefixes advertised with more labels than valid when maximum label count reduced (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-12, so no unit is bound to it.

### [`RFC8277-2.1-13`](#rfc8277-2.1-13)

Exchange complete set of routes for given AFI/SAFI when capability or count changes (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-13, so no unit is bound to it.

### [`RFC8277-2.1-14`](#rfc8277-2.1-14)

Apply Enhanced Graceful Restart procedures when capability lost or count reduced on restart (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-14, so no unit is bound to it.

### [`RFC8277-2.2-1`](#rfc8277-2.2-1)

S bit must be set to 1 on transmission in single-label encoding (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledSingleLabelSection22Encoding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L34) | unit/verify | unproven |

### [`RFC8277-2.2-2`](#rfc8277-2.2-2)

S bit must be ignored on reception in single-label encoding (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.2-2, so no unit is bound to it.

### [`RFC8277-2.2-3`](#rfc8277-2.2-3)

Rsrv field must be ignored on reception (§2.2, §2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L22) | unit/verify | unproven |
| positive | [`TestLabeledRsrvIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/rfc8277_test.go#L21) | unit/verify | unproven |

### [`RFC8277-2.3-1`](#rfc8277-2.3-1)

In multiple-label encoding, S bit must be 0 in all labels except the last (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L77) | unit/verify | unproven |

### [`RFC8277-2.3-2`](#rfc8277-2.3-2)

In multiple-label encoding, S bit must be 1 in the last label (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledLabelStackSBitPlacement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/labeled/rfc8277_test.go#L78) | unit/verify | unproven |

### [`RFC8277-2.4-1`](#rfc8277-2.4-1)

Compatibility field in withdrawal must be ignored on reception (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.4-1, so no unit is bound to it.

### [`RFC8277-2.1-15`](#rfc8277-2.1-15)

If both speakers sent Multiple Labels Capability but a given AFI/SAFI was not specified in both, UPDATEs for that AFI/SAFI MUST use the encoding of Section 2.2 (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.1-15, so no unit is bound to it.

### [`RFC8277-2.5-1`](#rfc8277-2.5-1)

Without ADD-PATH, UPDATE U2 for same prefix/nexthop MUST be interpreted as implicit withdrawal of U1 (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L94) | unit/verify | unproven |
| positive | [`TestLabeledImplicitWithdrawalNoAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L93) | unit/verify | unproven |

### [`RFC8277-2.5-2`](#rfc8277-2.5-2)

With ADD-PATH and same Path Identifier, UPDATE U2 MUST be interpreted as implicit withdrawal of U1 (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L140) | unit/verify | unproven |
| positive | [`TestLabeledImplicitWithdrawalAddPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L139) | unit/verify | unproven |

### [`RFC8277-2.5-3`](#rfc8277-2.5-3)

With ADD-PATH and different Path Identifiers, U2 MUST be interpreted as a new binding; U2 MUST NOT be interpreted as withdrawing U1 (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-2.5-3, so no unit is bound to it.

### [`RFC8277-3.1-1`](#rfc8277-3.1-1)

Routes with different labels for the same prefix must be considered comparable for best-path selection (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L227) | unit/verify | unproven |
| positive | [`TestLabeledRoutesWithDifferentLabelsAreComparable`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8277_test.go#L226) | unit/verify | unproven |

### [`RFC8277-3.2.1-1`](#rfc8277-3.2.1-1)

When propagating with unchanged Next Hop, Label field(s) must also be left unchanged (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLabeledPropagationUnchangedNextHopKeepsLabels`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8277_test.go#L33) | unit/verify | unproven |

### [`RFC8277-3.2.1-2`](#rfc8277-3.2.1-2)

Propagate route to peer if NLRI has multiple labels but Multiple Labels Capability not negotiated with that peer (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-2, so no unit is bound to it.

### [`RFC8277-3.2.1-3`](#rfc8277-3.2.1-3)

Propagate route to peer if NLRI has more labels than peer's announced Count (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-3, so no unit is bound to it.

### [`RFC8277-3.2.1-4`](#rfc8277-3.2.1-4)

Withdraw previous route from peer if propagation blocked due to label count (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.1-4, so no unit is bound to it.

### [`RFC8277-3.2.2-1`](#rfc8277-3.2.2-1)

When propagating with changed Next Hop, Label field(s) must contain labels bound to prefix at the new next hop (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.2-1, so no unit is bound to it.

### [`RFC8277-3.2.2-2`](#rfc8277-3.2.2-2)

Send multiple labels to peer without having exchanged Multiple Labels Capability (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8277-3.2.2-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8277, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8277, so its obligations are stated where they were written.
