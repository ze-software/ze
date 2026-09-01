# RFC 3209 - RSVP-TE: Extensions to RSVP for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 3209, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 38.5% | 5 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 46.2% | 6 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 15.4% | 2 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3209.md` |
| Requirement shard | `rfc/requirements/rfc3209.md` |
| RFC text | `rfc/full/rfc3209.txt` |

## Enrolment

Enrolled: RSVP-TE (RFC 3209): PATH/RESV signaling, ERO, LABEL, SE make-before-break, soft-state; 5 MET + 6 single-polarity positive + 2 gap (ERO strict-hop adjacency check, IP Router Alert option)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

PATH and RESV signaling, ERO routing, bandwidth admission, SE-style make-before-break, soft-state refresh/expiry, teardown.

**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc3209.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3209.md): [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1) -- the transit next-hop selector (engine.go) ignores the ERO L-bit and does no adjacency check, so a non-adjacent strict hop is not rejected; and RFC3209-x-1 -- the raw IP transport (transport_linux.go) sends PATH without the IP Router Alert option (same transport gap as RFC 2205). Cross-vendor interop remains constrained by available open daemons.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC3209-4.1-1`](#rfc3209-4.1-1), [`RFC3209-4.1-2`](#rfc3209-4.1-2), [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1), [`RFC3209-6-1`](#rfc3209-6-1), [`RFC3209-2.5-1`](#rfc3209-2.5-1)

**Annotated instead of tested (8):** [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1), [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1), [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2), [`RFC3209-4.2-1`](#rfc3209-4.2-1), [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1), [`RFC3209-4.1-3`](#rfc3209-4.1-3), [`RFC3209-6-2`](#rfc3209-6-2), [`RFC3209-x-1`](#rfc3209-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3209-4.6.1-1` | SESSION Object reserved field MUST be zero (S4.6.1, Wire Format) | MUST | 4.6.1 | **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L338). **negative:** no negative test. **{single-polarity}:** the encoder writes the SESSION reserved octet as 0 on every SESSION it emits, but the decoder never reads that octet, so a non-zero-reserved reject test is not meaningful (internal/plugins/rsvpte/wire.go:238, :243) |
| `RFC3209-4.6.2-1` | SENDER_TEMPLATE Object reserved field MUST be zero (S4.6.2, Wire Format) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L368). **negative:** no negative test. **{single-polarity}:** the encoder zeroes the 2-byte reserved field before LSP ID on every SENDER_TEMPLATE, and decodeSenderTemplate never inspects it (internal/plugins/rsvpte/wire.go:270, :275-276) |
| `RFC3209-4.6.1-2` | SESSION C-Type 7 (LSP_TUNNEL_IPv4) MUST be used for RSVP-TE LSP tunnels (S4.6.1) | MUST | 4.6.1 | **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L337). **negative:** no negative test. **{single-polarity}:** encodeSessionIPv4 always stamps C-Type 7, but DecodeMessage dispatches SESSION by Class-Num only and never rejects a wrong C-Type, so no negative decode-reject exists (internal/plugins/rsvpte/wire.go:240, :746-752) |
| `RFC3209-4.2-1` | LABEL_REQUEST object MUST be present in PATH messages to request label allocation (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L45). **negative:** no negative test. **{single-polarity}:** buildPath unconditionally appends a LABEL_REQUEST to every PATH it composes, and handlePath does not reject a received PATH lacking one, so only the positive is meaningful (internal/plugins/rsvpte/build.go:92) |
| `RFC3209-4.1-1` | LABEL object MUST be present in RESV messages; each hop allocates and reports its label upstream (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L75). **negative:** `unit/verify` [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L364) |
| `RFC3209-4.1-2` | Upper 12 bits of the LABEL value MUST be zero for MPLS (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L296). **negative:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L297) |
| `RFC3209-4.3.4-1` | ERO processing: transit node MUST remove itself (first subobject) from ERO before forwarding PATH (S4.3.4) | MUST | 4.3.4 | **positive:** `unit/verify` [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L195). **negative:** `unit/verify` [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L403) |
| `RFC3209-4.3.4.1-1` | ERO strict hop: next hop MUST be directly connected; reject PATH if not adjacent (S4.3.4.1) | MUST | 4.3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nextHopFromERO takes rem[0] as next hop regardless of the L (loose/strict) bit and performs no adjacency check; the Loose flag is decoded and only used for display/config, so a non-adjacent strict hop is neither validated nor rejected (internal/plugins/rsvpte/wire.go:455, :467, engine.go:407-416) |
| `RFC3209-4.1-3` | Reserved label values 0-15: only label 3 (Implicit NULL) MAY be allocated for penultimate hop popping (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestLSPTableAllocateSkipsReservedLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/fsm_test.go#L70). **negative:** no negative test. **{single-polarity}:** the local label allocator starts at firstDynamicLabel=1000 and wraps back to 1000, so ze never hands out any label in 0-15 and there is no receive path that would allocate a reserved label (internal/plugins/rsvpte/fsm.go:184, :205, :215-217) |
| `RFC3209-6-1` | Make-before-break: old and new LSPs MUST have the same SESSION (same Tunnel Endpoint, Tunnel ID, Extended Tunnel ID); only LSP ID differs (S6) | MUST | 6 | **positive:** `unit/verify` [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/reroute_test.go#L41). **negative:** `unit/verify` [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/admission_se_test.go#L35) |
| `RFC3209-6-2` | SE (Shared Explicit) style MUST be used for make-before-break rerouting (S6) | MUST | 6 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L78). **negative:** no negative test. **{single-polarity}:** every RESV ze originates carries STYLE = Shared Explicit (18) and admission always applies SE sharing semantics; ze never emits a Fixed-Filter style for an LSP tunnel (internal/plugins/rsvpte/engine.go:272, build.go:126, wire.go:680) |
| `RFC3209-x-1` | PATH messages MUST include the Router Alert IP option (Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** the raw IPv4 socket (proto 46) is opened and used for Sendto with no IP_ROUTER_ALERT socket option and no per-packet IP option, so emitted PATH datagrams carry no Router Alert (the same transport gap as RFC2205-x-1) (internal/plugins/rsvpte/transport_linux.go:35-58, :60-69) |
| `RFC3209-2.5-1` | Both PATH and RESV messages MUST be refreshed periodically for soft-state maintenance (S2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L56). **negative:** `unit/verify` [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L21) |
| `RFC3209-x-2` | Admission control failure SHOULD generate PathErr with Error Code 1 (Admission Control) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4-1` | RRO MAY be included in PATH and RESV to record the actual path taken (S4.4) | MAY | 4.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1) ERO strict hop: next hop MUST be directly connected; reject PATH if not adjacent (S4.3.4.1) | {gap}, no test | nextHopFromERO takes rem[0] as next hop regardless of the L (loose/strict) bit and performs no adjacency check; the Loose flag is decoded and only used for display/config, so a non-adjacent strict hop is neither validated nor rejected (internal/plugins/rsvpte/wire.go:455, :467, engine.go:407-416) |
| [`RFC3209-x-1`](#rfc3209-x-1) PATH messages MUST include the Router Alert IP option (Transport) | {gap}, no test | the raw IPv4 socket (proto 46) is opened and used for Sendto with no IP_ROUTER_ALERT socket option and no per-packet IP option, so emitted PATH datagrams carry no Router Alert (the same transport gap as RFC2205-x-1) (internal/plugins/rsvpte/transport_linux.go:35-58, :60-69) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1)

SESSION Object reserved field MUST be zero (S4.6.1, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L338) | unit/verify | unproven |

### [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1)

SENDER_TEMPLATE Object reserved field MUST be zero (S4.6.2, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L368) | unit/verify | unproven |

### [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2)

SESSION C-Type 7 (LSP_TUNNEL_IPv4) MUST be used for RSVP-TE LSP tunnels (S4.6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L337) | unit/verify | unproven |

### [`RFC3209-4.2-1`](#rfc3209-4.2-1)

LABEL_REQUEST object MUST be present in PATH messages to request label allocation (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L45) | unit/verify | unproven |

### [`RFC3209-4.1-1`](#rfc3209-4.1-1)

LABEL object MUST be present in RESV messages; each hop allocates and reports its label upstream (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L364) | unit/verify | unproven |
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L75) | unit/verify | unproven |

### [`RFC3209-4.1-2`](#rfc3209-4.1-2)

Upper 12 bits of the LABEL value MUST be zero for MPLS (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L297) | unit/verify | unproven |
| positive | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L296) | unit/verify | unproven |

### [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1)

ERO processing: transit node MUST remove itself (first subobject) from ERO before forwarding PATH (S4.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L403) | unit/verify | unproven |
| positive | [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L195) | unit/verify | unproven |

### [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1)

ERO strict hop: next hop MUST be directly connected; reject PATH if not adjacent (S4.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.3.4.1-1, so no unit is bound to it.

### [`RFC3209-4.1-3`](#rfc3209-4.1-3)

Reserved label values 0-15: only label 3 (Implicit NULL) MAY be allocated for penultimate hop popping (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLSPTableAllocateSkipsReservedLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/fsm_test.go#L70) | unit/verify | unproven |

### [`RFC3209-6-1`](#rfc3209-6-1)

Make-before-break: old and new LSPs MUST have the same SESSION (same Tunnel Endpoint, Tunnel ID, Extended Tunnel ID); only LSP ID differs (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/admission_se_test.go#L35) | unit/verify | unproven |
| positive | [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/reroute_test.go#L41) | unit/verify | unproven |

### [`RFC3209-6-2`](#rfc3209-6-2)

SE (Shared Explicit) style MUST be used for make-before-break rerouting (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L78) | unit/verify | unproven |

### [`RFC3209-x-1`](#rfc3209-x-1)

PATH messages MUST include the Router Alert IP option (Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-x-1, so no unit is bound to it.

### [`RFC3209-2.5-1`](#rfc3209-2.5-1)

Both PATH and RESV messages MUST be refreshed periodically for soft-state maintenance (S2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L21) | unit/verify | unproven |
| positive | [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L56) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 3209, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3209, so its obligations are stated where they were written.
