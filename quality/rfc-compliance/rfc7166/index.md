# RFC 7166 - Supporting Authentication Trailer for OSPFv3

Unsupported. Every requirement this repository extracted from RFC 7166, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 17 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 17 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 33 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 17 of 17 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 17 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 33 |
| Gated MUST-level | 17 |
| Obligations that bind Ze | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 17 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7166.md` |
| Requirement shard | `rfc/requirements/rfc7166.md` |
| RFC text | `rfc/full/rfc7166.txt` |

## Enrolment

Enrolled: Supporting Authentication Trailer for OSPFv3 (RFC 7166): all 17 MUST/MUST NOT are gap -- the OSPFv3 Authentication Trailer is unimplemented (no AT-bit in v3 Options, no SA-ID/64-bit-sequence/HMAC trailer, no IPv6-source-bound digest, no receive-side verification). OSPFv3 ships only transmit checksum-omission hooks; cryptographic auth is the separate OSPFv2 path (RFC 2328/5709/7474). rfc-status row Unsupported; each gap cites its absence producer

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered**

- None: the OSPFv3 Authentication Trailer wire format is not implemented. OSPFv3 carries only the transmit-side checksum-omission hooks ([`internal/plugins/ospf/v3/packet/checksum.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/checksum.go), header.go)
- an interface `authentication key-chain` drives the separate OSPFv2 cryptographic path (RFC 2328/5709/7474) through [`internal/plugins/ospf/packet/auth_verify.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify.go), not an OSPFv3 trailer.


**What the ledger says remains**

The OSPFv3 Authentication Trailer is absent: no AT-bit in the OSPFv3 Options, no 16-byte trailer (Auth Type / Auth Data Len / SA ID / 64-bit sequence / HMAC), no Security Association, no IPv6-source-bound digest, and no receive-side trailer verification or per-packet-type replay for OSPFv3. All 17 MUST-level requirements are annotated `{gap}` in [`rfc/short/rfc7166.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7166.md).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 17 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (17):** [`RFC7166-2.1-1`](#rfc7166-2.1-1), [`RFC7166-3-3`](#rfc7166-3-3), [`RFC7166-3-6`](#rfc7166-3-6), [`RFC7166-4.1-2`](#rfc7166-4.1-2), [`RFC7166-4.1-3`](#rfc7166-4.1-3), [`RFC7166-4.1-4`](#rfc7166-4.1-4), [`RFC7166-4.1-5`](#rfc7166-4.1-5), [`RFC7166-4.1.1-1`](#rfc7166-4.1.1-1), [`RFC7166-4.2-3`](#rfc7166-4.2-3), [`RFC7166-4.2-4`](#rfc7166-4.2-4), [`RFC7166-4.3-1`](#rfc7166-4.3-1), [`RFC7166-4.3-4`](#rfc7166-4.3-4), [`RFC7166-4.4-1`](#rfc7166-4.4-1), [`RFC7166-4.6-1`](#rfc7166-4.6-1), [`RFC7166-4.6-3`](#rfc7166-4.6-3), [`RFC7166-4.6-5`](#rfc7166-4.6-5), [`RFC7166-4.6-6`](#rfc7166-4.6-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7166-2.1-1` | OSPFv3 routers set the AT-bit in all OSPFv3 Hello and Database Description packets that contain an Authentication Trailer (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no AT-bit exists in the OSPFv3 Options bitset -- internal/plugins/ospf/v3/types/options.go:14-23 defines only V6/E/N/R/AF, and the Hello/DD encoders internal/plugins/ospf/v3/packet/hello.go:88 and dbdesc.go:73 never set one, so no Authentication Trailer is emitted |
| `RFC7166-3-3` | When a new key replaces an old key, the new key's KeyStartGenerate be less than or equal to the old key's KeyStopGenerate (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no OSPFv3 Security Association carrying the RFC 7166 KeyStartAccept/KeyStartGenerate/KeyStopGenerate/KeyStopAccept lifetimes -- the only key model is the OSPFv2 send-lifetime pair in internal/plugins/ospf/auth_keystore.go:29-32, and validateKeyRollover internal/plugins/ospf/config.go:1007 orders OSPFv2 send-lifetimes only |
| `RFC7166-3-6` | Transmit an OSPFv3 packet unauthenticated when the last key associated with an interface has expired (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no OSPFv3 Authentication Trailer is ever produced -- internal/plugins/ospf/auth_wiring.go:28 signPacket appends the OSPFv2 trailer via internal/plugins/ospf/packet/auth_verify.go:157, so there is no OSPFv3 authenticated-transmit path whose key expiry this could govern; the never-revert selection in auth_keystore.go:281-286 is OSPFv2-only |
| `RFC7166-4.1-2` | Ignore the Reserved field when receiving protocol packets (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no OSPFv3 trailer decoder exists -- the only receive-side auth verify internal/plugins/ospf/auth_wiring.go:56 verifyPacket delegates to internal/plugins/ospf/auth_keystore.go:330 which decodes an OSPFv2 common header, so an OSPFv3 trailer Reserved field is never parsed |
| `RFC7166-4.1-3` | Increment the 64-bit cryptographic sequence number for every OSPFv3 packet sent (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 64-bit ESN counter in internal/plugins/ospf/auth_keystore.go:304-314 feeds the OSPFv2 packet.Sign trailer, not an OSPFv3 trailer; no OSPFv3 Cryptographic Sequence Number field is ever written |
| `RFC7166-4.1-4` | On reception, require the sequence number to be greater than that of the last accepted OSPFv3 packet of the same packet type from the sending neighbor (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the per-packet-type strictly-greater replay check internal/plugins/ospf/auth_keystore.go:352-361 runs only inside the OSPFv2 verify path; there is no OSPFv3 trailer receive path to which it applies |
| `RFC7166-4.1-5` | Use available mechanisms to preserve the sequence number's strictly increasing property for the router's deployed life, including cold restarts (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the persisted boot-count mechanism internal/plugins/ospf/auth_keystore.go:115 loadOSPFBootCount seeds the OSPFv2 ESN sequence only; no OSPFv3 trailer sequence is emitted for it to protect |
| `RFC7166-4.1.1-1` | Reset all keys before the 64-bit sequence number can wrap, to avoid replay attacks (§4.1.1) | MUST | 4.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no OSPFv3 trailer or its 64-bit sequence exists to wrap -- internal/plugins/ospf/auth_keystore.go:311-314 only bumps bootCount on the OSPFv2 ESN low-word wrap and performs no key reset |
| `RFC7166-4.2-3` | Omit OSPFv3 header checksum verification for received packets that include an Authentication Trailer (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the receive path never detects an OSPFv3 Authentication Trailer, so it cannot conditionally omit header-checksum verification; only the transmit-side checksum-omission hook exists at internal/plugins/ospf/v3/packet/checksum.go:165-167 and internal/plugins/ospf/v3/packet/header.go:277-280 |
| `RFC7166-4.2-4` | Omit LLS data block checksum verification for received packets that include an Authentication Trailer (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPFv3 codec implements no LLS data block -- there is no LLS type under internal/plugins/ospf/v3/packet/ -- and no trailer receive path, so there is no LLS checksum to conditionally omit |
| `RFC7166-4.3-1` | Include support for at least HMAC-SHA-256 (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the HMAC-SHA-256 primitive exists only for the OSPFv2 trailer at internal/plugins/ospf/packet/auth_verify.go:69 and :120; no OSPFv3 Authentication Trailer wires any algorithm |
| `RFC7166-4.3-4` | Use HMAC-SHA-256 as the default authentication algorithm (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no OSPFv3 trailer means no OSPFv3 default algorithm; the OSPFv2 keychain algorithm is operator-selected per key at internal/plugins/ospf/auth_keystore.go:239-252 with no HMAC-SHA-256 default |
| `RFC7166-4.4-1` | Append the two-octet OSPFv3 Cryptographic Protocol ID to the Authentication Key prior to use; other protocols sharing common keys similarly append their own IDs (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** only the OSPFv2 Cryptographic Protocol ID 0x0001 is appended, and only for the OSPFv2 AuType-3 trailer at internal/plugins/ospf/packet/auth_verify.go:38 and :195; there is no OSPFv3 Cryptographic Protocol ID constant or OSPFv3 trailer to apply it to |
| `RFC7166-4.6-1` | Minimally support examining the L-bit in the OSPFv3 Options and using the LLS data block length to access the Authentication Trailer (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPFv3 codec parses no LLS block, defines no L-bit in internal/plugins/ospf/v3/types/options.go:14-23, and defines no trailer, so it cannot locate an Authentication Trailer past an LLS block |
| `RFC7166-4.6-3` | Drop a packet whose cryptographic sequence number is less than or equal to the last accepted value for that neighbor and OSPFv3 packet type (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the seq<=last drop at internal/plugins/ospf/auth_keystore.go:357-359 is reached only via the OSPFv2 verify path; no OSPFv3 trailer receive path exists |
| `RFC7166-4.6-5` | Discard the packet when the computed digest does not match the received Authentication Data (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the constant-time digest compare at internal/plugins/ospf/packet/auth_verify.go:242 and :263 verifies the OSPFv2 trailer only; there is no OSPFv3 trailer digest to verify |
| `RFC7166-4.6-6` | After successful authentication, store the 64-bit cryptographic sequence number for each OSPFv3 packet type received from the neighbor (§4.6) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the recvSeq high-water store keyed by packet type at internal/plugins/ospf/auth_keystore.go:352-361 records OSPFv2-trailer sequences only; no OSPFv3 trailer sequence is ever received to store |
| `RFC7166-3-1` | Set KeyStartAccept less than KeyStartGenerate for smooth key transition (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-3-2` | Set KeyStopGenerate less than KeyStopAccept for smooth key transition (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-3-4` | Persist key storage across warm or cold system restarts (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-3-5` | Notify the network operator when the last key associated with an interface expires (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.1-1` | Set the Reserved field to 0 when sending protocol packets (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.2-1` | Set the OSPFv3 header checksum to 0 before computing the Authentication Trailer digest on transmit (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.2-2` | Set the LLS data block checksum to 0 before computing the Authentication Trailer digest on transmit (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.3-2` | Include support for HMAC-SHA-1 (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.6-2` | Fully support OSPFv3 Link-Local Signaling (§4.6) | RECOMMENDED | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.6-4` | Log an error event when an OSPFv3 packet is dropped for replay or digest-verification failure (§4.6) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-5-1` | Migrate all OSPFv3 routers on a link to authentication at the same time (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-6-1` | Use sufficiently long and random values for the Authentication Key (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-6-2` | Have Authentication Keys incorporate at least 128 pseudorandom bits (§6) | RECOMMENDED | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-6-3` | Support hexadecimal input of Authentication Keys in management systems (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-4.3-3` | Include support for HMAC-SHA-384 and HMAC-SHA-512 (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7166-5-2` | Implement a transition mode that adds the Authentication Trailer to transmitted packets without verifying received packets (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7166-2.1-1`](#rfc7166-2.1-1) OSPFv3 routers set the AT-bit in all OSPFv3 Hello and Database Description packets that contain an Authentication Trailer (§2.1) | {gap}, no test | no AT-bit exists in the OSPFv3 Options bitset -- internal/plugins/ospf/v3/types/options.go:14-23 defines only V6/E/N/R/AF, and the Hello/DD encoders internal/plugins/ospf/v3/packet/hello.go:88 and dbdesc.go:73 never set one, so no Authentication Trailer is emitted |
| [`RFC7166-3-3`](#rfc7166-3-3) When a new key replaces an old key, the new key's KeyStartGenerate be less than or equal to the old key's KeyStopGenerate (§3) | {gap}, no test | ze has no OSPFv3 Security Association carrying the RFC 7166 KeyStartAccept/KeyStartGenerate/KeyStopGenerate/KeyStopAccept lifetimes -- the only key model is the OSPFv2 send-lifetime pair in internal/plugins/ospf/auth_keystore.go:29-32, and validateKeyRollover internal/plugins/ospf/config.go:1007 orders OSPFv2 send-lifetimes only |
| [`RFC7166-3-6`](#rfc7166-3-6) Transmit an OSPFv3 packet unauthenticated when the last key associated with an interface has expired (§3) | {gap}, no test | no OSPFv3 Authentication Trailer is ever produced -- internal/plugins/ospf/auth_wiring.go:28 signPacket appends the OSPFv2 trailer via internal/plugins/ospf/packet/auth_verify.go:157, so there is no OSPFv3 authenticated-transmit path whose key expiry this could govern; the never-revert selection in auth_keystore.go:281-286 is OSPFv2-only |
| [`RFC7166-4.1-2`](#rfc7166-4.1-2) Ignore the Reserved field when receiving protocol packets (§4.1) | {gap}, no test | no OSPFv3 trailer decoder exists -- the only receive-side auth verify internal/plugins/ospf/auth_wiring.go:56 verifyPacket delegates to internal/plugins/ospf/auth_keystore.go:330 which decodes an OSPFv2 common header, so an OSPFv3 trailer Reserved field is never parsed |
| [`RFC7166-4.1-3`](#rfc7166-4.1-3) Increment the 64-bit cryptographic sequence number for every OSPFv3 packet sent (§4.1) | {gap}, no test | the 64-bit ESN counter in internal/plugins/ospf/auth_keystore.go:304-314 feeds the OSPFv2 packet.Sign trailer, not an OSPFv3 trailer; no OSPFv3 Cryptographic Sequence Number field is ever written |
| [`RFC7166-4.1-4`](#rfc7166-4.1-4) On reception, require the sequence number to be greater than that of the last accepted OSPFv3 packet of the same packet type from the sending neighbor (§4.1) | {gap}, no test | the per-packet-type strictly-greater replay check internal/plugins/ospf/auth_keystore.go:352-361 runs only inside the OSPFv2 verify path; there is no OSPFv3 trailer receive path to which it applies |
| [`RFC7166-4.1-5`](#rfc7166-4.1-5) Use available mechanisms to preserve the sequence number's strictly increasing property for the router's deployed life, including cold restarts (§4.1) | {gap}, no test | the persisted boot-count mechanism internal/plugins/ospf/auth_keystore.go:115 loadOSPFBootCount seeds the OSPFv2 ESN sequence only; no OSPFv3 trailer sequence is emitted for it to protect |
| [`RFC7166-4.1.1-1`](#rfc7166-4.1.1-1) Reset all keys before the 64-bit sequence number can wrap, to avoid replay attacks (§4.1.1) | {gap}, no test | no OSPFv3 trailer or its 64-bit sequence exists to wrap -- internal/plugins/ospf/auth_keystore.go:311-314 only bumps bootCount on the OSPFv2 ESN low-word wrap and performs no key reset |
| [`RFC7166-4.2-3`](#rfc7166-4.2-3) Omit OSPFv3 header checksum verification for received packets that include an Authentication Trailer (§4.2) | {gap}, no test | the receive path never detects an OSPFv3 Authentication Trailer, so it cannot conditionally omit header-checksum verification; only the transmit-side checksum-omission hook exists at internal/plugins/ospf/v3/packet/checksum.go:165-167 and internal/plugins/ospf/v3/packet/header.go:277-280 |
| [`RFC7166-4.2-4`](#rfc7166-4.2-4) Omit LLS data block checksum verification for received packets that include an Authentication Trailer (§4.2) | {gap}, no test | the OSPFv3 codec implements no LLS data block -- there is no LLS type under internal/plugins/ospf/v3/packet/ -- and no trailer receive path, so there is no LLS checksum to conditionally omit |
| [`RFC7166-4.3-1`](#rfc7166-4.3-1) Include support for at least HMAC-SHA-256 (§4.3) | {gap}, no test | the HMAC-SHA-256 primitive exists only for the OSPFv2 trailer at internal/plugins/ospf/packet/auth_verify.go:69 and :120; no OSPFv3 Authentication Trailer wires any algorithm |
| [`RFC7166-4.3-4`](#rfc7166-4.3-4) Use HMAC-SHA-256 as the default authentication algorithm (§4.3) | {gap}, no test | no OSPFv3 trailer means no OSPFv3 default algorithm; the OSPFv2 keychain algorithm is operator-selected per key at internal/plugins/ospf/auth_keystore.go:239-252 with no HMAC-SHA-256 default |
| [`RFC7166-4.4-1`](#rfc7166-4.4-1) Append the two-octet OSPFv3 Cryptographic Protocol ID to the Authentication Key prior to use; other protocols sharing common keys similarly append their own IDs (§4.4) | {gap}, no test | only the OSPFv2 Cryptographic Protocol ID 0x0001 is appended, and only for the OSPFv2 AuType-3 trailer at internal/plugins/ospf/packet/auth_verify.go:38 and :195; there is no OSPFv3 Cryptographic Protocol ID constant or OSPFv3 trailer to apply it to |
| [`RFC7166-4.6-1`](#rfc7166-4.6-1) Minimally support examining the L-bit in the OSPFv3 Options and using the LLS data block length to access the Authentication Trailer (§4.6) | {gap}, no test | the OSPFv3 codec parses no LLS block, defines no L-bit in internal/plugins/ospf/v3/types/options.go:14-23, and defines no trailer, so it cannot locate an Authentication Trailer past an LLS block |
| [`RFC7166-4.6-3`](#rfc7166-4.6-3) Drop a packet whose cryptographic sequence number is less than or equal to the last accepted value for that neighbor and OSPFv3 packet type (§4.6) | {gap}, no test | the seq<=last drop at internal/plugins/ospf/auth_keystore.go:357-359 is reached only via the OSPFv2 verify path; no OSPFv3 trailer receive path exists |
| [`RFC7166-4.6-5`](#rfc7166-4.6-5) Discard the packet when the computed digest does not match the received Authentication Data (§4.6) | {gap}, no test | the constant-time digest compare at internal/plugins/ospf/packet/auth_verify.go:242 and :263 verifies the OSPFv2 trailer only; there is no OSPFv3 trailer digest to verify |
| [`RFC7166-4.6-6`](#rfc7166-4.6-6) After successful authentication, store the 64-bit cryptographic sequence number for each OSPFv3 packet type received from the neighbor (§4.6) | {gap}, no test | the recvSeq high-water store keyed by packet type at internal/plugins/ospf/auth_keystore.go:352-361 records OSPFv2-trailer sequences only; no OSPFv3 trailer sequence is ever received to store |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7166-2.1-1`](#rfc7166-2.1-1)

OSPFv3 routers set the AT-bit in all OSPFv3 Hello and Database Description packets that contain an Authentication Trailer (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-2.1-1, so no unit is bound to it.

### [`RFC7166-3-3`](#rfc7166-3-3)

When a new key replaces an old key, the new key's KeyStartGenerate be less than or equal to the old key's KeyStopGenerate (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-3-3, so no unit is bound to it.

### [`RFC7166-3-6`](#rfc7166-3-6)

Transmit an OSPFv3 packet unauthenticated when the last key associated with an interface has expired (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-3-6, so no unit is bound to it.

### [`RFC7166-4.1-2`](#rfc7166-4.1-2)

Ignore the Reserved field when receiving protocol packets (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.1-2, so no unit is bound to it.

### [`RFC7166-4.1-3`](#rfc7166-4.1-3)

Increment the 64-bit cryptographic sequence number for every OSPFv3 packet sent (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.1-3, so no unit is bound to it.

### [`RFC7166-4.1-4`](#rfc7166-4.1-4)

On reception, require the sequence number to be greater than that of the last accepted OSPFv3 packet of the same packet type from the sending neighbor (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.1-4, so no unit is bound to it.

### [`RFC7166-4.1-5`](#rfc7166-4.1-5)

Use available mechanisms to preserve the sequence number's strictly increasing property for the router's deployed life, including cold restarts (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.1-5, so no unit is bound to it.

### [`RFC7166-4.1.1-1`](#rfc7166-4.1.1-1)

Reset all keys before the 64-bit sequence number can wrap, to avoid replay attacks (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.1.1-1, so no unit is bound to it.

### [`RFC7166-4.2-3`](#rfc7166-4.2-3)

Omit OSPFv3 header checksum verification for received packets that include an Authentication Trailer (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.2-3, so no unit is bound to it.

### [`RFC7166-4.2-4`](#rfc7166-4.2-4)

Omit LLS data block checksum verification for received packets that include an Authentication Trailer (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.2-4, so no unit is bound to it.

### [`RFC7166-4.3-1`](#rfc7166-4.3-1)

Include support for at least HMAC-SHA-256 (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.3-1, so no unit is bound to it.

### [`RFC7166-4.3-4`](#rfc7166-4.3-4)

Use HMAC-SHA-256 as the default authentication algorithm (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.3-4, so no unit is bound to it.

### [`RFC7166-4.4-1`](#rfc7166-4.4-1)

Append the two-octet OSPFv3 Cryptographic Protocol ID to the Authentication Key prior to use; other protocols sharing common keys similarly append their own IDs (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.4-1, so no unit is bound to it.

### [`RFC7166-4.6-1`](#rfc7166-4.6-1)

Minimally support examining the L-bit in the OSPFv3 Options and using the LLS data block length to access the Authentication Trailer (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.6-1, so no unit is bound to it.

### [`RFC7166-4.6-3`](#rfc7166-4.6-3)

Drop a packet whose cryptographic sequence number is less than or equal to the last accepted value for that neighbor and OSPFv3 packet type (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.6-3, so no unit is bound to it.

### [`RFC7166-4.6-5`](#rfc7166-4.6-5)

Discard the packet when the computed digest does not match the received Authentication Data (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.6-5, so no unit is bound to it.

### [`RFC7166-4.6-6`](#rfc7166-4.6-6)

After successful authentication, store the 64-bit cryptographic sequence number for each OSPFv3 packet type received from the neighbor (§4.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7166-4.6-6, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7166, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7166, so its obligations are stated where they were written.
