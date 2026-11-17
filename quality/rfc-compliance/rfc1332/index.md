# RFC 1332 - The PPP Internet Protocol Control Protocol (IPCP)

IPCP. Every requirement listed for RFC 1332, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 8 | of 8 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 8 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 8 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 8 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 8 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 20 | 20 of 21 tests linked to requirements, 0 without a fault test and 1 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 6 | of 8 mandatory requirements reviewed | 2 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 8 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 8 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 8 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 8 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 8 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 8 mandatory requirements: they add up to 8. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IPCP |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 21 |
| Tests linked to requirements | 21 |
| Recorded audit verdicts | 6 |
| Fault test records | 21 |
| Summary | `rfc/short/rfc1332.md` |
| Requirement table | `rfc/requirements/rfc1332.md` |
| RFC text | `rfc/full/rfc1332.txt` |

## Automatic checks

Included in automatic checks: PPP Internet Protocol Control Protocol under L2TP and PPPoE. Ze implements the IPCP option codec, address negotiation and interface programming. Tagged tests cover packet framing, negotiation ordering, address suggestions and the MTU passed to the backend. Tagged tests also cover the Code-Reject of the Codes IPCP does not use and the refusal of Van Jacobson compression. The checklist distinguishes Linux IP fragmentation from Ze's MTU setup.

## Published support status

**Area:** IPCP

**Test verification:** 8 of 8 MUST proven

**Evidence:** unit 8

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **8** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (8):** [`RFC1332-2-1`](#rfc1332-2-1), [`RFC1332-2-2`](#rfc1332-2-2), [`RFC1332-2.1-1`](#rfc1332-2.1-1), [`RFC1332-2.1-3`](#rfc1332-2.1-3), [`RFC1332-2.1-4`](#rfc1332-2.1-4), [`RFC1332-3-1`](#rfc1332-3-1), [`RFC1332-3.3-2`](#rfc1332-3.3-2), [`RFC1332-4.1-1`](#rfc1332-4.1-1)

## Figures

- **MUST bar**: Ze verified 8
- **Overview bar**: Ze MUST verified 8, Not assessed 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 8 |
| MUST rows kept (the MUST bar) | 8 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1332.json](../../../data/rfc/rfc1332.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1332-2-1` | Exactly one IPCP packet is encapsulated in the Information field of PPP Data Link Layer frames where the Protocol field indicates type hex 8021 (IP Control Protocol). (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145). **positive:** `unit/verify` [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L257). **negative:** `unit/verify` [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L258) |
| `RFC1332-2-2` | Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate-Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestClientIPCPUnusedCodesAreCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1332_ipcp_codes_test.go#L31). **positive:** `unit/verify` [`TestIPCPUnusedCodesAreCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L35). **negative:** `unit/verify` [`TestIPCPUsedCodesAreNotCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L74) |
| `RFC1332-2.1-1` | Before any IP packets may be communicated, PPP must reach the Network-Layer Protocol phase, and the IP Control Protocol must reach the Opened state. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L106). **negative:** `unit/verify` [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L167). **negative:** `unit/verify` [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L168) |
| `RFC1332-2.1-3` | The maximum length of an IP packet transmitted over a PPP link is the same as the maximum length of the Information field of a PPP data link layer frame. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L18). **negative:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L19) |
| `RFC1332-2.1-4` | Larger IP datagrams must be fragmented as necessary. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L20). **negative:** `unit/verify` [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L21) |
| `RFC1332-3-1` | IPCP uses the same Configuration Option format defined for LCP [1], with a separate set of Options. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIPCPOptionsWrittenInLCPFormat`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_clauses_test.go#L23). **positive:** `unit/verify` [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L14). **negative:** `unit/verify` [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L71) |
| `RFC1332-3.3-2` | The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestIPCPNakCarriesTheAssignedRemoteAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L299). **negative:** `unit/verify` [`TestIPCPNakNeverCarriesAnUnacceptableAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L313) |
| `RFC1332-3.1-1` | This option SHOULD NOT be sent in a Configure-Request if a Configure-Request has been received which includes either an IP- Addresses or IP-Address option. (§3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-4.1-1` | The slot identifier must not be compressed if there is no ability for the PPP link level to indicate an error in reception to the decompression module. (§4.1) | MUST NOT | 4.1 | **positive:** `unit/verify` [`TestIPCPNeverOffersVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L111). **negative:** `unit/verify` [`TestClientIPCPRejectsVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1332_ipcp_codes_test.go#L90). **negative:** `unit/verify` [`TestIPCPRejectsVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L151) |
| `RFC1332-2-3` | An implementation should be prepared to wait for Authentication and Link Quality Determination to finish before timing out waiting for a Configure-Ack or other response. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-2.1-2` | If a system wishes to avoid fragmentation and reassembly, it should use the TCP Maximum Segment Size option [4], and MTU discovery [5]. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.3-1` | If negotiation about the remote IP-address is required, and the peer did not provide the option in its Configure-Request, the option SHOULD be appended to a Configure-Nak. (Section 3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-2` | This option MAY be sent if a Configure-Reject is received for the IP-Address option, or a Configure-Nak is received with an IP-Addresses option as an appended option. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1332-3.1-3` | Support for this option MAY be removed after the IPCP protocol status advances to Internet Draft Standard. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1332 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1332-2-1`](#rfc1332-2-1)

Exactly one IPCP packet is encapsulated in the Information field of PPP Data Link Layer frames where the Protocol field indicates type hex 8021 (IP Control Protocol). (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1332 Section2, exactly one IPCP packet per PPP Information field with protocol0x8021. Read every tagged body: TestIPCPFrameCarriesExactlyOnePacket pins raw Protocol octets, decoded protocol, declared Length and whole Information span. TestIPCPOnePacketPerFrameThroughRun completes real LCP/auth/address request and reads actual emitted IPCP Configure-Request, requiring Length==Information length; its negative delivers one valid address Configure-Request followed by a separately well-formed Terminate-Request in the same frame and requires only the first Configure-Ack and no subsequent frame through a one-second bound. The second packet is padding after the first declared Length, not a malformed first request. ParseLCPPacket truncates to that declared length and runNCPPhase handles one packet per frame; the real session path distinguishes a writer defect from an inert codec-only fixture. Current answers use actual request capabilities, retaining the same inputs/assertions. This is semantic source judgment, not a fresh observed discrimination record.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestIPCPFrameCarriesExactlyOnePacket`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/frame_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestIPCPOnePacketPerFrameThroughRun`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L257) | unit/verify | revert, verified |

### [`RFC1332-2-2`](#rfc1332-2-2)

Only Codes 1 through 7 (Configure-Request, Configure-Ack, Configure-Nak, Configure-Reject, Terminate-Request, Terminate-Ack and Code-Reject) are used. Other Codes should be treated as unrecognized and should result in Code-Rejects. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPUsedCodesAreNotCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestIPCPUnusedCodesAreCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestClientIPCPUnusedCodesAreCodeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1332_ipcp_codes_test.go#L31) | unit/verify | revert, verified |

### [`RFC1332-2.1-1`](#rfc1332-2.1-1)

Before any IP packets may be communicated, PPP must reach the Network-Layer Protocol phase, and the IP Control Protocol must reach the Opened state. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC1332 Section2.1 in full: both Network-Layer Protocol phase and IPCP Opened are necessary before IP communication. Every tagged unit was read: TestIPResponseConfiguresInterface completes actual IPCP and pins exact ppp42 local/peer installation and EventSessionIPAssigned, then waits for SessionUp before observing admin-up; TestIPCPNoAddressBeforeOpened acknowledges only the original local request, holds AckRcvd short of Opened and forbids address, assigned event and admin-up. TestNCPHeldUntilNetworkPhase separately holds valid CHAP authentication pending with IPCP enabled, forbids address requests, installed addresses and non-CHAP wire traffic, then uses accepted authentication to prove the NCP path is live. afterLCPOpen runs authentication before runNCPPhase; onNCPOpened installs the address only on actual Opened transition, with interface activation after settlement. This covers both clauses at Ze's actual delegated kernel boundary; it does not claim observed kernel data forwarding. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPNoAddressBeforeOpened`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L167) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestNCPHeldUntilNetworkPhase`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1661_phase_lifecycle_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestIPResponseConfiguresInterface`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/ncp_test.go#L106) | unit/verify | revert, verified |

### [`RFC1332-2.1-3`](#rfc1332-2.1-3)

The maximum length of an IP packet transmitted over a PPP link is the same as the maximum length of the Information field of a PPP data link layer frame. (§2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Read RFC1332 Section2.1: the maximum IP packet length must equal the PPP Information field maximum. The sole tagged TestIPMTUInstalledFromNegotiatedMRU drives actual proxy-LCP/Driver admission with peer MRU1400 and receive capacity1500; it pins exactly one ppp7 MTU call at1400 and receive MRU1500. afterLCPOpen uses transmitMRU for SetMTU before channel connection. This is meaningful delegated-kernel evidence and the current MaxFrameLen-for-mru cut targets that exact observation. Both polarity tags nevertheless use the same input and exact1400 assertion; the later no-above1400 loop cannot fail independently after that assertion passed. Preserve the existing polarity-proof gap; mechanical helper/caller preservation does not upgrade it. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L19) | unit/verify | mutant, verified |
| positive | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L18) | unit/verify | mutant, verified |

### [`RFC1332-2.1-4`](#rfc1332-2.1-4)

Larger IP datagrams must be fragmented as necessary. (§2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Read RFC1332 Section2.1: larger IP datagrams must be fragmented as necessary. The sole tagged TestIPMTUInstalledFromNegotiatedMRU proves actual negotiated MRU1400 installs device MTU1400, the delegated Linux fragmentation threshold. It does not inject or observe an oversized datagram or kernel fragmentation. Delegation is the agreed production boundary, but the same unit's two tags share one1400 input and the exact same backend observation; its later above1400 loop adds no independent negative case. The current MaxFrameLen cut can discriminate the installed-MTU observation, not supply another polarity or an observed fragmentation event. Preserve existing weak verdict and installed-MTU scoped truth without inventing a production fragmentation defect. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L21) | unit/verify | mutant, verified |
| positive | [`TestIPMTUInstalledFromNegotiatedMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_mtu_test.go#L20) | unit/verify | mutant, verified |

### [`RFC1332-3-1`](#rfc1332-3-1)

IPCP uses the same Configuration Option format defined for LCP [1], with a separate set of Options. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestIPCPOptionsWrittenInLCPFormat changed only by the iPCPOptions -> IPCPOptions type rename; it still compares WriteIPCPOptions output byte for byte (Types 3/129/131, Length 6, four octets) and walks it with ParseLCPOptions, so any drift of Type, Length or Data is red. Receive side unchanged: TestIPCPParseOptions decodes a well-formed list, TestIPCPParseRejects refuses malformed TLVs with the exact errors.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPParseRejects`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestIPCPOptionsWrittenInLCPFormat`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_clauses_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestIPCPParseOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_ipcp_test.go#L14) | unit/verify | revert, verified |

### [`RFC1332-3.3-2`](#rfc1332-3.3-2)

The value of the IP-address given must be acceptable as the remote IP-address, or indicate a request that the peer provide the information. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/auth_phase_rfc1334_test.go::TestIPCPNakCarriesTheAssignedRemoteAddress, internal/component/l2tp/ppp/auth_phase_rfc1334_test.go::TestIPCPNakNeverCarriesAnUnacceptableAddress moved. Forbidden: a Configure-Nak IP-Address that is not acceptable as the remote address. TestIPCPNakCarriesTheAssignedRemoteAddress asserts the Nak carries the assigned 10.0.0.2 for a request of 10.9.9.9 and for a request with no address, red if another value is given. TestIPCPNakNeverCarriesAnUnacceptableAddress asserts the requested address is not echoed and that with nothing assigned the option is omitted rather than written invalid. The 'or indicate a request' branch is a permitted alternative Ze does not use.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPNakNeverCarriesAnUnacceptableAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L313) | unit/verify | revert, verified |
| positive | [`TestIPCPNakCarriesTheAssignedRemoteAddress`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L299) | unit/verify | revert, verified |

### [`RFC1332-4.1-1`](#rfc1332-4.1-1)

The slot identifier must not be compressed if there is no ability for the PPP link level to indicate an error in reception to the decompression module. (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPCPRejectsVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L151) | unit/verify | revert, verified |
| negative | [`TestClientIPCPRejectsVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1332_ipcp_codes_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestIPCPNeverOffersVanJacobsonCompression`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1332_codes_test.go#L111) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc1332.txt |
| Source fingerprint | 05b932877d43bddd |
| Record | rfc/extraction/rfc1332.json |
| Mapped sentences | 5 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 2 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `4.1` | not stated | 3 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 is a non-normative description of PPP's three components. The sentence describes the LCP establishment sequence that RFC 1331 defines, not an IPCP obligation this document places on an implementation. | In order to establish communications over a point-to-point link, each end of the PPP link must first send LCP packets to configure and test the data link. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Same non-normative introduction: it describes how the NCP family defined in RFC 1331 is used after LCP establishment. IPCP's own obligations start at Section 2. | After the link has been established and optional facilities have been negotiated as needed by the LCP, PPP must send NCP packets to choose and configure one or more network-layer protocols. |
| `4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Van Jacobson TCP/IP header compression is optional in this document and Ze declined it. Section 3.2 states the default: "By default, compression is not enabled." Ze never requests the IP-Compression-Protocol option and Configure-Rejects a peer's Type-2 option (isKnownIPCPOption, internal/component/l2tp/ppp/ipcp.go), so the obligation to request it separately per direction never binds. | Each end of the link must separately request this option if bi-directional compression is desired. |
| `4.1:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Comp-Slot-Id semantics bind only a speaker that negotiated the optional IP-Compression-Protocol option, and Section 3.2 makes it optional: "By default, compression is not enabled." Ze negotiates only IPCP options 3, 129 and 131 and Configure-Rejects Type 2, so it never carries a Comp-Slot-Id field. | 0 The slot identifier must not be compressed. |
| `4.1:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | The C bit and slot identifier rules bind a Van Jacobson compressor, a feature Section 3.2 makes optional: "By default, compression is not enabled." Ze emits no compressed TCP frames (PPP protocol 002d) because it never negotiates the option. | All compressed TCP packets must set the C bit in every change mask, and must include the slot identifier. |

## Superseded

No document obsoletes RFC 1332, so its obligations are stated where they were written.
