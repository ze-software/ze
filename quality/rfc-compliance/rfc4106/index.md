# RFC 4106 - The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP)

Partial. Every requirement this repository extracted from RFC 4106, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 37.5% | 3 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 2 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 8 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 37.5% | 3 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc4106.md` |
| Requirement shard | `rfc/requirements/rfc4106.md` |
| RFC text | `rfc/full/rfc4106.txt` |

## Enrolment

Enrolled: AES-GCM for ESP: eight MUST-level requirements, every one classified. Three are performed by the ESP datapath on the association ze installs and carry {lower-layer} (IV size, IV uniqueness, nonce reuse). Five are ze's own: IKEv2 keying (RFC4106-2-2), the 16-octet ICV ze installs (RFC4106-6-1), the four-octet salt in the KEYMAT (RFC4106-8.1-1, RFC4106-10-1) and rekeying (RFC4106-10-2); internal/component/ike/crypto/aead.go and internal/component/ike/dataplane/xfrm_linux.go produce them and they owe tagged tests. The two Section 8.4 sites bind an IKEv1 Phase 2 negotiator, which ze is not.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- ENCR_AES_GCM_16 (Transform ID 20) with 128- and 256-bit keys for ESP and the IKE SA, four-octet salt from KEYMAT, 16-octet ICV
- per-packet processing by Linux XFRM or VPP.


**What the ledger says remains:**

The 8- and 12-octet ICV variants (Transform IDs 18 and 19) and 192-bit keys are not offered. Requirement tests are owed for [`RFC4106-2-2`](#rfc4106-2-2), [`RFC4106-6-1`](#rfc4106-6-1), [`RFC4106-8.1-1`](#rfc4106-8.1-1), [`RFC4106-10-1`](#rfc4106-10-1) and [`RFC4106-10-2`](#rfc4106-10-2).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC4106-2-2`](#rfc4106-2-2), [`RFC4106-6-1`](#rfc4106-6-1), [`RFC4106-8.1-1`](#rfc4106-8.1-1)

**Annotated (including scoped evidence) (5):** [`RFC4106-2-1`](#rfc4106-2-1), [`RFC4106-3.1-1`](#rfc4106-3.1-1), [`RFC4106-3.1-2`](#rfc4106-3.1-2), [`RFC4106-10-1`](#rfc4106-10-1), [`RFC4106-10-2`](#rfc4106-10-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4106-2-1` | The same nonce and key combination MUST NOT be used more than once. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the datapath chooses every IV, so no value ze writes decides the nonce |
| `RFC4106-2-2` | For safety's sake, implementations MUST use an automated key management system, such as the Internet Key Exchange (IKE) [RFC2409], to ensure that this requirement is met. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L66). **negative:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L49) |
| `RFC4106-3.1-1` | The AES-GCM-ESP IV field MUST be eight octets. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)); internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the algorithm and the kernel writes the IV field of every packet |
| `RFC4106-3.1-2` | For a given key, the IV MUST NOT repeat. (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs no IV, so the kernel alone decides IV uniqueness |
| `RFC4106-6-1` | Implementations MUST support a full-length 16-octet ICV, and MAY support 8 or 12 octet ICVs, and MUST NOT support other ICV lengths. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L40). **negative:** `unit/verify` [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L54) |
| `RFC4106-8.1-1` | The size of the KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the associated AES key. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L41). **negative:** `unit/verify` [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L54) |
| `RFC4106-10-1` | Thus, to avoid counter block collisions, ESP implementations that permit use of the same key for encrypting and decrypting packets with the same peer MUST ensure that the two peers assign different salt values to the security association (SA). (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4106DirectionsTakeDifferentSalts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L78). **negative:** no negative test. **{single-polarity}:** every AES-GCM Child SA takes its two keys from consecutive runs of one prf+ stream, so there is no input for which the two directions may share a salt and no contrasting case to test |
| `RFC4106-10-2` | To protect against this problem, implementations MUST generate a fresh key before encrypting 2^64 blocks of data with a given key. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4106GCMKeyCannotOutliveTheSequenceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L85). **negative:** no negative test. **{single-polarity}:** the bound is a property of every state xfrmStateFromParams builds (sequence rollover forbidden, no ESN, so at most 2^32 packets of at most 4096 blocks per key), and no state exists without it to contrast |
| `RFC4106-3.2-1` | Implementations that do not seek to hide the length of the plaintext SHOULD use the minimum amount of padding required, which will be less than four octets. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-4-1` | The salt SHOULD be unpredictable (i.e., chosen at random) before it is selected, but need not be secret. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-8.2-1` | Implementations SHOULD use an IKE Phase 1 cipher that is at least as strong as AES-GCM. (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-8.2-2` | The use of AES CBC [RFC3602] with the same key size used by AES-GCM-ESP is RECOMMENDED. (§8.2) | RECOMMENDED | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-10-3` | Implementations SHOULD use as large a size as reasonable. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4106-2-1`](#rfc4106-2-1) The same nonce and key combination MUST NOT be used more than once. (§2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the datapath chooses every IV, so no value ze writes decides the nonce |
| [`RFC4106-3.1-1`](#rfc4106-3.1-1) The AES-GCM-ESP IV field MUST be eight octets. (§3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)); internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the algorithm and the kernel writes the IV field of every packet |
| [`RFC4106-3.1-2`](#rfc4106-3.1-2) For a given key, the IV MUST NOT repeat. (§3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs no IV, so the kernel alone decides IV uniqueness |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4106-2-1`](#rfc4106-2-1)

The same nonce and key combination MUST NOT be used more than once. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4106-2-1, so no unit is bound to it.

### [`RFC4106-2-2`](#rfc4106-2-2)

For safety's sake, implementations MUST use an automated key management system, such as the Internet Key Exchange (IKE) [RFC2409], to ensure that this requirement is met. (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L66) | unit/verify | revert, verified |

### [`RFC4106-3.1-1`](#rfc4106-3.1-1)

The AES-GCM-ESP IV field MUST be eight octets. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4106-3.1-1, so no unit is bound to it.

### [`RFC4106-3.1-2`](#rfc4106-3.1-2)

For a given key, the IV MUST NOT repeat. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4106-3.1-2, so no unit is bound to it.

### [`RFC4106-6-1`](#rfc4106-6-1)

Implementations MUST support a full-length 16-octet ICV, and MAY support 8 or 12 octet ICVs, and MUST NOT support other ICV lengths. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L40) | unit/verify | revert, verified |

### [`RFC4106-8.1-1`](#rfc4106-8.1-1)

The size of the KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the associated AES key. (§8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L41) | unit/verify | revert, verified |

### [`RFC4106-10-1`](#rfc4106-10-1)

Thus, to avoid counter block collisions, ESP implementations that permit use of the same key for encrypting and decrypting packets with the same peer MUST ensure that the two peers assign different salt values to the security association (SA). (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4106DirectionsTakeDifferentSalts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L78) | unit/verify | revert, verified |

### [`RFC4106-10-2`](#rfc4106-10-2)

To protect against this problem, implementations MUST generate a fresh key before encrypting 2^64 blocks of data with a given key. (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4106GCMKeyCannotOutliveTheSequenceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L85) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4106.txt |
| Source fingerprint | c1d66da1e4b757c6 |
| Record | rfc/extraction/rfc4106.json |
| Mapped sentences | 8 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 1 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `13` | acknowledgements | 0 | skipped (acknowledgements) | acknowledgements |
| `14` | normative references | 0 | skipped (references) | normative references |
| `15` | informative references | 0 | skipped (references) | informative references |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IKEv1 Phase 2 negotiator: Section 8.4 states the Key Length attribute of the IKE Phase 2 exchange [RFC2407], which is IKEv1. ze implements IKEv2 only (internal/component/ike/engine), whose Key Length attribute RFC 7296 Section 3.3.5 governs | Because the AES supports three key lengths, the Key Length attribute MUST be specified in the IKE Phase 2 exchange [RFC2407]. |
| `8.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IKEv1 Phase 2 negotiator: Section 8.4 states the Key Length attribute of the IKE Phase 2 exchange [RFC2407], which is IKEv1. ze implements IKEv2 only (internal/component/ike/engine), whose Key Length attribute RFC 7296 Section 3.3.5 governs | The Key Length attribute MUST have a value of 128, 192, or 256. |

## Superseded

No document obsoletes RFC 4106, so its obligations are stated where they were written.
