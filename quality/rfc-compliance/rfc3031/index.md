# RFC 3031 - Multiprotocol Label Switching Architecture

MPLS architecture. Every requirement listed for RFC 3031, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 10 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 10 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 10 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 10 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 5 | 5 of 5 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 8 | of 10 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 10 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 10 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 10 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 10 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 10 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 10 | of 12 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 10 mandatory requirements: they add up to 10. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | MPLS architecture |
| Automatic checks | Included |
| Requirements | 12 |
| Mandatory requirements (MUST level) | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 5 |
| Tests linked to requirements | 5 |
| Recorded audit verdicts | 0 |
| Fault test records | 5 |
| Summary | `rfc/short/rfc3031.md` |
| Requirement table | `rfc/requirements/rfc3031.md` |
| RFC text | `rfc/full/rfc3031.txt` |

## Automatic checks

Included in automatic checks: Multiprotocol Label Switching Architecture: ten MUST-level requirements. Four are forwarding-plane behaviors executed below ze, by Linux AF_MPLS or the VPP FIB, and are gaps in proof naming both halves: ze is an MPLS control plane that programs label operations as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go), and has no in-process MPLS packet-forwarding path. Six bind ze's own label distribution, four of them with no test under their ids (3.16-2, 3.16-3, 3.23-1, 4.1.2.2-1): label-value uniqueness per peer (S3.14), the three penultimate-hop-popping obligations (S3.16), the shim TTL field (S3.23), and the per-prefix label binding and distribution each LSR owes (S4.1.2.2).

## Published support status

**Area:** MPLS architecture

**Proof:** 2 of 10 MUST proven

**Evidence:** unit 2

**Unmet:** RFC3031-3.10-1, RFC3031-3.10-2, RFC3031-3.16-2, RFC3031-3.16-3, RFC3031-3.23-1, RFC3031-3.8-1, RFC3031-4.1.2.2-1, RFC3031-x-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 8 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **10** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC3031-3.14-1`](#rfc3031-3.14-1), [`RFC3031-3.16-1`](#rfc3031-3.16-1)

**Annotated (including scoped evidence) (8):** [`RFC3031-3.8-1`](#rfc3031-3.8-1), [`RFC3031-3.10-1`](#rfc3031-3.10-1), [`RFC3031-3.10-2`](#rfc3031-3.10-2), [`RFC3031-x-1`](#rfc3031-x-1), [`RFC3031-3.16-2`](#rfc3031-3.16-2), [`RFC3031-3.16-3`](#rfc3031-3.16-3), [`RFC3031-3.23-1`](#rfc3031-3.23-1), [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1)

## Figures

- **MUST bar**: Ze proven 2, Not built 8
- **Overview bar**: Ze MUST proven 2, Not assessed 2, Not built 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 10 |
| MUST rows kept (the MUST bar) | 10 |
| Ze proven | 2 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 8 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc3031.json](../../../data/rfc/rfc3031.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3031-3.8-1` | The "Next Hop Label Forwarding Entry" (NHLFE) is used when forwarding a labeled packet. (§3.10) | MUST | 3.10 - The NHLFE | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. On Linux, AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) looks up the NHLFE for each labeled packet in the label routes internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap installs; on the VPP FIB backend, VPP does the same from the entries internal/plugins/fib/vpp/mpls.go::addMPLSSwap programs. No test reads the installed entry back on either |
| `RFC3031-3.14-1` | In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs (S3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestRFC3031LocalFECsGetDistinctLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L60). **negative:** `unit/verify` [`TestLDPLabelExhaustionRefusesSecondBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L195). **negative:** `unit/verify` [`TestRFC3031WrappedAllocatorNeverReissuesALabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L93) |
| `RFC3031-3.10-1` | If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10) | MUST | 3.10 - The NHLFE | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. On Linux, AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) pops the stack for the pop disposition ze installs as a label route (internal/plugins/fib/kernel/mplsentry_linux.go); on the VPP FIB backend, VPP pops for the entries internal/plugins/fib/vpp/mpls.go programs. No test reads the installed disposition back on either |
| `RFC3031-3.10-2` | The processing is always based on the top label, without regard for the possibility that some number of other labels may have been "above it" in the past, or that some number of other labels may be below it at present. (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. ze keys each entry on the incoming top label, MPLSDst in internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap and inLabel in internal/plugins/fib/vpp/mpls.go::addMPLSSwap, and Linux AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) or VPP makes the top-label-only decision on each packet. No test reads the installed key back on either |
| `RFC3031-x-1` | Therefore, when a labeled packet is received with an invalid incoming label, it MUST be discarded, UNLESS it is determined by some means (not within the scope of the current document) that forwarding it unlabeled cannot cause any harm. (§3.18) | MUST | 3.18 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. Linux AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) drops a packet whose top label has no route among those internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap installs, and VPP drops on a lookup miss in the table internal/plugins/fib/vpp/mpls.go programs; ze installs no unlabeled fallback on either. No test sends an unknown label on either |
| `RFC3031-3.16-1` | An LSR which is capable of popping the label stack at all MUST do penultimate hop popping when so requested by its downstream label distribution peer (S3.16) | MUST | 3.16 | **positive:** `unit/verify` [`TestApplyRemoteBindingImplicitNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L171). **negative:** `unit/verify` [`TestApplyRemoteBindingRealLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L154) |
| `RFC3031-3.16-2` | Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16) | MUST | 3.16 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unverified: ze's LDP (internal/plugins/ldp) applies the implicit null label a peer advertises, but no test carries this id or shows that the session negotiation lets ze learn whether a neighbour can pop the label stack |
| `RFC3031-3.16-3` | A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16) | MUST | 3.16 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unverified: no test carries this id or shows that ze asks a peer for penultimate hop popping, by advertising implicit null, only where it can pop the stack itself through the pop entry it installs (internal/plugins/fib/kernel/mplsentry_linux.go) |
| `RFC3031-3.23-1` | If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23) | MUST | 3.23 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the shim is built by Linux AF_MPLS (net/mpls) or by VPP from the label routes ze installs (internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap, internal/plugins/fib/vpp/mpls.go::addMPLSSwap), and no test carries this id on either |
| `RFC3031-4.1.2.2-1` | In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST: 1. bind one or more labels to each address prefix that appears in its routing table; 2. for each such address prefix X, use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X. (S4.1.2.2) | MUST | 4.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unverified: ze's LDP binds and distributes labels for prefixes (internal/plugins/ldp/wire.go::encodeLabelMapping encodes each Label Mapping), but no test carries this id or shows a binding for every prefix in the routing table distributed to every label distribution peer |
| `RFC3031-3.14-2` | The MPLS architecture accommodates both merging and non-merging LSRs, but allows for the fact that there may be LSRs which do not support label merging. (§3.26) | MAY | 3.26 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-2.1-1` | Sometimes it is desirable to force a packet to follow a particular route which is explicitly chosen at or before the time the packet enters the network, rather than being chosen by the normal dynamic routing algorithm as the packet travels through the network. This may be done as a matter of policy, or to support traffic engineering. (S2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3031-3.8-1`](#rfc3031-3.8-1) The "Next Hop Label Forwarding Entry" (NHLFE) is used when forwarding a labeled packet. (§3.10) | {gap}, no test | gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. On Linux, AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) looks up the NHLFE for each labeled packet in the label routes internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap installs; on the VPP FIB backend, VPP does the same from the entries internal/plugins/fib/vpp/mpls.go::addMPLSSwap programs. No test reads the installed entry back on either |
| [`RFC3031-3.10-1`](#rfc3031-3.10-1) If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10) | {gap}, no test | gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. On Linux, AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) pops the stack for the pop disposition ze installs as a label route (internal/plugins/fib/kernel/mplsentry_linux.go); on the VPP FIB backend, VPP pops for the entries internal/plugins/fib/vpp/mpls.go programs. No test reads the installed disposition back on either |
| [`RFC3031-3.10-2`](#rfc3031-3.10-2) The processing is always based on the top label, without regard for the possibility that some number of other labels may have been "above it" in the past, or that some number of other labels may be below it at present. (§3.9) | {gap}, no test | gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. ze keys each entry on the incoming top label, MPLSDst in internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap and inLabel in internal/plugins/fib/vpp/mpls.go::addMPLSSwap, and Linux AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) or VPP makes the top-label-only decision on each packet. No test reads the installed key back on either |
| [`RFC3031-x-1`](#rfc3031-x-1) Therefore, when a labeled packet is received with an invalid incoming label, it MUST be discarded, UNLESS it is determined by some means (not within the scope of the current document) that forwarding it unlabeled cannot cause any harm. (§3.18) | {gap}, no test | gap in proof: two forwarding backends carry ze's labels, and the proof is missing on both halves. Linux AF_MPLS (net/mpls, linux:CONFIG_MPLS_ROUTING) drops a packet whose top label has no route among those internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap installs, and VPP drops on a lookup miss in the table internal/plugins/fib/vpp/mpls.go programs; ze installs no unlabeled fallback on either. No test sends an unknown label on either |
| [`RFC3031-3.16-2`](#rfc3031-3.16-2) Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16) | {gap}, no test | unverified: ze's LDP (internal/plugins/ldp) applies the implicit null label a peer advertises, but no test carries this id or shows that the session negotiation lets ze learn whether a neighbour can pop the label stack |
| [`RFC3031-3.16-3`](#rfc3031-3.16-3) A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16) | {gap}, no test | unverified: no test carries this id or shows that ze asks a peer for penultimate hop popping, by advertising implicit null, only where it can pop the stack itself through the pop entry it installs (internal/plugins/fib/kernel/mplsentry_linux.go) |
| [`RFC3031-3.23-1`](#rfc3031-3.23-1) If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23) | {gap}, no test | the shim is built by Linux AF_MPLS (net/mpls) or by VPP from the label routes ze installs (internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap, internal/plugins/fib/vpp/mpls.go::addMPLSSwap), and no test carries this id on either |
| [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1) In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST: 1. bind one or more labels to each address prefix that appears in its routing table; 2. for each such address prefix X, use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X. (S4.1.2.2) | {gap}, no test | unverified: ze's LDP binds and distributes labels for prefixes (internal/plugins/ldp/wire.go::encodeLabelMapping encodes each Label Mapping), but no test carries this id or shows a binding for every prefix in the routing table distributed to every label distribution peer |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC3031-3.8-1`](#rfc3031-3.8-1)

The "Next Hop Label Forwarding Entry" (NHLFE) is used when forwarding a labeled packet. (§3.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.8-1, so there is no linked test.

### [`RFC3031-3.14-1`](#rfc3031-3.14-1)

In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs (S3.14)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPLabelExhaustionRefusesSecondBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L195) | unit/verify | revert, verified |
| negative | [`TestRFC3031WrappedAllocatorNeverReissuesALabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L93) | unit/verify | mutant, verified |
| positive | [`TestRFC3031LocalFECsGetDistinctLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L60) | unit/verify | mutant, verified |

### [`RFC3031-3.10-1`](#rfc3031-3.10-1)

If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.10-1, so there is no linked test.

### [`RFC3031-3.10-2`](#rfc3031-3.10-2)

The processing is always based on the top label, without regard for the possibility that some number of other labels may have been "above it" in the past, or that some number of other labels may be below it at present. (§3.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.10-2, so there is no linked test.

### [`RFC3031-x-1`](#rfc3031-x-1)

Therefore, when a labeled packet is received with an invalid incoming label, it MUST be discarded, UNLESS it is determined by some means (not within the scope of the current document) that forwarding it unlabeled cannot cause any harm. (§3.18)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-x-1, so there is no linked test.

### [`RFC3031-3.16-1`](#rfc3031-3.16-1)

An LSR which is capable of popping the label stack at all MUST do penultimate hop popping when so requested by its downstream label distribution peer (S3.16)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestApplyRemoteBindingRealLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L154) | unit/verify | mutant, verified |
| positive | [`TestApplyRemoteBindingImplicitNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc3031_label_test.go#L171) | unit/verify | mutant, verified |

### [`RFC3031-3.16-2`](#rfc3031-3.16-2)

Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.16-2, so there is no linked test.

### [`RFC3031-3.16-3`](#rfc3031-3.16-3)

A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.16-3, so there is no linked test.

### [`RFC3031-3.23-1`](#rfc3031-3.23-1)

If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-3.23-1, so there is no linked test.

### [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1)

In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST: 1. bind one or more labels to each address prefix that appears in its routing table; 2. for each such address prefix X, use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X. (S4.1.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3031-4.1.2.2-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3031.txt |
| Source fingerprint | da270cc6acad74d1 |
| Record | rfc/extraction/rfc3031.json |
| Mapped sentences | 8 |
| Declined as scope | 73 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `2.4` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 1 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 1 | reviewed | not stated |
| `3.6` | not stated | 0 | reviewed | not stated |
| `3.7` | not stated | 1 | reviewed | not stated |
| `3.8` | Label Retention Mode | 0 | reviewed | Label Retention Mode. Indicative prose with no capitalised keyword site. RFC3031-3.8-1 (label forwarding uses the NHLFE for lookup) was read from the indicative description in section 3.10, "The "Next Hop Label Forwarding Entry" (NHLFE) is used when forwarding a labeled packet", and section 3.8 states nothing about the NHLFE; the id keeps its section because ids are permanent. |
| `3.9` | not stated | 2 | reviewed | not stated |
| `3.10` | The NHLFE | 1 | reviewed | The NHLFE. One capitalised MUST site, mapped to RFC3031-3.10-1. RFC3031-3.10-2 (only the top label determines forwarding) was read from the indicative prose of section 3.9, "The processing is always based on the top label, without regard for the possibility that some number of other labels may have been "above it" in the past, or that some number of other labels may be below it at present." |
| `3.11` | not stated | 1 | reviewed | not stated |
| `3.12` | not stated | 1 | reviewed | not stated |
| `3.13` | not stated | 0 | reviewed | not stated |
| `3.14` | not stated | 3 | reviewed | not stated |
| `3.15` | not stated | 0 | reviewed | not stated |
| `3.16` | not stated | 6 | reviewed | not stated |
| `3.17` | not stated | 0 | reviewed | not stated |
| `3.18` | not stated | 1 | reviewed | not stated |
| `3.19` | not stated | 1 | reviewed | not stated |
| `3.20` | not stated | 0 | reviewed | not stated |
| `3.21` | not stated | 0 | reviewed | not stated |
| `3.22` | not stated | 0 | reviewed | not stated |
| `3.23` | not stated | 3 | reviewed | not stated |
| `3.24` | Loop Control | 1 | reviewed | Loop Control. Its one site is the lower-case loop-detection sentence, excluded. Section 3.24 does not mention TTL handling. The two rows once listed here, RFC3031-3.24-1 and RFC3031-3.24-2, were retired 2026-09-26 (rfc/corrections/rfc3031.md): RFC 3032 section 2.4.2 states the label-stack TTL obligations. |
| `3.25` | not stated | 0 | reviewed | not stated |
| `3.25.1` | not stated | 0 | reviewed | not stated |
| `3.25.2` | not stated | 2 | reviewed | not stated |
| `3.25.3` | not stated | 1 | reviewed | not stated |
| `3.26` | not stated | 3 | reviewed | not stated |
| `3.26.1` | not stated | 0 | reviewed | not stated |
| `3.26.2` | not stated | 1 | reviewed | not stated |
| `3.26.3` | not stated | 0 | reviewed | not stated |
| `3.26.3.1` | not stated | 1 | reviewed | not stated |
| `3.26.3.2` | not stated | 1 | reviewed | not stated |
| `3.27` | not stated | 0 | reviewed | not stated |
| `3.27.1` | not stated | 0 | reviewed | not stated |
| `3.27.2` | not stated | 0 | reviewed | not stated |
| `3.27.3` | not stated | 1 | reviewed | not stated |
| `3.27.4` | not stated | 1 | reviewed | not stated |
| `3.27.5` | not stated | 1 | reviewed | not stated |
| `3.28` | not stated | 0 | reviewed | not stated |
| `3.29` | not stated | 0 | reviewed | not stated |
| `3.29.1` | not stated | 0 | reviewed | not stated |
| `3.29.2` | not stated | 0 | reviewed | not stated |
| `3.29.3` | not stated | 0 | reviewed | not stated |
| `3.30` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.1.1` | not stated | 0 | reviewed | not stated |
| `4.1.2` | not stated | 0 | reviewed | not stated |
| `4.1.2.1` | not stated | 0 | reviewed | not stated |
| `4.1.2.2` | not stated | 4 | reviewed | not stated |
| `4.1.3` | not stated | 2 | reviewed | not stated |
| `4.1.4` | not stated | 0 | reviewed | not stated |
| `4.1.5` | not stated | 1 | reviewed | not stated |
| `4.1.6` | not stated | 3 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.2.1` | not stated | 2 | reviewed | not stated |
| `4.3` | not stated | 3 | reviewed | not stated |
| `4.4` | not stated | 0 | reviewed | not stated |
| `4.5` | not stated | 1 | reviewed | not stated |
| `4.6` | not stated | 3 | reviewed | not stated |
| `4.7` | not stated | 2 | reviewed | not stated |
| `4.8` | not stated | 2 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 2 | reviewed | not stated |
| `5.1.1` | not stated | 1 | reviewed | not stated |
| `5.1.1.1` | not stated | 1 | reviewed | not stated |
| `5.1.1.2` | not stated | 0 | reviewed | not stated |
| `5.1.1.3` | not stated | 1 | reviewed | not stated |
| `5.1.1.4` | not stated | 4 | reviewed | not stated |
| `5.1.2` | not stated | 0 | reviewed | not stated |
| `5.1.2.1` | not stated | 0 | reviewed | not stated |
| `5.1.2.2` | not stated | 0 | reviewed | not stated |
| `5.1.2.3` | not stated | 1 | reviewed | not stated |
| `5.1.3` | not stated | 0 | reviewed | not stated |
| `5.1.3.1` | not stated | 0 | reviewed | not stated |
| `5.1.3.2` | not stated | 0 | reviewed | not stated |
| `5.1.4` | not stated | 0 | reviewed | not stated |
| `5.1.4.1` | not stated | 0 | reviewed | not stated |
| `5.1.4.2` | not stated | 0 | reviewed | not stated |
| `5.1.5` | not stated | 0 | reviewed | not stated |
| `5.1.5.1` | not stated | 0 | reviewed | not stated |
| `5.1.5.2` | not stated | 0 | reviewed | not stated |
| `5.1.6` | not stated | 4 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `5.2.1` | not stated | 1 | reviewed | not stated |
| `5.2.2` | not stated | 2 | reviewed | not stated |
| `5.2.3` | not stated | 3 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | (This will typically be the case when Ru and Rd are not direct neighbors.) In such cases, Rd must make sure that the binding from label to FEC is one-to-one. |
| `3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1 states the label-uniqueness prohibition in passing; Section 3.14, "Scope and Uniqueness of Labels", states it in full as "In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs", which RFC3031-3.14-1 carries. | That is, Rd MUST NOT agree with Ru1 to bind L to FEC F1, while also agreeing with some other LSR Ru2 to bind L to a different FEC F2, UNLESS Rd can always tell, when it receives a packet with incoming label L, whether the label was put on the packet by Ru1 or whether it was put on by Ru2. |
| `3.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The particular encoding technique to be used must be agreed to by both the entity which encodes the label and the entity which decodes the label. |
| `3.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru, acting as a downstream LSR, also distributes a binding of a label to FEC F, then under certain conditions, it may be required to also distribute the corresponding attribute that it received from Rd. |
| `3.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | On any given label distribution adjacency, the upstream LSR and the downstream LSR must agree on which technique is to be used. |
| `3.9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As we shall see, it is useful to have a more general model in which a labeled packet carries a number of labels, organized as a last-in, first-out stack. |
| `3.9:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Although, as we shall see, MPLS supports a hierarchy, the processing of a labeled packet is completely independent of the level of hierarchy. |
| `3.11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the ILM maps a particular label to a set of NHLFEs that contains more than one element, exactly one element of the set must be chosen before the packet is forwarded. |
| `3.12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the FTN maps a particular label to a set of NHLFEs that contains more than one element, exactly one element of the set must be chosen before the packet is forwarded. |
| `3.14:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When these conditions do not hold, the labels must be unique over the LSR which has assigned them, and we may say that the LSR is using a "per- platform label space." |
| `3.14:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, in such cases the LSR must have some means, not specified by the architecture, of determining, for a particular incoming label, which label space that label belongs to. |
| `3.16:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must pop the stack, and examine what remains of the packet. |
| `3.16:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The creation of the forwarding "fastpath" in a label switching product may be greatly aided if it is known that only a single lookup is ever required: |
| `3.16:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, some hardware switching engines may not be able to pop the label stack, so this cannot be universally required. |
| `3.19:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If one wants to ensure that traffic in a particular FEC follows a path with some specified set of properties (e.g., that the traffic does not traverse any node twice, that a specified amount of resources are available to the traffic, that the traffic follows an explicitly specified path, etc.) ordered control must be used. |
| `3.23:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In this case, the LSR at the ingress to the non-TTL LSP segment must not label switch the packet. |
| `3.23:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | This means that special procedures must be developed to support traceroute functionality, for example, traceroute packets may be forwarded using conventional hop by hop forwarding. |
| `3.24:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | All LSRs that may attach to non-TTL LSP segments will therefore be required to support a common technique for loop detection; however, use of the loop detection technique is optional. |
| `3.25.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As we shall see in section 3.26, this enables us to do label merging, without running into any cell interleaving problems, on ATM switches which can provide multipoint-to-point VPs, but which do not have the VC merge capability. |
| `3.25.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If there are more labels on the stack than can be encoded in the ATM header, the ATM encodings must be combined with the generic encapsulation. |
| `3.25.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When a labeled packet is received, the LSR must decode it to determine the current value of the label stack, then must operate on the label stack to determine the new value of the stack, and then encode the new value appropriately before transmitting the labeled packet to its next hop. |
| `3.26:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Let us say that an LSR is not capable of label merging if, for any two packets which arrive from different interfaces, or with different labels, the packets must either be transmitted out different interfaces, or must have different labels. |
| `3.26:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If a particular LSR cannot perform label merging, then if two packets in the same FEC arrive with different incoming labels, they must be forwarded with different outgoing labels. |
| `3.26:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In fact, it is difficult for an LSR to even determine how many such incoming labels it must support for a particular FEC. |
| `3.26.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When a downstream neighbor receives such a request from upstream, and the downstream neighbor does not itself support label merging, then it must in turn ask its downstream neighbor for another label for the FEC in question. |
| `3.26.3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When VC merge is used, switches are required to buffer cells from one packet until the entire packet is received (this may be determined by looking for the AAL5 end of frame indicator). |
| `3.26.3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The number required will be determined by allowing the upstream nodes to request additional VPI/VCIs from their downstream neighbors (this is again analogous to the method used with frame merge). |
| `3.27.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The set of packets which are to be sent though the LSP tunnel constitutes a FEC, and each LSR in the tunnel must assign a label to that FEC (i.e., must assign a label to the tunnel). |
| `3.27.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | R2, switching on the label, determines that P must enter the tunnel. |
| `3.27.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that in this example, R2 and R21 must be IGP neighbors, but R2 and R3 need not be. |
| `4.1.2.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | There is also one circumstance in which an LSR must distribute a label binding for an address prefix, even if it is not the LSR which bound that label to that address prefix: |
| `4.1.2.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If R1 uses BGP to distribute a route to X, naming some other LSR R2 as the BGP Next Hop to X, and if R1 knows that R2 has assigned label L to X, then R1 must distribute the binding between L and X to any BGP peer to which it distributes that route. |
| `4.1.2.2:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | These rules are intended only to indicate which label bindings must be distributed by a given LSR to which other LSRs. |
| `4.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | At that point, the LSP must end and the best match algorithm must be performed again. |
| `4.1.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In this situation, packet P can be label Switched until it reaches R2, but since R2 has performed route aggregation, it must execute the best match algorithm to find P's FEC. |
| `4.1.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If LSR Ru, by consulting its ILM, sees that labeled packet P must be forwarded next to Rd, but that Rd has distributed a binding of Implicit NULL to the corresponding address prefix, then instead of replacing the value of the label on top of the label stack, Ru pops the label stack, and then forwards the resulting packet to Rd. |
| `4.1.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | There are situations in which an LSP Ingress, Ri, knows that packets of several different FECs must all follow the same LSP, terminating at, say, LSP Egress Re. |
| `4.1.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | - If the network is running a link state routing algorithm, and all nodes in the area support MPLS, then the routing algorithm provides Ri with enough information to determine the routers through which packets in that FEC must leave the routing domain or area. |
| `4.1.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | - If the network is running BGP, Ri may be able to determine that the packets in a particular FEC must leave the network via some particular router which is the "BGP Next Hop" for that FEC. |
| `4.2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the transmit endpoint of the tunnel wishes to put a labeled packet into the tunnel, it must first replace the label value at the top of the stack with a label value that was distributed to it by the tunnel's receive endpoint. |
| `4.2.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must push on the label which corresponds to the tunnel itself, as distributed to it by the next hop along the tunnel. |
| `4.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Ru must push L2 and then L1 onto the packet's label stack, and then forward the packet to Rd; |
| `4.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When Ru distributes label bindings for X to its label distribution peers, it must include L2 as the stack attribute. |
| `4.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Whenever the stack attribute changes (possibly as a result of a change in Ru's LSP next hop for X), Ru must distribute the new stack attribute. |
| `4.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Since conventional ATM switches do not support multipoint-to-point connections, there must be procedures to ensure that each LSP is realized as a point-to-point VC. |
| `4.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, there must be some means of ensuring that the transit traffic will be delivered from Border Router to Border Router by the interior routers. |
| `4.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then before sending packet P to I1, B1 must create a label stack for P, then push on label L1, and then push on label L2. |
| `4.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then before sending packet P to I1, B1 must replace the label at the top of the label stack with L1, and then push on label L2. |
| `4.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the transmit endpoint of the tunnel wishes to put a labeled packet into the tunnel, it must first replace the label value at the top of the stack with a label value that was distributed to it by the tunnel's receive endpoint. |
| `4.7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must push on the label which corresponds to the tunnel itself, as distributed to it by the next hop along the tunnel. |
| `4.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The tree along which a particular multicast packet must get forwarded depends in general on the packet's source address and its destination address. |
| `4.8:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | (If the node in question is on a LAN, and has siblings on that LAN, it must also distribute the binding to its siblings. |
| `5.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The downstream LSR must perform: |
| `5.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The upstream LSR must perform: |
| `5.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Irrespective of the particular procedure that is used, if a label binding for a particular address prefix has been distributed by a downstream LSR Rd to an upstream LSR Ru, and if at any time the attributes (as defined above) of that binding change, then Rd must inform Ru of the new attributes. |
| `5.1.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Whenever these conditions hold, Rd must bind a label to X and distribute that binding to Ru. |
| `5.1.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that if X is not in Rd's routing table, or if Rd is not a label distribution peer of Ru with respect to X, then Rd must inform Ru that it cannot provide a binding at this time. |
| `5.1.1.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that if X is not in Rd's routing table and a binding for X is not obtainable via Rd's next hop for X, or if Rd is not a label distribution peer of Ru with respect to X, then Rd must inform Ru that it cannot provide a binding at this time. |
| `5.1.1.4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, if the only condition that fails to hold is that Rn has not yet provided a label to Rd, then Rd must defer any response to Ru until such time as it has receiving a binding from Rn. |
| `5.1.1.4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd has distributed a label binding for address prefix X to Ru, and at some later time, any attribute of the label binding changes, then Rd must redistribute the label binding to Ru, with the new attribute. |
| `5.1.1.4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | It must do this even though Ru does not issue a new Request. |
| `5.1.2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd receives such a request from Ru, for an address prefix for which Rd has already distributed Ru a label, Rd shall assign a new (distinct) label, bind it to X, and distribute that binding. |
| `5.1.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When LSR Rd decides to break the binding between label L and address prefix X, then this unbinding must be distributed to all LSRs to which the binding was distributed. |
| `5.1.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | It is required that the unbinding of L from X be distributed by Rd to a LSR Ru before Rd distributes to Ru any new binding of L to any other address prefix Y, where X != Y. |
| `5.1.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If LSR R1 has a label distribution adjacency to LSR R2, and has received label bindings from LSR R2 via that adjacency, then if adjacency is brought down by either peer (whether as a result of failure or as a matter of normal operation), all bindings received over that adjacency must be considered to have been withdrawn. |
| `5.1.6:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As long as the relevant label distribution adjacency remains in place, label bindings that are withdrawn must always be withdrawn explicitly. |
| `5.2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru and Rd are label distribution peers, and both support label merging, one of the following schemes must be used: |
| `5.2.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Since there is no multipoint-to-point capability, the LSPs must be realized as point-to-point VCs, which means that there needs to be three such VCs for address prefix X: <R1, R2, R3, R4>, <R2, R3, R4>, and <R3, R4>. |
| `5.2.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Therefore, if R1 and R2 are MPLS peers, and either is an LSR which is implemented using conventional ATM switching hardware (i.e., no cell interleave suppression), or is otherwise incapable of performing label merging, the MPLS scheme in use between R1 and R2 must be one of the following: |
| `5.2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Each must state whether it supports label merging. |
| `5.2.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd does not support label merging, Rd must choose either the PulledUnconditional procedure or the PulledConditional procedure. |
| `5.2.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru does not support label merging, but Rd does, Ru must choose either the RequestRetry or RequestNoRetry procedure. |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the RFC Editor's full copyright statement, which constrains republication of the document and states no protocol obligation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3031, so its obligations are stated where they were written.
