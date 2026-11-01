# RFC 5304 - IS-IS Cryptographic Authentication

IS-IS HMAC-MD5 authentication. Every requirement this repository extracted from RFC 5304, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 9 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1 | of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 9 gated MUSTs: they add up to 9. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | IS-IS HMAC-MD5 authentication |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5304.md` |
| Requirement shard | `rfc/requirements/rfc5304.md` |
| RFC text | `rfc/full/rfc5304.txt` |

## Enrolment

Enrolled: IS-IS Cryptographic Authentication (HMAC-MD5, Authentication TLV type 54): nine MUST-level requirements. Eight are met with positive+negative tags in internal/plugins/isis: 2-1 and 2-2 (Level-1 area and Level-2 domain authentication with the correct key chain), 2-3 (Hello/IIH link-level authentication), 2-6 (reject a PDU whose HMAC does not match), 2-7 (a signed purge carries only the Authentication TLV), 2-8 (do not accept an unauthenticated purge under configured auth), and 2-9 (reject a purge carrying extra TLVs). 2-4 (sign over the padded PDU) is {single-polarity: positive} with a new test capturing the bytes handed to the signer. 2-5 (do not include the auth value in the per-PDU Checksum TLV) is a gap in proof: ze implements no optional IS-IS per-PDU Checksum TLV (no TLV 12 codec), so no PDU carries one, and no test asserts it.

## What the public ledger says

**Area:** IS-IS HMAC-MD5 authentication

**Proof:** 0 of 9 MUST proven

**Evidence:** -

**Unmet:** RFC5304-2-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5304-2-1`](#rfc5304-2-1), [`RFC5304-2-2`](#rfc5304-2-2), [`RFC5304-2-3`](#rfc5304-2-3), [`RFC5304-2-6`](#rfc5304-2-6), [`RFC5304-2-7`](#rfc5304-2-7), [`RFC5304-2-8`](#rfc5304-2-8), [`RFC5304-2-9`](#rfc5304-2-9)

**Annotated (including scoped evidence) (2):** [`RFC5304-2-4`](#rfc5304-2-4), [`RFC5304-2-5`](#rfc5304-2-5)

## Figures

- **MUST bar**: Ze correctness gap 9
- **Overview bar**: SHOULD not decided 5, Ze correctness gap 9

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 9 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 5 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc5304.json](../../../data/rfc/rfc5304.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5304-2-1` | Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L106). **negative:** `unit/verify` [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L188) |
| `RFC5304-2-2` | Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L109). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L267) |
| `RFC5304-2-3` | IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L150). **negative:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158) |
| `RFC5304-2-4` | The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L69). **negative:** no negative test. **{single-polarity}:** the LAN/P2P hello is padded then signed in one straight-line path (internal/plugins/isis/circuit/runtime.go:273-276), and there is no unpadded-sign code path to drive a negative |
| `RFC5304-2-5` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/isis/packet meets it, since ze implements no optional Checksum TLV (TLV 12) codec, so no SNP or IIH it builds carries one; no test asserts it |
| `RFC5304-2-6` | An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) |
| `RFC5304-2-7` | ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) |
| `RFC5304-2-8` | ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) |
| `RFC5304-2-9` | ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) |
| `RFC5304-2.1-1` | One possible solution to this problem is for the IS-IS process to detect if any inbound LSP with an authentication failure has the local System ID and also has a higher Sequence Number than the IS-IS process has. In this event, the IS-IS process SHOULD increase its own LSP Sequence Number accordingly and re-flood the LSPs. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-10` | IS-IS Hello PDUs SHALL use the Link Level Authentication String, which MAY be different from that of Link State PDUs. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-11` | An implementation MAY have a transition mode where it includes HMAC- MD5 Authentication Information in PDUs but does not verify the HMAC- MD5 Authentication Information. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-12` | An implementation MAY check a set of passwords when verifying the Authentication Value (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-13` | An implementation that does not implement HMAC-MD5 authentication MAY accept a PDU that contains the HMAC-MD5 Authentication Type (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5304-2-5`](#rfc5304-2-5) Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | {gap}, no test | gap in proof: internal/plugins/isis/packet meets it, since ze implements no optional Checksum TLV (TLV 12) codec, so no SNP or IIH it builds carries one; no test asserts it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5304-2-1`](#rfc5304-2-1)

Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L188) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L106) | unit/verify | unproven |

### [`RFC5304-2-2`](#rfc5304-2-2)

Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L267) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L109) | unit/verify | unproven |

### [`RFC5304-2-3`](#rfc5304-2-3)

IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158) | unit/verify | unproven |
| positive | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L150) | unit/verify | unproven |

### [`RFC5304-2-4`](#rfc5304-2-4)

The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L69) | unit/verify | unproven |

### [`RFC5304-2-5`](#rfc5304-2-5)

Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5304-2-5, so no unit is bound to it.

### [`RFC5304-2-6`](#rfc5304-2-6)

An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608) | unit/verify | unproven |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131) | unit/verify | unproven |

### [`RFC5304-2-7`](#rfc5304-2-7)

ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455) | unit/verify | unproven |

### [`RFC5304-2-8`](#rfc5304-2-8)

ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447) | unit/verify | unproven |

### [`RFC5304-2-9`](#rfc5304-2-9)

ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5304.txt |
| Source fingerprint | 125142e8956ce6f3 |
| Record | rfc/extraction/rfc5304.json |
| Mapped sentences | 9 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 9 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5304 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5304, so its obligations are stated where they were written.
