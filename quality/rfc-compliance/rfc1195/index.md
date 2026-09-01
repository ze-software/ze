# RFC 1195 - Use of OSI IS-IS for Routing in TCP/IP and Dual Environments

Experimental. Every requirement this repository extracted from RFC 1195, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 1 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 83.3% | 5 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 10 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1195.md` |
| Requirement shard | `rfc/requirements/rfc1195.md` |
| RFC text | `rfc/full/rfc1195.txt` |

## Enrolment

Enrolled: Use of OSI IS-IS for Routing in TCP/IP and Dual Environments (Integrated IS-IS): ten MUST-level requirements. Six are met in internal/plugins/isis: 5.2-1 (advertise the Protocols Supported TLV 129 with NLPID 0xCC for IPv4), 5.2-2 (include the IP Interface Address TLV 132), 5.2-3 (advertise the same IP interface addresses at Level 1 and Level 2), and 5.2-4 (every IP reachability entry carries a metric) are {single-polarity: positive}; 3.9-1 (discard a PDU that fails authentication) carries positive+negative tags; 3.1-1 (flood LSPs with unrecognized TLVs preserved verbatim) is {single-polarity: positive}. Four are {not-applicable}: 5.3.4-1 (the I/E bit in the narrow TLV 128), 5.3.4-2 (narrow TLVs 128/130/131 in a pseudonode LSP), and 3.2-1 (cap the narrow 6-bit metric at 63) -- ze is wide-metric-only and encodes only the RFC 5305 extended TLV 135, never the narrow TLVs; and 1.4-1 (the dual OSI/IP topology constraint) -- ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Native IS-IS over Layer 2, L1/L2, broadcast and point-to-point circuits, TLV 132.

**What the ledger says remains:**

Feature remains Experimental pending production hardening and deployment evidence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC1195-3.9-1`](#rfc1195-3.9-1)

**Annotated instead of tested (9):** [`RFC1195-5.2-1`](#rfc1195-5.2-1), [`RFC1195-5.2-2`](#rfc1195-5.2-2), [`RFC1195-5.2-3`](#rfc1195-5.2-3), [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1), [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2), [`RFC1195-5.2-4`](#rfc1195-5.2-4), [`RFC1195-3.2-1`](#rfc1195-3.2-1), [`RFC1195-3.1-1`](#rfc1195-3.1-1), [`RFC1195-1.4-1`](#rfc1195-1.4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1195-5.2-1` | Include Protocols Supported (129) in all Hellos, all LSP number 0, and point-to-point ISHs (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L122). **negative:** no negative test. **{single-polarity}:** ze unconditionally emits the Protocols Supported TLV 129 with NLPID 0xCC for every IP-capable circuit IIH (internal/plugins/isis/circuit/hello.go:97-104) and LSP fragment 0 (internal/plugins/isis/lsdb/origination.go:407-409); there is no code path that omits TLV 129 or emits a non-0xCC IPv4 NLPID, so there is no negative form to reject |
| `RFC1195-5.2-2` | Include IP Interface Address (132) in every IP-capable router's LSPs (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L110). **positive:** `unit/verify` [`TestISISTLVIPv4InterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L19). **negative:** no negative test. **{single-polarity}:** ze packs the IP Interface Address TLV 132 into LSP fragment 0 from the node's own interface addresses (internal/plugins/isis/lsdb/origination.go:433-438) and the codec reads it back verbatim (internal/plugins/isis/packet/tlv_ipv4.go DecodeIPv4InterfaceAddrTLV); this is an emit obligation with no decode-side reject-on-absence path, so there is no negative form to drive |
| `RFC1195-5.2-3` | Advertise the same IP address(es) at Level 1 and Level 2 when the router is both (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISEngineOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L81). **negative:** no negative test. **{single-polarity}:** ze collects interface addresses level-independently (internal/plugins/isis/lsdb_wiring.go:436-448) so the same set is advertised at both levels by construction; there is no per-level divergence code path to drive a negative |
| `RFC1195-5.3.4-1` | Set the I/E bit to 0 in IP Internal Reachability (128) entries (Section 5.3.4) | MUST | 5.3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is wide-metric-only and its IS-IS codec never encodes the narrow IP Reachability TLV 128 (internal/plugins/isis/packet/tlv.go recognizes no type 128; internal/plugins/isis/types/metric.go uses 24-bit and 32-bit metrics, not the 6-bit narrow field). It advertises IP reachability via the RFC 5305 extended TLV 135 (already enrolled), which has no I/E bit, so there is no narrow-TLV I/E-bit code path |
| `RFC1195-5.3.4-2` | Place codes 128, 130, 131 in pseudonode LSPs (Section 5.3.4, Section 5.3.5) | MUST NOT | 5.3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never emits the narrow IP reachability TLVs 128/130/131 (wide-metric-only), and its pseudonode LSP builder emits only TLV 22 (internal/plugins/isis/lsdb/pseudonode.go:186-193), so the literal-code prohibition has no applicable code path |
| `RFC1195-5.2-4` | Carry a default metric in every reachability entry (Section 5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L66). **negative:** no negative test. **{single-polarity}:** TLV 135 WriteTo unconditionally writes the 4-octet prefix metric for every entry (internal/plugins/isis/packet/tlv_ipv4.go ExtendedIPReachTLV.WriteTo) and the metric is a mandatory struct field (internal/plugins/isis/types/metric.go PrefixMetric), so a metric-less entry is unrepresentable and there is no negative form to reject |
| `RFC1195-3.2-1` | Cap a Level 2 metric sum at 63 (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze uses RFC 5305 wide metrics (24-bit IS reachability and 32-bit IP reachability, internal/plugins/isis/types/metric.go) and has no 6-bit narrow metric field to cap at 63 |
| `RFC1195-3.9-1` | Discard a packet with invalid authentication information (Section 3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611) |
| `RFC1195-3.1-1` | Ignore unrecognised codes and pass them unchanged in forwarded LSPs (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16). **negative:** no negative test. **{single-polarity}:** the codec retains every TLV type it does not recognize as an opaque span and re-encodes it byte-for-byte (internal/plugins/isis/packet/tlv.go:66-89 iterator plus DecodeTLVs / writeTLVs) so the LSDB re-floods unknown TLVs verbatim (internal/plugins/isis/lsdb/flooding.go:342-360); preserving unrecognized codes IS the requirement and there is no reject path for an unknown TLV, so no negative form exists |
| `RFC1195-1.4-1` | Use only dual routers in a dual area, and dual Level 2 routers when routing both IP and OSI between areas (Section 1.4) | MUST | 1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |
| `RFC1195-5.3.5-1` | Set the I/E bit to 0 or 1 in IP External Reachability (130) entries (Section 5.3.5) | MAY | 5.3.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1) Set the I/E bit to 0 in IP Internal Reachability (128) entries (Section 5.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze is wide-metric-only and its IS-IS codec never encodes the narrow IP Reachability TLV 128 (internal/plugins/isis/packet/tlv.go recognizes no type 128; internal/plugins/isis/types/metric.go uses 24-bit and 32-bit metrics, not the 6-bit narrow field). It advertises IP reachability via the RFC 5305 extended TLV 135 (already enrolled), which has no I/E bit, so there is no narrow-TLV I/E-bit code path |
| [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2) Place codes 128, 130, 131 in pseudonode LSPs (Section 5.3.4, Section 5.3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze never emits the narrow IP reachability TLVs 128/130/131 (wide-metric-only), and its pseudonode LSP builder emits only TLV 22 (internal/plugins/isis/lsdb/pseudonode.go:186-193), so the literal-code prohibition has no applicable code path |
| [`RFC1195-3.2-1`](#rfc1195-3.2-1) Cap a Level 2 metric sum at 63 (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze uses RFC 5305 wide metrics (24-bit IS reachability and 32-bit IP reachability, internal/plugins/isis/types/metric.go) and has no 6-bit narrow metric field to cap at 63 |
| [`RFC1195-1.4-1`](#rfc1195-1.4-1) Use only dual routers in a dual area, and dual Level 2 routers when routing both IP and OSI between areas (Section 1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1195-5.2-1`](#rfc1195-5.2-1)

Include Protocols Supported (129) in all Hellos, all LSP number 0, and point-to-point ISHs (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L122) | unit/verify | unproven |

### [`RFC1195-5.2-2`](#rfc1195-5.2-2)

Include IP Interface Address (132) in every IP-capable router's LSPs (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L110) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4InterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L19) | unit/verify | unproven |

### [`RFC1195-5.2-3`](#rfc1195-5.2-3)

Advertise the same IP address(es) at Level 1 and Level 2 when the router is both (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISEngineOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L81) | unit/verify | unproven |

### [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1)

Set the I/E bit to 0 in IP Internal Reachability (128) entries (Section 5.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-5.3.4-1, so no unit is bound to it.

### [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2)

Place codes 128, 130, 131 in pseudonode LSPs (Section 5.3.4, Section 5.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-5.3.4-2, so no unit is bound to it.

### [`RFC1195-5.2-4`](#rfc1195-5.2-4)

Carry a default metric in every reachability entry (Section 5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L66) | unit/verify | unproven |

### [`RFC1195-3.2-1`](#rfc1195-3.2-1)

Cap a Level 2 metric sum at 63 (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-3.2-1, so no unit is bound to it.

### [`RFC1195-3.9-1`](#rfc1195-3.9-1)

Discard a packet with invalid authentication information (Section 3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134) | unit/verify | unproven |

### [`RFC1195-3.1-1`](#rfc1195-3.1-1)

Ignore unrecognised codes and pass them unchanged in forwarded LSPs (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16) | unit/verify | unproven |

### [`RFC1195-1.4-1`](#rfc1195-1.4-1)

Use only dual routers in a dual area, and dual Level 2 routers when routing both IP and OSI between areas (Section 1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-1.4-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 1195, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 1195, so its obligations are stated where they were written.
