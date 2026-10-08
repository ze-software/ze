# RFC 3209 - RSVP-TE: Extensions to RSVP for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 3209, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 32.2% | 19 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.9% | 7 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 59 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 88.3% | 53 of 60 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 62 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 3.4% | 2 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 52.5% | 31 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 17 | of 59 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 62 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 31 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 60 |
| Tagged units | 60 |
| Recorded audit verdicts | 17 |
| Discrimination records | 53 |
| Summary | `rfc/short/rfc3209.md` |
| Requirement shard | `rfc/requirements/rfc3209.md` |
| RFC text | `rfc/full/rfc3209.txt` |

## Enrolment

Enrolled: RSVP-TE (RFC 3209): PATH/RESV signaling, ERO, LABEL, SE make-before-break, soft-state. The 2026-09-21 extraction walk added the 50 MUST-level and SHALL-level rows the earlier summary did not list, so the checklist now carries 65 rows and every one of them is sourced to a sentence of the document. The added rows carry no test.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

PATH and RESV signaling, ERO routing, bandwidth admission, SE-style make-before-break, soft-state refresh/expiry, teardown.

**What the ledger says remains**

Strict-hop validation and native loose-hop next-hop expansion are implemented in routing.go and proven for [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1) and [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1) by tagged PATH wire tests; this is not CSPF. Independent-peer evidence is freeRouter only, in the Docker suite `./le test integration interop-rsvpte` ([`internal/le/interoplab/rsvpte/checkers.go`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/rsvpte/checkers.go)). The Linux carrier `TestRSVPFreeRouterInterop` (freeRouter ingress, Ze egress) is a manual check with no recorded pass, so it is not evidence here. In that suite a Ze transit expands a loose hop freeRouter originated (`transit-loose-ero-expansion`), forwards a strict hop with the ERO trimmed (`transit-strict-hop-forwarded`) and refuses an outside strict hop with PathErr 24/2 (`transit-strict-hop-outside-refused`), and Ze-originated PATH, PathErr, ResvErr and ResvTear are captured by a Ze node after freeRouter parsed and relayed them. freeRouter names its configured ERO hops loose and prepends one strict subobject for its own next hop unless the first hop is already strict (`ipFwdTab.fillRsvpFrst`). It routes each hop without reading the strict bit (`rtrRsvpIface.getHop`), so it does not itself enforce strict hops, and it never originates PathErr, ResvErr or ResvTear: these scenarios prove freeRouter accepts and relays what Ze sends, not that a second implementation agrees on strict-hop validation. Two scenarios (`transit-resv-increase-refused-in-place`, `transit-ff-resv-unknown-sender`) turn on a knob of a test-only freeRouter patch, [`test/interop-rsvpte/freertr/ze-interop-resv.patch`](https://github.com/ze-software/ze/blob/main/test/interop-rsvpte/freertr/ze-interop-resv.patch), that alters the RESV its egress originates. Non-RSVP-hop knowledge, Hello, ATM/Frame Relay label ranges, IPv6 LSP_TUNNEL objects, resource-affinity validation and per-sender policing remain absent. Existing planning records do not authorize adding them. The IPv4 SESSION test does not prove [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2). Path-MTU programming exists, but bounded exact-fit and oversized-DF evidence does not prove Section 2.6's full fragmentation algorithm. Kernel implementation and proof must be assessed separately; the Go RSVP producer alone cannot establish a Linux-wide absence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 19 | one part of the gated population |
| Annotated (including scoped evidence) | 40 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (19):** [`RFC3209-4.2-1`](#rfc3209-4.2-1), [`RFC3209-4.1-1`](#rfc3209-4.1-1), [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1), [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1), [`RFC3209-3-1`](#rfc3209-3-1), [`RFC3209-4.2.1-1`](#rfc3209-4.2.1-1), [`RFC3209-4.2.4-2`](#rfc3209-4.2.4-2), [`RFC3209-4.2.4-3`](#rfc3209-4.2.4-3), [`RFC3209-4.3.3-1`](#rfc3209-4.3.3-1), [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1), [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1), [`RFC3209-4.4.1-1`](#rfc3209-4.4.1-1), [`RFC3209-4.4.3-1`](#rfc3209-4.4.3-1), [`RFC3209-4.4.3-2`](#rfc3209-4.4.3-2), [`RFC3209-4.4.3-3`](#rfc3209-4.4.3-3), [`RFC3209-4.4.3-4`](#rfc3209-4.4.3-4), [`RFC3209-4.7.3-1`](#rfc3209-4.7.3-1), [`RFC3209-4.7.4-3`](#rfc3209-4.7.4-3), [`RFC3209-6-1`](#rfc3209-6-1)

**Annotated (including scoped evidence) (40):** [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1), [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1), [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2), [`RFC3209-2.6-1`](#rfc3209-2.6-1), [`RFC3209-2.6-2`](#rfc3209-2.6-2), [`RFC3209-2.6-3`](#rfc3209-2.6-3), [`RFC3209-2.6-4`](#rfc3209-2.6-4), [`RFC3209-2.6-5`](#rfc3209-2.6-5), [`RFC3209-3-2`](#rfc3209-3-2), [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1), [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2), [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3), [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1), [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2), [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3), [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1), [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2), [`RFC3209-4.2.4-1`](#rfc3209-4.2.4-1), [`RFC3209-4.2.4-4`](#rfc3209-4.2.4-4), [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1), [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2), [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1), [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2), [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1), [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2), [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3), [`RFC3209-5.3-1`](#rfc3209-5.3-1), [`RFC3209-5.3-2`](#rfc3209-5.3-2), [`RFC3209-5.3-3`](#rfc3209-5.3-3), [`RFC3209-5.3-4`](#rfc3209-5.3-4), [`RFC3209-5.3-5`](#rfc3209-5.3-5), [`RFC3209-5.3-6`](#rfc3209-5.3-6), [`RFC3209-5.3-7`](#rfc3209-5.3-7), [`RFC3209-5.3-8`](#rfc3209-5.3-8), [`RFC3209-5.3-9`](#rfc3209-5.3-9), [`RFC3209-5.3-10`](#rfc3209-5.3-10), [`RFC3209-5.3-11`](#rfc3209-5.3-11), [`RFC3209-5.4-1`](#rfc3209-5.4-1), [`RFC3209-5.4-2`](#rfc3209-5.4-2), [`RFC3209-6-2`](#rfc3209-6-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3209-4.6.1-1` | \| IPv4 tunnel end point address \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| MUST be zero \| Tunnel ID \| (§4.6.1.1) | MUST | 4.6.1.1 | **positive:** `unit/verify` [`TestRFC3209SessionReservedZeroOverDirtyBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L59). **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L342). **negative:** no negative test. **{single-polarity}:** the encoder writes the SESSION reserved octet as 0 on every SESSION it emits, but the decoder never reads that octet, so a non-zero-reserved reject test is not meaningful (internal/plugins/rsvpte/wire.go:238, :243) |
| `RFC3209-4.6.2-1` | Class = SENDER_TEMPLATE, LSP_TUNNEL_IPv4 C-Type = 7 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| IPv4 tunnel sender address \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| MUST be zero \| LSP ID \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ (S4.6.2, Wire Format) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L372). **negative:** no negative test. **{single-polarity}:** the encoder zeroes the 2-byte reserved field before LSP ID on every SENDER_TEMPLATE, and decodeSenderTemplate never inspects it (internal/plugins/rsvpte/wire.go:270, :275-276) |
| `RFC3209-4.6.1-2` | \| IPv6 tunnel end point address \| + + \| (16 bytes) \| + + \| \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| MUST be zero \| Tunnel ID \| (§4.6.1.2) | MUST | 4.6.1.2 | **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L341). **negative:** no negative test. **{single-polarity}:** Ze emits no LSP_TUNNEL_IPv6 SESSION (wire.go::encodeSessionIPv4 writes C-Type 7 only), so no output of Ze can carry a non-zero IPv6 reserved field to refuse; the tagged test proves the IPv4 object and the IPv6 codec is scheduled in plan/spec-rsvpte-ipv6-lsp-tunnel.md |
| `RFC3209-4.2-1` | To establish an LSP tunnel the sender creates a Path message with a LABEL_REQUEST object. The LABEL_REQUEST object indicates that a label binding for this path is requested and provides an indication of the network layer protocol that is to be carried over this path. (§4.2.4) | MUST | 4.2.4 | **positive:** `unit/verify` [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L45). **positive:** `unit/verify` [`TestRFC3209IngressPathCarriesLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L46). **negative:** `unit/verify` [`TestRFC3209IngressPathNeverWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L55). **negative:** `unit/verify` [`TestRFC3209NoLabelWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L222) |
| `RFC3209-4.1-1` | The label for a sender MUST immediately follow the FILTER_SPEC for that sender in the Resv message (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L75). **negative:** `unit/verify` [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L463) |
| `RFC3209-4.1-2` | Labels MAY be carried in Resv messages (S4.1) | MAY | 4.1 | **positive:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L300). **negative:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L301) |
| `RFC3209-4.3.4-1` | The node determines whether it is topologically adjacent to the abstract node described by the second subobject. If so, the node selects a particular next hop which is a member of the abstract node. The node then deletes the first subobject and continues processing with section 4.3.4.2. (§4.3.4.1) | MUST | 4.3.4.1 | **positive:** `unit/verify` [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L291). **negative:** `unit/verify` [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L509). **negative:** `unit/verify` [`TestRFC3209TransitNotAdjacentToSecondSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L88) |
| `RFC3209-4.3.4.1-1` | 1) The node receiving the RSVP message MUST first evaluate the first subobject. (S4.3.4.1) | MUST | 4.3.4.1 | **positive:** `unit/verify` [`TestRFC3209TransitEvaluatesFirstEROSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L123). **negative:** `unit/verify` [`TestRFC3209TransitNoFirstEROSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L145). **negative:** `unit/verify` [`TestRFC3209TransitRefusesForeignFirstSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L71) |
| `RFC3209-2.6-1` | an LSR MUST execute the following algorithm (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer executes the labeled-datagram forwarding algorithm |
| `RFC3209-2.6-2` | (a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (S2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer breaks an oversized fragmentable labeled datagram into fragments no larger than M |
| `RFC3209-2.6-3` | (b) each fragment MUST be labeled and then forwarded. (S2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer labels and forwards the fragments of an oversized labeled datagram |
| `RFC3209-2.6-4` | When the size of an IPv4 datagram (without labels) exceeds the value of M, If the DF bit is not set in the IPv4 header, then (a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (b) each fragment MUST be labeled and then forwarded. If the DF bit is set in the IPv4 header, then (a) the datagram MUST NOT be forwarded (§2.6) | MUST NOT | 2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux MPLS forwarding (net/mpls); internal/plugins/rsvpte/fib.go::programSwap installs the label operation and no MTU, so the no-forward decision reads nothing Ze writes |
| `RFC3209-2.6-5` | When the size of an IPv6 datagram (without labels) exceeds the value of M, (a) the datagram MUST NOT be forwarded (S2.6) | MUST NOT | 2.6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux MPLS forwarding (net/mpls); internal/plugins/rsvpte/fib.go::programSwap installs the label operation and no MTU, so the IPv6 no-forward decision reads nothing Ze writes |
| `RFC3209-3-1` | In Resv messages they MUST appear after the associated FILTER_SPEC and prior to any subsequent FILTER_SPEC. (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC3209LabelFollowsFilterSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L254). **positive:** `unit/verify` [`TestRFC3209ResvLabelFollowsFilterSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L47). **negative:** `unit/verify` [`TestRFC3209ResvNeverCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L58) |
| `RFC3209-3-2` | The ordering of these objects is not important, so an implementation MUST be prepared to accept objects in any order (S3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L187). **negative:** no negative test. **{single-polarity}:** DecodeMessage reads objects in any order, and no violating input exists for a MUST-accept |
| `RFC3209-4.1.1.1-1` | If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.1.1.1-2` | Note that if a node intends to police individual senders to a session, it MUST assign unique labels to those senders. (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze polices no sender; plan/spec-rsvpte-sender-policing.md |
| `RFC3209-4.1.1.1-3` | If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.1-1` | This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC3209LabelRequestReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L49). **negative:** `unit/verify` [`TestRFC3209LabelRequestReservedIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L62) |
| `RFC3209-4.2.2-1` | This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.2-2` | If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.2-3` | If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.3-1` | This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§4.2.3) | MUST | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.3-2` | The DLCI MUST be right justified in this field and unused bits MUST be set to 0. (S4.2.3) | MUST | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| `RFC3209-4.2.4-1` | A receiver that accepts a LABEL_REQUEST object MUST include a LABEL object in Resv messages pertaining to that Path message (S4.2.4) | MUST | 4.2.4 | **positive:** `unit/verify` [`TestRFC3209EgressResvCarriesLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L30). **negative:** no negative test. **{single-polarity}:** the egress always encodes a LABEL in the RESV it answers a LABEL_REQUEST PATH with (handlePathEgress, buildResv), and no input makes it accept the request and omit the label |
| `RFC3209-4.2.4-2` | If a LABEL_REQUEST object was not present in the Path message, a node MUST NOT include a LABEL object in a Resv message for that Path message's session and PHOP (S4.2.4) | MUST NOT | 4.2.4 | **positive:** `unit/verify` [`TestRFC3209LabelFollowsLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L205). **negative:** `unit/verify` [`TestRFC3209NoLabelWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L221) |
| `RFC3209-4.2.4-3` | A node that sends a LABEL_REQUEST object MUST be ready to accept and correctly process a LABEL object in the corresponding Resv messages (S4.2.4) | MUST | 4.2.4 | **positive:** `unit/verify` [`TestRFC3209IngressProcessesResvLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L48). **negative:** `unit/verify` [`TestRFC3209IngressRefusesResvWithoutLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L74) |
| `RFC3209-4.2.4-4` | A node which receives and forwards a Path message each with a LABEL_REQUEST object, MUST copy the L3PID from the received LABEL_REQUEST object to the forwarded LABEL_REQUEST object. (S4.2.4) | MUST | 4.2.4 | **positive:** `unit/verify` [`TestRFC3209TransitCopiesL3PID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L106). **negative:** no negative test. **{single-polarity}:** handlePathTransit copies the received LABEL_REQUEST whole into the relayed PATH, so a wrong L3PID never appears and the negative is the same assertion |
| `RFC3209-4.2.5-1` | This means that if a router has a neighbor that is known to not be RSVP capable, the router MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers. (S4.2.5) | MUST NOT | 4.2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| `RFC3209-4.3.3-1` | The Length MUST be at least 4, and MUST be a multiple of 4. (S4.3.3) | MUST | 4.3.3 | **positive:** `unit/verify` [`TestRFC3209EROSubobjectLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L70). **negative:** `unit/verify` [`TestRFC3209EROSubobjectLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L87) |
| `RFC3209-4.3.3.1-1` | The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node (S4.3.3.1) | MUST | 4.3.3.1 | **positive:** `unit/verify` [`TestRFC3209StrictHopReachedDirectly`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L60). **negative:** `unit/verify` [`TestRFC3209StrictHopThroughOutsideNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L81) |
| `RFC3209-4.3.4.2-1` | Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node. (S4.3.4.2) | MUST | 4.3.4.2 | **positive:** `unit/verify` [`TestRFC3209InteriorHopStaysInAbstractNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L118). **negative:** `unit/verify` [`TestRFC3209InteriorHopLeavingAbstractNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L146) |
| `RFC3209-4.4.1-1` | The length MUST always be a multiple of 4, and at least 4. (S4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC3209RROSubobjectLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L99). **negative:** `unit/verify` [`TestRFC3209RROSubobjectLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L116) |
| `RFC3209-4.4.3-1` | The newly added subobject MUST be this router's IP address. (S4.4.3) | MUST | 4.4.3 | **positive:** `unit/verify` [`TestRFC3209EngineRecordsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L117). **positive:** `unit/verify` [`TestRFC3209RRONewSubobjectIsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L13). **negative:** `unit/verify` [`TestRFC3209EngineNeverRecordsNeighborAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L137). **negative:** `unit/verify` [`TestRFC3209RRONoSubobjectWithoutOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L24) |
| `RFC3209-4.4.3-2` | A node MUST NOT push on a Label Record subobject without also pushing on an IPv4 or IPv6 subobject (S4.4.3) | MUST NOT | 4.4.3 | **positive:** `unit/verify` [`TestRFC3209RROLabelRecordFollowsAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L32). **negative:** `unit/verify` [`TestRFC3209RRONoLabelRecordWithoutAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L43) |
| `RFC3209-4.4.3-3` | If the newly added subobject causes the RRO to be too big to fit in a Path (or Resv) message, the RRO object SHALL be dropped from the message and message processing continues as normal. (S4.4.3) | SHALL | 4.4.3 | **positive:** `unit/verify` [`TestRFC3209PathRRODroppedWhenTooBig`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L171). **positive:** `unit/verify` [`TestRFC3209RRODroppedWhenTooBig`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L44). **negative:** `unit/verify` [`TestRFC3209PathRROKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L183). **negative:** `unit/verify` [`TestRFC3209RROKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L63) |
| `RFC3209-4.4.3-4` | A received Path message without an RRO indicates that the sender node no longer needs route recording.  Subsequent Resv messages SHALL NOT contain an RRO. (S4.4.3) | SHALL NOT | 4.4.3 | **positive:** `unit/verify` [`TestRFC3209PathRROWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L285). **negative:** `unit/verify` [`TestRFC3209PathRROForwarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L330) |
| `RFC3209-4.6.2-2` | 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| \| + + \| IPv6 tunnel sender address \| + + \| (16 bytes) \| + + \| \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| MUST be zero \| LSP ID \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ (S4.6.2) | MUST | 4.6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no IPv6 SESSION or SENDER_TEMPLATE codec; plan/spec-rsvpte-ipv6-lsp-tunnel.md |
| `RFC3209-4.7.3-1` | The Length MUST always be a multiple of 4 and MUST be at least 8. (S4.7.3) | MUST | 4.7.3 | **positive:** `unit/verify` [`TestRFC3209SessionAttributeLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L127). **negative:** `unit/verify` [`TestRFC3209SessionAttributeLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L141) |
| `RFC3209-4.7.4-1` | In order to be validated a link MUST pass the three tests below. (S4.7.4) | MUST | 4.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze decodes no affinity mask and validates no link against one; plan/spec-rsvpte-resource-affinities.md |
| `RFC3209-4.7.4-2` | When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4) | MUST | 4.7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze decodes no affinity mask and validates no link against one; plan/spec-rsvpte-resource-affinities.md |
| `RFC3209-4.7.4-3` | All RSVP routers, whether they support the SESSION_ATTRIBUTE object or not, SHALL forward the object unmodified (S4.7.4) | SHALL | 4.7.4 | **positive:** `unit/verify` [`TestRFC3209SessionAttributeRelayedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L165). **negative:** `unit/verify` [`TestRFC3209SessionAttributeNotInserted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L195) |
| `RFC3209-5.2.2-1` | This value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node and otherwise remains the same. (S5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.2.2-2` | This field MUST NOT be set to zero (0). (S5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.2.2-3` | This field MUST be set to zero (0) when no value has ever been seen from the neighbor. (S5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-1` | This value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor. (S5.3) | MUST NOT | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-2` | On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-3` | If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-4` | If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, then the node MUST treat the neighbor as if communication has been lost. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-5` | On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-6` | The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-7` | If the neighbor advertises a wrong value in the Dst_Instance field, then a node MUST treat the neighbor as if communication has been lost. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-8` | If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, then a node MUST presume that it cannot communicate with the neighbor. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-9` | If a node does re-initiate it MUST use a Src_Instance value different than the one advertised in the previous HELLO message. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-10` | This new value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.3-11` | If a new instance value has not been received from the neighbor, then the node MUST advertise zero in the Dst_instance value field. (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.4-1` | When the links between neighbors are numbered, then Hellos MUST be run on each link and the previously described mechanisms apply. (S5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-5.4-2` | When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| `RFC3209-6-1` | When an ingress node with an established path wants to change that path, it forms a new Path message as follows. The existing SESSION object is used. In particular the Tunnel_ID and Extended_Tunnel_ID are unchanged. The ingress node picks a new LSP_ID to form a new SENDER_TEMPLATE. (§4.6.4) | MUST | 4.6.4 | **positive:** `unit/verify` [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_reroute_test.go#L41). **positive:** `unit/verify` [`TestRFC3209ReroutePathKeepsSessionNewLSPID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L195). **negative:** `unit/verify` [`TestRFC3209RerouteNeverReusesLSPID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L211). **negative:** `unit/verify` [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_admission_se_test.go#L38) |
| `RFC3209-6-2` | On receipt of the Path message, the egress node sends a Resv message with the STYLE Shared Explicit toward the ingress node. (§4.6.4) | MUST | 4.6.4 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L80). **positive:** `unit/verify` [`TestRFC3209EgressResvStyleIsSharedExplicit`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L227). **negative:** no negative test. **{single-polarity}:** every RESV ze originates carries STYLE = Shared Explicit (18) and admission always applies SE sharing semantics; ze never emits a Fixed-Filter style for an LSP tunnel (internal/plugins/rsvpte/engine.go:272, build.go:126, wire.go:680) |
| `RFC3209-x-2` | If the requested bandwidth is not available a PathErr message is returned with an Error Code of 01, Admission Control Failure, and an Error Value of 0x0002. (§4.7.3) | SHOULD | 4.7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4-1` | The RRO can be present in both RSVP Path and Resv messages. (§4.4) | MAY | 4.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3209-2.6-1`](#rfc3209-2.6-1) an LSR MUST execute the following algorithm (§2.6) | {gap}, no test | labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer executes the labeled-datagram forwarding algorithm |
| [`RFC3209-2.6-2`](#rfc3209-2.6-2) (a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (S2.6) | {gap}, no test | labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer breaks an oversized fragmentable labeled datagram into fragments no larger than M |
| [`RFC3209-2.6-3`](#rfc3209-2.6-3) (b) each fragment MUST be labeled and then forwarded. (S2.6) | {gap}, no test | labeled forwarding at a transit LSR is Linux MPLS (net/mpls/af_mpls.c) on the label routes Ze installs, no Ze code fragments a labeled datagram, and no evidence shows the kernel path running the Section 2.6 algorithm; Ze programs path MTU (internal/plugins/rsvpte/mtu.go) but nothing shows that a layer labels and forwards the fragments of an oversized labeled datagram |
| [`RFC3209-2.6-4`](#rfc3209-2.6-4) When the size of an IPv4 datagram (without labels) exceeds the value of M, If the DF bit is not set in the IPv4 header, then (a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (b) each fragment MUST be labeled and then forwarded. If the DF bit is set in the IPv4 header, then (a) the datagram MUST NOT be forwarded (§2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux MPLS forwarding (net/mpls); internal/plugins/rsvpte/fib.go::programSwap installs the label operation and no MTU, so the no-forward decision reads nothing Ze writes |
| [`RFC3209-2.6-5`](#rfc3209-2.6-5) When the size of an IPv6 datagram (without labels) exceeds the value of M, (a) the datagram MUST NOT be forwarded (S2.6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux MPLS forwarding (net/mpls); internal/plugins/rsvpte/fib.go::programSwap installs the label operation and no MTU, so the IPv6 no-forward decision reads nothing Ze writes |
| [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1) If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2) Note that if a node intends to police individual senders to a session, it MUST assign unique labels to those senders. (S4.1.1.1) | {gap}, no test | Ze polices no sender; plan/spec-rsvpte-sender-policing.md |
| [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3) If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1) This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§4.2.2) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2) If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3) If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1) This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§4.2.3) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2) The DLCI MUST be right justified in this field and unused bits MUST be set to 0. (S4.2.3) | {gap}, no test | Ze encodes and decodes no ATM or Frame Relay label range; plan/spec-rsvpte-atm-frame-relay-labels.md |
| [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1) This means that if a router has a neighbor that is known to not be RSVP capable, the router MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers. (S4.2.5) | {gap}, no test | Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2) 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| \| + + \| IPv6 tunnel sender address \| + + \| (16 bytes) \| + + \| \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \| MUST be zero \| LSP ID \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ (S4.6.2) | {gap}, no test | Ze has no IPv6 SESSION or SENDER_TEMPLATE codec; plan/spec-rsvpte-ipv6-lsp-tunnel.md |
| [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1) In order to be validated a link MUST pass the three tests below. (S4.7.4) | {gap}, no test | Ze decodes no affinity mask and validates no link against one; plan/spec-rsvpte-resource-affinities.md |
| [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2) When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4) | {gap}, no test | Ze decodes no affinity mask and validates no link against one; plan/spec-rsvpte-resource-affinities.md |
| [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1) This value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node and otherwise remains the same. (S5.2.2) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2) This field MUST NOT be set to zero (0). (S5.2.2) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3) This field MUST be set to zero (0) when no value has ever been seen from the neighbor. (S5.2.2) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-1`](#rfc3209-5.3-1) This value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-2`](#rfc3209-5.3-2) On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-3`](#rfc3209-5.3-3) If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-4`](#rfc3209-5.3-4) If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, then the node MUST treat the neighbor as if communication has been lost. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-5`](#rfc3209-5.3-5) On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-6`](#rfc3209-5.3-6) The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-7`](#rfc3209-5.3-7) If the neighbor advertises a wrong value in the Dst_Instance field, then a node MUST treat the neighbor as if communication has been lost. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-8`](#rfc3209-5.3-8) If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, then a node MUST presume that it cannot communicate with the neighbor. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-9`](#rfc3209-5.3-9) If a node does re-initiate it MUST use a Src_Instance value different than the one advertised in the previous HELLO message. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-10`](#rfc3209-5.3-10) This new value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.3-11`](#rfc3209-5.3-11) If a new instance value has not been received from the neighbor, then the node MUST advertise zero in the Dst_instance value field. (S5.3) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.4-1`](#rfc3209-5.4-1) When the links between neighbors are numbered, then Hellos MUST be run on each link and the previously described mechanisms apply. (S5.4) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |
| [`RFC3209-5.4-2`](#rfc3209-5.4-2) When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4) | {gap}, no test | Ze speaks no RSVP Hello; plan/spec-rsvpte-hello-extension.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1)

| IPv4 tunnel end point address | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | MUST be zero | Tunnel ID | (§4.6.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 after the tagged units shifted byte-identical; re-read against rfc/full/rfc3209.txt and the current producers. Section 4.6.1.1 diagram: the field before Tunnel ID is "MUST be zero". TestRFC3209SessionReservedZeroOverDirtyBuffer encodes over a 0xFF-prefilled buffer and asserts buf[8:10] zero, so encodeSessionIPv4 skipping its buf[8]=0, buf[9]=0 goes red. TestRSVPSessionObjectEncoding encodes into a zeroed buffer and alone would not discriminate the write; the dirty-buffer test carries the verdict.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3209SessionReservedZeroOverDirtyBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L342) | unit/verify | revert, verified |

### [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1)

Class = SENDER_TEMPLATE, LSP_TUNNEL_IPv4 C-Type = 7 0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | IPv4 tunnel sender address | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | MUST be zero | LSP ID | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ (S4.6.2, Wire Format)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 after the tagged units shifted byte-identical; re-read against rfc/full/rfc3209.txt and the current producers. Section 4.6.2.1 diagram: the field before LSP ID is "MUST be zero". TestRSVPSenderTemplateReservedZeroOnSend prefills 0xAA, asserts buf[8],buf[9]==0 and the whole 12-octet object, and that nothing past it was written, so an encodeSenderTemplate that skips or corrupts the reserved write goes red. Single polarity: the decoder does not read the field.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L372) | unit/verify | unproven |

### [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2)

| IPv6 tunnel end point address | + + | (16 bytes) | + + | | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | MUST be zero | Tunnel ID | (§4.6.1.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Rejudged 2026-10-08 after the tagged units shifted byte-identical; re-read against rfc/full/rfc3209.txt and the current producers. The row is the LSP_TUNNEL_IPv6 SESSION (Section 4.6.1.2, C-Type 8) reserved field. TestRSVPSessionObjectEncoding encodes only the IPv4 SESSION via encodeSessionIPv4 (C-Type 7) and proves RFC3209-4.6.1-1. Ze has no IPv6 SESSION encoder, so the tag claims a row its test cannot observe.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L341) | unit/verify | unproven |

### [`RFC3209-4.2-1`](#rfc3209-4.2-1)

To establish an LSP tunnel the sender creates a Path message with a LABEL_REQUEST object. The LABEL_REQUEST object indicates that a label binding for this path is requested and provides an indication of the network layer protocol that is to be carried over this path. (§4.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 after the tagged units shifted byte-identical; re-read against rfc/full/rfc3209.txt and the current producers. Section 4.2.4: "To establish an LSP tunnel the sender creates a Path message with a LABEL_REQUEST object." TestRFC3209IngressPathCarriesLabelRequest reads the PATH the ingress engine sends and asserts LABEL_REQUEST with L3PID 0x0800; TestRFC3209IngressPathNeverWithoutLabelRequest asserts it on the originated PATH, two refreshes and the make-before-break PATH. TestBuildPathRoundTrip proves buildPath appends the object; TestRFC3209NoLabelWithoutLabelRequest proves the neighbouring receiver rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209NoLabelWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L222) | unit/verify | mutant, verified |
| negative | [`TestRFC3209IngressPathNeverWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC3209IngressPathCarriesLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L46) | unit/verify | revert, verified |

### [`RFC3209-4.1-1`](#rfc3209-4.1-1)

The label for a sender MUST immediately follow the FILTER_SPEC for that sender in the Resv message (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L463) | unit/verify | unproven |
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L75) | unit/verify | unproven |

### [`RFC3209-4.1-2`](#rfc3209-4.1-2)

Labels MAY be carried in Resv messages (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L301) | unit/verify | unproven |
| positive | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L300) | unit/verify | unproven |

### [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1)

The node determines whether it is topologically adjacent to the abstract node described by the second subobject. If so, the node selects a particular next hop which is a member of the abstract node. The node then deletes the first subobject and continues processing with section 4.3.4.2. (§4.3.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Closure rejudge 2026-10-02: TestEngineTransitForwarding changed only in its positive claim, which named nextHopFromERO (gone) and now names the current producer, routing.go resolveExplicitPath, which skips the leading local subobjects and, when the next node is in the second abstract node, returns the ERO from that subobject on; the body still asserts the PATH is relayed to the egress with a one-hop ERO beginning at the egress. Positive re-recorded on resolveExplicitPath (observed red). Negatives TestRFC3209TransitNotAdjacentToSecondSubobject (no route to the strict second subobject: no next hop, nothing forwarded, no state, PathErr 24/2) and TestEngineTransitNoUsableERONextHop, both recorded on resolveExplicitPath.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L509) | unit/verify | revert, verified |
| negative | [`TestRFC3209TransitNotAdjacentToSecondSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_test.go#L291) | unit/verify | revert, verified |

### [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1)

1) The node receiving the RSVP message MUST first evaluate the first subobject. (S4.3.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). The missing input is now covered: TestRFC3209TransitRefusesForeignFirstSubobject sends a first subobject 10.0.0.77/32 that does not contain this node; the transit forwards nothing, keeps no state, and answers PathErr 24/4 to the previous hop, which a blind pop would fail. Positive TestRFC3209TransitEvaluatesFirstEROSubobject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209TransitNoFirstEROSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L145) | unit/verify | mutant, verified |
| negative | [`TestRFC3209TransitRefusesForeignFirstSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC3209TransitEvaluatesFirstEROSubobject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L123) | unit/verify | revert, verified |

### [`RFC3209-2.6-1`](#rfc3209-2.6-1)

an LSR MUST execute the following algorithm (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-1, so no unit is bound to it.

### [`RFC3209-2.6-2`](#rfc3209-2.6-2)

(a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-2, so no unit is bound to it.

### [`RFC3209-2.6-3`](#rfc3209-2.6-3)

(b) each fragment MUST be labeled and then forwarded. (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-3, so no unit is bound to it.

### [`RFC3209-2.6-4`](#rfc3209-2.6-4)

When the size of an IPv4 datagram (without labels) exceeds the value of M, If the DF bit is not set in the IPv4 header, then (a) the datagram MUST be broken into fragments, each of whose size is no greater than M, and (b) each fragment MUST be labeled and then forwarded. If the DF bit is set in the IPv4 header, then (a) the datagram MUST NOT be forwarded (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-4, so no unit is bound to it.

### [`RFC3209-2.6-5`](#rfc3209-2.6-5)

When the size of an IPv6 datagram (without labels) exceeds the value of M, (a) the datagram MUST NOT be forwarded (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-5, so no unit is bound to it.

### [`RFC3209-3-1`](#rfc3209-3-1)

In Resv messages they MUST appear after the associated FILTER_SPEC and prior to any subsequent FILTER_SPEC. (S3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: LABEL or RECORD_ROUTE placed before its FILTER_SPEC or after a subsequent one. TestRFC3209ResvLabelFollowsFilterSpec asserts objs[filter+1]==LABEL and objs[filter+2]==RECORD_ROUTE; TestRFC3209LabelFollowsFilterSpec asserts exactly one FILTER_SPEC and label==filter+1; TestRFC3209ResvNeverCarriesSenderTemplate asserts no LABEL/RRO precedes the FILTER_SPEC. buildResv emits one sender per RESV, so both clauses are pinned.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209ResvNeverCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC3209ResvLabelFollowsFilterSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC3209LabelFollowsFilterSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L254) | unit/verify | revert, verified |

### [`RFC3209-3-2`](#rfc3209-3-2)

The ordering of these objects is not important, so an implementation MUST be prepared to accept objects in any order (S3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L187) | unit/verify | mutant, verified |

### [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1)

If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-1, so no unit is bound to it.

### [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2)

Note that if a node intends to police individual senders to a session, it MUST assign unique labels to those senders. (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-2, so no unit is bound to it.

### [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3)

If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-3, so no unit is bound to it.

### [`RFC3209-4.2.1-1`](#rfc3209-4.2.1-1)

This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 (zero on transmission): TestRFC3209LabelRequestReservedZeroOnSend prefills 0xFF and asserts buf[4:6]=={0,0}. Clause 2 (ignored on receipt): TestRFC3209LabelRequestReservedIgnoredOnReceipt decodes reserved 0xFFFF and asserts NoError and L3PID 0x0800, red if the decoder refuses or misreads it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209LabelRequestReservedIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC3209LabelRequestReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L49) | unit/verify | revert, verified |

### [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1)

This field is reserved. It MUST be set to zero on transmission and MUST be ignored on receipt. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-1, so no unit is bound to it.

### [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2)

If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-2, so no unit is bound to it.

### [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3)

If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero. (S4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-3, so no unit is bound to it.

### [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1)

This field is reserved. It MUST be set to zero on transmission and ignored on receipt. (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.3-1, so no unit is bound to it.

### [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2)

The DLCI MUST be right justified in this field and unused bits MUST be set to 0. (S4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.3-2, so no unit is bound to it.

### [`RFC3209-4.2.4-1`](#rfc3209-4.2.4-1)

A receiver that accepts a LABEL_REQUEST object MUST include a LABEL object in Resv messages pertaining to that Path message (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3209EgressResvCarriesLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L30) | unit/verify | revert, verified |

### [`RFC3209-4.2.4-2`](#rfc3209-4.2.4-2)

If a LABEL_REQUEST object was not present in the Path message, a node MUST NOT include a LABEL object in a Resv message for that Path message's session and PHOP (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209NoLabelWithoutLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L221) | unit/verify | mutant, verified |
| positive | [`TestRFC3209LabelFollowsLabelRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L205) | unit/verify | mutant, verified |

### [`RFC3209-4.2.4-3`](#rfc3209-4.2.4-3)

A node that sends a LABEL_REQUEST object MUST be ready to accept and correctly process a LABEL object in the corresponding Resv messages (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209IngressRefusesResvWithoutLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L74) | unit/verify | mutant, verified |
| positive | [`TestRFC3209IngressProcessesResvLabel`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L48) | unit/verify | mutant, verified |

### [`RFC3209-4.2.4-4`](#rfc3209-4.2.4-4)

A node which receives and forwards a Path message each with a LABEL_REQUEST object, MUST copy the L3PID from the received LABEL_REQUEST object to the forwarded LABEL_REQUEST object. (S4.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a relayed PATH whose LABEL_REQUEST L3PID is not the received one. TestRFC3209TransitCopiesL3PID sends L3PID 0x86DD and asserts fwd.LabelRequest.L3PID==0x86DD, red if the transit substitutes its local default 0x0800 (register.go, reroute.go). Single polarity is declared on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3209TransitCopiesL3PID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_engine_label_test.go#L106) | unit/verify | revert, verified |

### [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1)

This means that if a router has a neighbor that is known to not be RSVP capable, the router MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers. (S4.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.5-1, so no unit is bound to it.

### [`RFC3209-4.3.3-1`](#rfc3209-4.3.3-1)

The Length MUST be at least 4, and MUST be a multiple of 4. (S4.3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Send: TestRFC3209EROSubobjectLengthOnSend asserts lengths {8,20,8}, each >=4 and %4==0, red on a short or non-multiple Length. Receive: TestRFC3209EROSubobjectLengthShortRefused asserts decodeERO refuses Length 3 with errShortERO.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209EROSubobjectLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC3209EROSubobjectLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L70) | unit/verify | revert, verified |

### [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1)

The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node (S4.3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209StrictHopThroughOutsideNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRFC3209StrictHopReachedDirectly`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L60) | unit/verify | revert, verified |

### [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1)

Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node. (S4.3.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209InteriorHopLeavingAbstractNodeRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestRFC3209InteriorHopStaysInAbstractNode`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_explicit_hops_test.go#L118) | unit/verify | revert, verified |

### [`RFC3209-4.4.1-1`](#rfc3209-4.4.1-1)

The length MUST always be a multiple of 4, and at least 4. (S4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Send: TestRFC3209RROSubobjectLengthOnSend asserts lengths {8,8,20}, each >=4 and %4==0. Receive: TestRFC3209RROSubobjectLengthShortRefused asserts decodeRRO refuses Length 3 with errShortRRO.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209RROSubobjectLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestRFC3209RROSubobjectLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L99) | unit/verify | revert, verified |

### [`RFC3209-4.4.3-1`](#rfc3209-4.4.3-1)

The newly added subobject MUST be this router's IP address. (S4.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). Now proven at the engine, not the helper: TestRFC3209EngineRecordsOwnAddress drives a transit through handlePacket and reads the RRO head of the relayed PATH and the relayed RESV as the router's own 10.0.0.5, rest in order; TestRFC3209EngineNeverRecordsNeighborAddress asserts the head is neither the next hop nor the previous hop.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209RRONoSubobjectWithoutOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestRFC3209EngineNeverRecordsNeighborAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRFC3209RRONewSubobjectIsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC3209EngineRecordsOwnAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L117) | unit/verify | revert, verified |

### [`RFC3209-4.4.3-2`](#rfc3209-4.4.3-2)

A node MUST NOT push on a Label Record subobject without also pushing on an IPv4 or IPv6 subobject (S4.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209RRONoLabelRecordWithoutAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L43) | unit/verify | mutant, verified |
| positive | [`TestRFC3209RROLabelRecordFollowsAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_rro_test.go#L32) | unit/verify | mutant, verified |

### [`RFC3209-4.4.3-3`](#rfc3209-4.4.3-3)

If the newly added subobject causes the RRO to be too big to fit in a Path (or Resv) message, the RRO object SHALL be dropped from the message and message processing continues as normal. (S4.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). Path direction now covered: TestRFC3209PathRRODroppedWhenTooBig (32 subobjects in, PATH forwarded without RRO, path state installed) and TestRFC3209PathRROKeptWhenItFits (31 in, 32 out headed by this router); the Resv direction was already covered. 'Too big' is realised as maxRecordRouteHops (32), the bound that keeps the RRO inside the fixed encode buffer (wire.go); that threshold is a model choice, not the message size.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209RROKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L63) | unit/verify | revert, verified |
| negative | [`TestRFC3209PathRROKeptWhenItFits`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestRFC3209RRODroppedWhenTooBig`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC3209PathRRODroppedWhenTooBig`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L171) | unit/verify | revert, verified |

### [`RFC3209-4.4.3-4`](#rfc3209-4.4.3-4)

A received Path message without an RRO indicates that the sender node no longer needs route recording.  Subsequent Resv messages SHALL NOT contain an RRO. (S4.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Row widened to the verbatim trigger plus ban; the tests already drive the trigger. TestRFC3209PathRROWithdrawal: after a Path without an RRO, the next relayed RESV and a sendResv refresh carry no RRO, at transit and egress. Negative TestRFC3209PathRROForwarded keeps the RRO while the PATH requests it, so an always-drop implementation goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209PathRROForwarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L330) | unit/verify | unproven |
| positive | [`TestRFC3209PathRROWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L285) | unit/verify | mutant, verified |

### [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2)

0 1 2 3 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | | + + | IPv6 tunnel sender address | + + | (16 bytes) | + + | | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ | MUST be zero | LSP ID | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ (S4.6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.6.2-2, so no unit is bound to it.

### [`RFC3209-4.7.3-1`](#rfc3209-4.7.3-1)

The Length MUST always be a multiple of 4 and MUST be at least 8. (S4.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Send: TestRFC3209SessionAttributeLengthOnSend asserts header Length >=8 and %4==0 for names of 0 to 5 octets. Receive: TestRFC3209SessionAttributeLengthShortRefused asserts decodeSessionAttr refuses a body under the fixed four octets (Length below 8) for both C-Types.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209SessionAttributeLengthShortRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestRFC3209SessionAttributeLengthOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L127) | unit/verify | revert, verified |

### [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1)

In order to be validated a link MUST pass the three tests below. (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.4-1, so no unit is bound to it.

### [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2)

When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.4-2, so no unit is bound to it.

### [`RFC3209-4.7.4-3`](#rfc3209-4.7.4-3)

All RSVP routers, whether they support the SESSION_ATTRIBUTE object or not, SHALL forward the object unmodified (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3209SessionAttributeNotInserted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC3209SessionAttributeRelayedUnmodified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_relay_test.go#L165) | unit/verify | revert, verified |

### [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1)

This value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node and otherwise remains the same. (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-1, so no unit is bound to it.

### [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2)

This field MUST NOT be set to zero (0). (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-2, so no unit is bound to it.

### [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3)

This field MUST be set to zero (0) when no value has ever been seen from the neighbor. (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-3, so no unit is bound to it.

### [`RFC3209-5.3-1`](#rfc3209-5.3-1)

This value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-1, so no unit is bound to it.

### [`RFC3209-5.3-2`](#rfc3209-5.3-2)

On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-2, so no unit is bound to it.

### [`RFC3209-5.3-3`](#rfc3209-5.3-3)

If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-3, so no unit is bound to it.

### [`RFC3209-5.3-4`](#rfc3209-5.3-4)

If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, then the node MUST treat the neighbor as if communication has been lost. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-4, so no unit is bound to it.

### [`RFC3209-5.3-5`](#rfc3209-5.3-5)

On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-5, so no unit is bound to it.

### [`RFC3209-5.3-6`](#rfc3209-5.3-6)

The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-6, so no unit is bound to it.

### [`RFC3209-5.3-7`](#rfc3209-5.3-7)

If the neighbor advertises a wrong value in the Dst_Instance field, then a node MUST treat the neighbor as if communication has been lost. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-7, so no unit is bound to it.

### [`RFC3209-5.3-8`](#rfc3209-5.3-8)

If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, then a node MUST presume that it cannot communicate with the neighbor. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-8, so no unit is bound to it.

### [`RFC3209-5.3-9`](#rfc3209-5.3-9)

If a node does re-initiate it MUST use a Src_Instance value different than the one advertised in the previous HELLO message. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-9, so no unit is bound to it.

### [`RFC3209-5.3-10`](#rfc3209-5.3-10)

This new value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-10, so no unit is bound to it.

### [`RFC3209-5.3-11`](#rfc3209-5.3-11)

If a new instance value has not been received from the neighbor, then the node MUST advertise zero in the Dst_instance value field. (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-11, so no unit is bound to it.

### [`RFC3209-5.4-1`](#rfc3209-5.4-1)

When the links between neighbors are numbered, then Hellos MUST be run on each link and the previously described mechanisms apply. (S5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.4-1, so no unit is bound to it.

### [`RFC3209-5.4-2`](#rfc3209-5.4-2)

When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.4-2, so no unit is bound to it.

### [`RFC3209-6-1`](#rfc3209-6-1)

When an ingress node with an established path wants to change that path, it forms a new Path message as follows. The existing SESSION object is used. In particular the Tunnel_ID and Extended_Tunnel_ID are unchanged. The ingress node picks a new LSP_ID to form a new SENDER_TEMPLATE. (§4.6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). TestRFC3209ReroutePathKeepsSessionNewLSPID reads the emitted make-before-break PATH: SESSION equal to the original and to the PSB, same sender, new LSP_ID. TestRFC3209RerouteNeverReusesLSPID: two successive reroutes give three distinct LSP_IDs under one SESSION.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_admission_se_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC3209RerouteNeverReusesLSPID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_reroute_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestRFC3209ReroutePathKeepsSessionNewLSPID`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L195) | unit/verify | revert, verified |

### [`RFC3209-6-2`](#rfc3209-6-2)

On receipt of the Path message, the egress node sends a Resv message with the STYLE Shared Explicit toward the ingress node. (§4.6.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 after the tagged units shifted byte-identical; re-read against rfc/full/rfc3209.txt and the current producers. Section 4.6.4: "On receipt of the Path message, the egress node sends a Resv message with the STYLE Shared Explicit toward the ingress node." TestRFC3209EgressResvStyleIsSharedExplicit drives a PATH into an egress engine through handlePacket and asserts the STYLE of the RESV it sends is 0x12. TestBuildResvRoundTrip proves the builder encodes SE when asked.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_build_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC3209EgressResvStyleIsSharedExplicit`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc3209_signaling_test.go#L227) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3209.txt |
| Source fingerprint | 976eb5437f567b6f |
| Record | rfc/extraction/rfc3209.json |
| Mapped sentences | 55 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.4.1` | not stated | 0 | walked | not stated |
| `2.4.2` | not stated | 0 | walked | not stated |
| `2.4.3` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 5 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.1` | not stated | 3 | walked | not stated |
| `4.1.1.2` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 5 | walked | not stated |
| `4.2.3` | not stated | 3 | walked | not stated |
| `4.2.4` | not stated | 5 | walked | not stated |
| `4.2.5` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 0 | walked | not stated |
| `4.3.2` | not stated | 0 | walked | not stated |
| `4.3.3` | not stated | 1 | walked | not stated |
| `4.3.3.1` | not stated | 1 | walked | not stated |
| `4.3.3.2` | not stated | 0 | walked | not stated |
| `4.3.3.3` | not stated | 0 | walked | not stated |
| `4.3.3.4` | not stated | 0 | walked | not stated |
| `4.3.4` | not stated | 0 | walked | not stated |
| `4.3.4.1` | not stated | 1 | walked | not stated |
| `4.3.4.2` | not stated | 1 | walked | not stated |
| `4.3.5` | not stated | 0 | walked | not stated |
| `4.3.6` | not stated | 0 | walked | not stated |
| `4.3.7` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 1 | walked | not stated |
| `4.4.1.1` | not stated | 0 | walked | not stated |
| `4.4.1.2` | not stated | 0 | walked | not stated |
| `4.4.1.3` | not stated | 0 | walked | not stated |
| `4.4.2` | not stated | 0 | walked | not stated |
| `4.4.3` | not stated | 4 | walked | not stated |
| `4.4.4` | not stated | 0 | walked | not stated |
| `4.4.5` | not stated | 0 | walked | not stated |
| `4.4.6` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.6.1` | not stated | 0 | walked | not stated |
| `4.6.1.1` | not stated | 1 | walked | not stated |
| `4.6.1.2` | not stated | 1 | walked | not stated |
| `4.6.2` | not stated | 0 | walked | not stated |
| `4.6.2.1` | not stated | 1 | walked | not stated |
| `4.6.2.2` | not stated | 1 | walked | not stated |
| `4.6.3` | not stated | 0 | walked | not stated |
| `4.6.3.1` | not stated | 0 | walked | not stated |
| `4.6.3.2` | not stated | 0 | walked | not stated |
| `4.6.4` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 0 | walked | not stated |
| `4.7.1` | not stated | 0 | walked | not stated |
| `4.7.2` | not stated | 0 | walked | not stated |
| `4.7.3` | not stated | 1 | walked | not stated |
| `4.7.4` | not stated | 5 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 13 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum VPI field repeats the right-justification rule the Minimum VPI field states; site 4.2.2:2 maps it | If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero. |
| `4.2.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum VCI field repeats the right-justification rule the Minimum VCI field states; site 4.2.2:3 maps it | If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero. |
| `4.2.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum DLCI field repeats the right-justification rule the Minimum DLCI field states; site 4.2.3:2 maps it | The DLCI MUST be right justified in this field and unused bits MUST be set to 0. |
| `4.2.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.2.4 restates the section 4.1.1.1 rule that a label is allocated from a specified label range; site 4.1.1.1:1 maps it | If a label range was specified, the label MUST be allocated from that range. |
| `4.7.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the second statement of the same three-test validation rule, for the second of the two resource-affinity cases; site 4.7.4:1 maps it | In order to be validated a link MUST pass the following three tests. |
| `4.7.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | 'For a link to be acceptable, all three tests MUST pass' restates the validation rule the preceding sentence states; site 4.7.4:1 maps it | For a link to be acceptable, all three tests MUST pass. |
| `5.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a scoping statement about how the RFC 2119 keywords in section 5.3 are to be read ('the use of MUST and SHOULD with respect to the receiver applies only to a node that supports Hello message processing'), not an obligation of its own | In particular, the use of MUST and SHOULD with respect to the receiver applies only to a node that supports Hello message processing. |
| `5.3:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the HELLO ACK path repeats the HELLO REQUEST sentence word for word; site 5.3:4 maps it | If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost. |

## Superseded

No document obsoletes RFC 3209, so its obligations are stated where they were written.
