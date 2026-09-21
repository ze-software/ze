# RFC 1334 - PPP Authentication Protocols

Partial. Every requirement this repository extracted from RFC 1334, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 38.5% | 5 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.7% | 1 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 7.7% | 1 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 46.2% | 6 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 16 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 6 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1334.md` |
| Requirement shard | `rfc/requirements/rfc1334.md` |
| RFC text | `rfc/full/rfc1334.txt` |

## Enrolment

Enrolled: PPP Authentication Protocols (PAP and the LCP Authentication-Protocol option): seven MUST-level requirements. Five are met with positive+negative tags in internal/component/l2tp/ppp: 1-1 (advertise Auth-Protocol in LCP CONFREQ when auth is desired), x-1 (offer CHAP before PAP via the default fallback order), 2.3-1 (Authenticate-Ack Code 2 on accept), 2.3-2 (Authenticate-Nak Code 3 on reject), and 2.3-3 (echo the request Identifier into the reply). 2.3-3 is {single-polarity: positive} (the reply copies the Identifier and never validates or rejects on it). 2.3-4 (the Ack/Nak Message field must not affect operation) is met by a new pppoeclient test proving runClientAuth branches only on Code. 2.2-1 (change Identifier on reissue) is {not-applicable}: ze never reissues a PAP Authenticate-Request.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

PAP authentication option through PPP auth handling.

**What the ledger says remains:**

Carries the L2TP and PPPoE Partial status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 6 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC1334-1-1`](#rfc1334-1-1), [`RFC1334-x-1`](#rfc1334-x-1), [`RFC1334-2.3-1`](#rfc1334-2.3-1), [`RFC1334-2.3-2`](#rfc1334-2.3-2), [`RFC1334-2.3-4`](#rfc1334-2.3-4)

**Annotated instead of tested (2):** [`RFC1334-2.2-1`](#rfc1334-2.2-1), [`RFC1334-2.3-3`](#rfc1334-2.3-3)

**No test and no annotation (6):** [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1), [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2), [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3), [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4), [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5), [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1334-1-1` | If authentication is desired, specify Authentication-Protocol Configuration Option during Link Establishment phase (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L669). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) |
| `RFC1334-x-1` | Any implementation that includes a stronger authentication method (such as CHAP) must offer to negotiate that method prior to PAP (Security Considerations) | MUST | x | **positive:** `unit/verify` [`TestDefaultAuthFallbackOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_test.go#L269). **negative:** `unit/verify` [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_test.go#L111) |
| `RFC1334-2.3-1` | If id/password pair is recognizable and acceptable, authenticator must transmit Authenticate-Ack (Code=2) (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L293). **negative:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L411) |
| `RFC1334-2.3-2` | If id/password pair is not recognizable or acceptable, authenticator must transmit Authenticate-Nak (Code=3) (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L407). **negative:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L297) |
| `RFC1334-2.2.1-1` | The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2.1-2` | The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2.1-3` | Upon reception of an Authenticate-Request packet, some type of Authenticate reply MUST be returned (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2.1-4` | Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate-Request packets after completing the Authentication phase (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2.1-5` | Any Authenticate-Request packets received during the Network-Layer Protocol phase MUST return the same reply Code returned when the Authentication phase completed (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2.1-6` | Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2-1` | Identifier field must be changed each time an Authenticate-Request is issued (Section 2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never issues a second PAP Authenticate-Request; the authenticator role only responds (internal/component/l2tp/ppp/pap.go:160) and the client issues exactly one request per session with a fixed Identifier and no reissue path (internal/component/l2tp/pppoeclient/session.go:230-241), so the change-Identifier-on-reissue obligation has no code path |
| `RFC1334-2.3-3` | Ack/Nak Identifier must be copied from the Authenticate-Request which caused the reply (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L414). **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L300). **negative:** no negative test. **{single-polarity}:** writePAPReply copies the request Identifier into the PAP reply (internal/component/l2tp/ppp/pap.go:138) and ze never validates or rejects on the Identifier, so there is no negative case |
| `RFC1334-2.3-4` | Message field must not affect operation of the protocol (Section 2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L49). **negative:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L53) |
| `RFC1334-2.3-5` | Authenticator should take action to terminate the link on Nak (Section 2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.3-6` | Message field may be empty (Section 2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2-2` | Implementations may retry Authenticate-Request at implementation-defined intervals (Section 2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1) The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2) The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3) Upon reception of an Authenticate-Request packet, some type of Authenticate reply MUST be returned (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4) Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate-Request packets after completing the Authentication phase (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5) Any Authenticate-Request packets received during the Network-Layer Protocol phase MUST return the same reply Code returned when the Authentication phase completed (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6) Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | no test | no test carries this requirement id |
| [`RFC1334-2.2-1`](#rfc1334-2.2-1) Identifier field must be changed each time an Authenticate-Request is issued (Section 2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never issues a second PAP Authenticate-Request; the authenticator role only responds (internal/component/l2tp/ppp/pap.go:160) and the client issues exactly one request per session with a fixed Identifier and no reissue path (internal/component/l2tp/pppoeclient/session.go:230-241), so the change-Identifier-on-reissue obligation has no code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1334-1-1`](#rfc1334-1-1)

If authentication is desired, specify Authentication-Protocol Configuration Option during Link Establishment phase (Section 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) | unit/verify | unproven |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L669) | unit/verify | unproven |

### [`RFC1334-x-1`](#rfc1334-x-1)

Any implementation that includes a stronger authentication method (such as CHAP) must offer to negotiate that method prior to PAP (Security Considerations)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_test.go#L111) | unit/verify | unproven |
| positive | [`TestDefaultAuthFallbackOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_test.go#L269) | unit/verify | unproven |

### [`RFC1334-2.3-1`](#rfc1334-2.3-1)

If id/password pair is recognizable and acceptable, authenticator must transmit Authenticate-Ack (Code=2) (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L411) | unit/verify | unproven |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L293) | unit/verify | unproven |

### [`RFC1334-2.3-2`](#rfc1334-2.3-2)

If id/password pair is not recognizable or acceptable, authenticator must transmit Authenticate-Nak (Code=3) (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L297) | unit/verify | unproven |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L407) | unit/verify | unproven |

### [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1)

The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-1, so no unit is bound to it.

### [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2)

The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-2, so no unit is bound to it.

### [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3)

Upon reception of an Authenticate-Request packet, some type of Authenticate reply MUST be returned (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-3, so no unit is bound to it.

### [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4)

Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate-Request packets after completing the Authentication phase (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-4, so no unit is bound to it.

### [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5)

Any Authenticate-Request packets received during the Network-Layer Protocol phase MUST return the same reply Code returned when the Authentication phase completed (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-5, so no unit is bound to it.

### [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6)

Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2.1-6, so no unit is bound to it.

### [`RFC1334-2.2-1`](#rfc1334-2.2-1)

Identifier field must be changed each time an Authenticate-Request is issued (Section 2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1334-2.2-1, so no unit is bound to it.

### [`RFC1334-2.3-3`](#rfc1334-2.3-3)

Ack/Nak Identifier must be copied from the Authenticate-Request which caused the reply (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L414) | unit/verify | unproven |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/pap_test.go#L300) | unit/verify | unproven |

### [`RFC1334-2.3-4`](#rfc1334-2.3-4)

Message field must not affect operation of the protocol (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L53) | unit/verify | unproven |
| positive | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L49) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc1334.txt |
| Source fingerprint | 34483221567656af |
| Record | rfc/extraction/rfc1334.json |
| Mapped sentences | 13 |
| Declined as scope | 18 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 3 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 7 | walked | not stated |
| `2.2.2` | not stated | 4 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 10 | walked | not stated |
| `3.2.2` | not stated | 4 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | MUST This word, or the adjective "required", means that the definition is an absolute requirement of the specification. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | MUST NOT This phrase means that the definition is an absolute prohibition of the specification. |
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. |
| `3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. |
| `3.2.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). |
| `3.2.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. |
| `3.2.1:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). |
| `3.2.1:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). |
| `3.2.1:5` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Implementation Note: Because the Success might be lost, the authenticator MUST allow repeated Response packets after completing the Authentication phase. |
| `3.2.1:6` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code returned when the Authentication phase completed (the message portion MAY be different). |
| `3.2.1:7` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Any Response packets received during any other phase MUST be silently discarded. |
| `3.2.1:8` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Identifier field MUST be changed each time a Challenge is sent. |
| `3.2.1:9` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. |
| `3.2.1:10` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Challenge Value MUST be changed each time a Challenge is sent. |
| `3.2.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). |
| `3.2.2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. |
| `3.2.2:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. |
| `3.2.2:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | It is intended to be human readable, and MUST NOT affect operation of the protocol. |

## Superseded

RFC 1334 is obsoleted by RFC 1994.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC1334-1-1`](#rfc1334-1-1) If authentication is desired, specify Authentication-Protocol Configuration Option during Link Establishment phase (Section 1) | restated | RFC1994-1-1 | RFC 1994 Section 1 repeats the sentence word for word, that an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase if authentication of the link is desired. The obligation is PPP-wide and binds neither authentication protocol in particular |
| [`RFC1334-x-1`](#rfc1334-x-1) Any implementation that includes a stronger authentication method (such as CHAP) must offer to negotiate that method prior to PAP (Security Considerations) | dropped | not stated | RFC 1994 defines CHAP alone and states no obligation to offer a stronger method before PAP. Its Security Considerations warn that authenticating one user name by several methods exposes the least secure of them, and recommend one method per user name, with no keyword. The rule is still owed for as long as PAP is offered |
| [`RFC1334-2.3-1`](#rfc1334-2.3-1) If id/password pair is recognizable and acceptable, authenticator must transmit Authenticate-Ack (Code=2) (Section 2.3) | dropped | not stated | RFC 1994 defines no PAP packet. Its Section 4.2 obliges a CHAP packet with Code 3 (Success) when the received Response Value equals the expected value, which is the CHAP handshake rather than the PAP Authenticate-Ack. PAP stays defined by RFC 1334, as this summary's forward Meta row records |
| [`RFC1334-2.3-2`](#rfc1334-2.3-2) If id/password pair is not recognizable or acceptable, authenticator must transmit Authenticate-Nak (Code=3) (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 obliges a CHAP packet with Code 4 (Failure) when the Response Value does not match, and defines no PAP Authenticate-Nak. The PAP rule is still owed for as long as PAP peers are authenticated |
| [`RFC1334-2.2-1`](#rfc1334-2.2-1) Identifier field must be changed each time an Authenticate-Request is issued (Section 2.2) | dropped | not stated | RFC 1994 states no PAP obligation. Its Section 4.1 requires the Identifier to change each time a Challenge is sent, which binds the CHAP Challenge and not a reissued PAP Authenticate-Request |
| [`RFC1334-2.3-3`](#rfc1334-2.3-3) Ack/Nak Identifier must be copied from the Authenticate-Request which caused the reply (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 requires the Success or Failure Identifier to be copied from the Response which caused the reply, which binds CHAP. It states nothing about a PAP Authenticate-Ack or Authenticate-Nak |
| [`RFC1334-2.3-4`](#rfc1334-2.3-4) Message field must not affect operation of the protocol (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 carries the same MUST NOT for the Message field of a CHAP Success or Failure packet. It states nothing about the PAP Ack and Nak Message field, which RFC 1334 still defines |
| [`RFC1334-2.3-5`](#rfc1334-2.3-5) Authenticator should take action to terminate the link on Nak (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 says a CHAP implementation SHOULD take action to terminate the link when it transmits a Failure. That binds the CHAP exchange, and RFC 1994 states nothing about a PAP Authenticate-Nak |
| [`RFC1334-2.3-6`](#rfc1334-2.3-6) Message field may be empty (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 says the Message field of a CHAP Success or Failure is zero or more octets. It states nothing about the PAP Ack and Nak Message field |
| [`RFC1334-2.2-2`](#rfc1334-2.2-2) Implementations may retry Authenticate-Request at implementation-defined intervals (Section 2.2) | dropped | not stated | RFC 1994 Section 4.1 obliges the authenticator to send further Challenges until a valid Response arrives or a retry counter expires, which is a CHAP obligation on the authenticator. RFC 1994 states nothing about a peer retrying a PAP Authenticate-Request |
