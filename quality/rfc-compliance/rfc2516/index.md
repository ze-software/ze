# RFC 2516 - A Method for Transmitting PPP Over Ethernet (PPPoE)

PPPoE. Every requirement listed for RFC 2516, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 26 | of 37 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 6 | of 37 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 37 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 37 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 70 | 70 of 84 tests linked to requirements, 0 without a fault test and 3 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 37 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 37 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 37 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 1 | of 37 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 37 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 37 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 37 | of 40 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 37 mandatory requirements: they add up to 37. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | PPPoE |
| Automatic checks | Included |
| Requirements | 40 |
| Mandatory requirements (MUST level) | 37 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 84 |
| Tests linked to requirements | 84 |
| Recorded audit verdicts | 31 |
| Fault test records | 73 |
| Summary | `rfc/short/rfc2516.md` |
| Requirement table | `rfc/requirements/rfc2516.md` |
| RFC text | `rfc/full/rfc2516.txt` |

## Automatic checks

Included in automatic checks: PPPoE access-concentrator and client roles: discovery framing and tags, session identity, kernel transport setup, PADT lifetime, and transport-specific LCP negotiation. Requirement bindings come from the tagged tests; the checklist records remaining kernel-path evidence and conditional feature decisions.

## Published support status

**Area:** PPPoE

**Test verification:** 25 of 37 MUST proven

**Evidence:** unit 25

**Unmet:** RFC2516-3-1, RFC2516-x-11, RFC2516-x-14, RFC2516-x-15


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 26 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 11 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **37** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (26):** [`RFC2516-x-1`](#rfc2516-x-1), [`RFC2516-x-2`](#rfc2516-x-2), [`RFC2516-x-4`](#rfc2516-x-4), [`RFC2516-5.2-1`](#rfc2516-5.2-1), [`RFC2516-5.3-1`](#rfc2516-5.3-1), [`RFC2516-x-5`](#rfc2516-x-5), [`RFC2516-5.2-2`](#rfc2516-5.2-2), [`RFC2516-5.2-4`](#rfc2516-5.2-4), [`RFC2516-5.4-1`](#rfc2516-5.4-1), [`RFC2516-5.4-2`](#rfc2516-5.4-2), [`RFC2516-x-6`](#rfc2516-x-6), [`RFC2516-x-7`](#rfc2516-x-7), [`RFC2516-x-9`](#rfc2516-x-9), [`RFC2516-7-1`](#rfc2516-7-1), [`RFC2516-4-1`](#rfc2516-4-1), [`RFC2516-5.1-4`](#rfc2516-5.1-4), [`RFC2516-5.1-5`](#rfc2516-5.1-5), [`RFC2516-5.2-6`](#rfc2516-5.2-6), [`RFC2516-5.3-5`](#rfc2516-5.3-5), [`RFC2516-5.4-3`](#rfc2516-5.4-3), [`RFC2516-5.5-3`](#rfc2516-5.5-3), [`RFC2516-5.5-4`](#rfc2516-5.5-4), [`RFC2516-7-2`](#rfc2516-7-2), [`RFC2516-7-3`](#rfc2516-7-3), [`RFC2516-7-4`](#rfc2516-7-4), [`RFC2516-x-10`](#rfc2516-x-10)

**Annotated (including scoped evidence) (11):** [`RFC2516-5.1-1`](#rfc2516-5.1-1), [`RFC2516-5.1-2`](#rfc2516-5.1-2), [`RFC2516-5.3-2`](#rfc2516-5.3-2), [`RFC2516-x-8`](#rfc2516-x-8), [`RFC2516-3-1`](#rfc2516-3-1), [`RFC2516-6-1`](#rfc2516-6-1), [`RFC2516-x-11`](#rfc2516-x-11), [`RFC2516-x-12`](#rfc2516-x-12), [`RFC2516-x-13`](#rfc2516-x-13), [`RFC2516-x-14`](#rfc2516-x-14), [`RFC2516-x-15`](#rfc2516-x-15)

## Figures

- **MUST bar**: Ze verified 25, Tests awaiting verification 7, OS testing gap 1, Not built 4
- **Overview bar**: Ze MUST verified 25, Tests awaiting verification 7, OS testing gap 1, Not assessed 3, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 37 |
| MUST rows kept (the MUST bar) | 37 |
| Ze verified | 25 |
| OS verified | 0 |
| Tests awaiting verification | 7 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2516.json](../../../data/rfc/rfc2516.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2516-x-1` | The VER field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L454). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L306) |
| `RFC2516-x-2` | The TYPE field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L455). **negative:** `unit/verify` [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L474) |
| `RFC2516-x-4` | This TAG indicates that there are no further TAGs in the list. The TAG_LENGTH of this TAG MUST always be zero. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L365). **positive:** `unit/verify` [`TestRFC2516NoSentFrameCarriesEndOfList`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L348). **negative:** `unit/verify` [`TestRFC2516NoSentFrameCarriesEndOfList`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L349) |
| `RFC2516-5.2-1` | If the Access Concentrator receives this TAG, it MUST include the TAG unmodified in the associated PADO or PADS response. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L129). **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L178). **positive:** `unit/verify` [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L77). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L578) |
| `RFC2516-5.3-1` | If a Host receives this TAG, it MUST return the TAG unmodified in the following PADR. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L508). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L549) |
| `RFC2516-x-5` | If either the Host or Access Concentrator receives this TAG they MUST include it unmodified in any discovery packet they send as a response. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2516RefusingPADSEchoesRelaySessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L271). **positive:** `unit/verify` [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L615). **negative:** `unit/verify` [`TestRFC2516RefusingPADSEchoesRelaySessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L272). **negative:** `unit/verify` [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L658) |
| `RFC2516-5.1-1` | The PADI packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L485). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADI always writes exactly one Service-Name tag (discovery.go:307). The AC does not enforce the count on receive (MatchServiceName tolerates zero or many, discovery.go:220): this is a deliberate choice, not an omission, per the owner decision of 2026-09-08 recorded in docs/architecture/l2tp/bng-5-pppoe.md ("Ze refuses a PADR with no Service-Name tag and serves a PADI with none"), because Section 5.1 binds the sending host and neither accel-ppp nor FreeBSD enforces this count on receipt either |
| `RFC2516-5.1-2` | The Host sends the PADI packet with the DESTINATION_ADDR set to the broadcast address. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L486). **negative:** no negative test. **{single-polarity}:** emit-side: BuildPADI addresses the PADI to the Ethernet broadcast MAC (discovery.go:305); the receive-side non-broadcast-PADI discard is not enforced (handlePADI checks only SID, server.go:53) |
| `RFC2516-5.2-2` | The PADO packet MUST contain one AC-Name TAG containing the Access Concentrator's name, a Service-Name TAG identical to the one in the PADI, and any number of other Service-Name TAGs indicating other services that the Access Concentrator offers. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L725). **positive:** `unit/verify` [`TestRFC2516PADOCarriesACNameAndServiceNames`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L240). **negative:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L726) |
| `RFC2516-5.2-4` | If the Access Concentrator can not serve the PADI it MUST NOT respond with a PADO. (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L201). **negative:** `unit/verify` [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L200) |
| `RFC2516-5.3-2` | The PADR packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L509). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADR writes exactly one Service-Name tag (discovery.go:322). The AC's requireServiceNameTag (server.go) now refuses a PADR carrying zero Service-Name tags on receipt, but that enforces PRESENCE, not COUNT: a PADR carrying two Service-Name tags is accepted, first-wins (TestDuplicateServiceNameTagTakesTheFirst), so there is still no receive-side enforcement of "exactly one, not more" to give a negative for this requirement |
| `RFC2516-5.4-1` | The PADS packet contains exactly one TAG of TAG_TYPE Service-Name, indicating the service under which Access Concentrator has accepted the PPPoE session, and any number of other TAG types. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L179). **negative:** `unit/verify` [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L782) |
| `RFC2516-5.4-2` | If the Access Concentrator does not like the Service-Name in the PADR, then it MUST reply with a PADS containing a TAG of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this case the SESSION_ID MUST be set to 0x0000. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_server_test.go#L165). **positive:** `unit/verify` [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L199). **negative:** `unit/verify` [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L76) |
| `RFC2516-x-6` | For Discovery packets, the value is either a unicast or broadcast address as defined in the Discovery section. For PPP session traffic, this field MUST contain the peer's unicast address as determined from the Discovery stage. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L75). **negative:** `unit/verify` [`TestRFC2516SessionNeverBoundToANonUnicastPeer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L161) |
| `RFC2516-x-7` | The SOURCE_ADDR field MUST contains the Ethernet MAC address of the source device. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L51). **positive:** `unit/verify` [`TestRFC2516BuildersWriteTheSendersMAC`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L303). **negative:** `unit/verify` [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L332). **negative:** `unit/verify` [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L342) |
| `RFC2516-x-8` | If a discovery packet is received with a TAG of unknown TAG_TYPE, the TAG MUST be ignored unless otherwise specified in this document. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L678). **negative:** no negative test. **{single-polarity}:** tolerate requirement: parseTags stores unknown tag types generically and never errors on them (discovery.go:154); rejecting an unknown tag would itself violate the MUST NOT, so no negative exists |
| `RFC2516-x-9` | The Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a larger size than 1492. (§7) | MUST NOT | 7 | **positive:** `unit/verify` [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117). **positive:** `unit/verify` [`TestRFC2516AccessConcentratorNegotiatesMRUAtThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go#L154). **positive:** `unit/verify` [`TestRFC2516ClientNegotiatesMRUAtThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_mru_ceiling_test.go#L120). **positive:** `unit/verify` [`TestRFC2516HostSessionMTUDefaultsToThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_session_mtu_test.go#L34). **negative:** `unit/verify` [`TestRFC2516AccessConcentratorRefusesAnMRUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go#L179). **negative:** `unit/verify` [`TestRFC2516ClientRefusesAnMRUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_mru_ceiling_test.go#L153). **negative:** `unit/verify` [`TestRFC2516HostRefusesASessionMTUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_session_mtu_test.go#L96) |
| `RFC2516-7-1` | The SESSION_ID MUST NOT change for that PPPoE session and MUST be the value assigned in the Discovery stage. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L74). **negative:** `unit/verify` [`TestRFC2516PADRReplayKeepsTheSessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L132) |
| `RFC2516-3-1` | Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** both roles allocate the interface (server.go::handlePADR, pppoeclient/dialer.go::Dial) but no tagged test observes the ppp unit, which only a QEMU run can; plan/pre-release/spec-pppoe-virtual-interface-proof.md |
| `RFC2516-4-1` | A value of 0xffff is reserved for future use and MUST NOT be used (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC2516AllocSIDReturnsUsableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L52). **negative:** `unit/verify` [`TestRFC2516SessionID0xffffIsNeverAllocated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L64) |
| `RFC2516-5.1-4` | The CODE field is set to 0x09 and the SESSION_ID MUST be set to 0x0000. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC2516PADISessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L85). **negative:** `unit/verify` [`TestRFC2516PADIWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L101) |
| `RFC2516-5.1-5` | An entire PADI packet (including the PPPoE header) MUST NOT exceed 1484 octets so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG. (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L17). **negative:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L18) |
| `RFC2516-5.2-6` | The CODE field is set to 0x07 and the SESSION_ID MUST be set to 0x0000. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC2516PADOSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L121). **negative:** `unit/verify` [`TestRFC2516PADOWithNonZeroSessionIDIsNotAnOffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_discovery_test.go#L37) |
| `RFC2516-5.3-5` | The CODE field is set to 0x19 and the SESSION_ID MUST be set to 0x0000. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC2516PADRSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L138). **negative:** `unit/verify` [`TestRFC2516PADRWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L155) |
| `RFC2516-5.4-3` | The CODE field is set to 0x65 and the SESSION_ID MUST be set to the unique value generated for this PPPoE session. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L73). **positive:** `unit/verify` [`TestRFC2516PADSCarriesTheSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L184). **negative:** `unit/verify` [`TestRFC2516PADSWithZeroSessionIDIsARefusal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_discovery_test.go#L61) |
| `RFC2516-5.5-3` | The DESTINATION_ADDR field is a unicast Ethernet address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to indicate which session is to be terminated. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L225). **positive:** `unit/verify` [`TestRFC2516HostPADTNamesItsAccessConcentratorAndSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L142). **positive:** `unit/verify` [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L225). **negative:** `unit/verify` [`TestRFC2516HostPADTNeverAddressesAGroupOrAForeignSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L163). **negative:** `unit/verify` [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L318) |
| `RFC2516-5.5-4` | Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5) | MUST NOT | 5.5 | **positive:** `unit/verify` [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L53). **positive:** `unit/verify` [`TestRFC2516ACSentPADTClosesTransportBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L87). **positive:** `unit/verify` [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L29). **positive:** `unit/verify` [`TestRFC2516ClientSentPADTStopsPPPBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L131). **negative:** `unit/verify` [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L54). **negative:** `unit/verify` [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L30) |
| `RFC2516-6-1` | The PPPoE CODE MUST be set to 0x00. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux pppoe (PX_PROTO_OE, drivers/net/ppp/pppoe.c), linux:CONFIG_PPPOE; internal/component/l2tp/pppoe/kernel_linux.go::pppoeCreate connects the AF_PPPOX socket to the session ID, the peer MAC and the device, and the module writes the CODE byte of every session frame itself, so no value Ze writes decides this field |
| `RFC2516-7-2` | An implementation MUST NOT request any of the following options, and MUST reject a request for such an option: Field Check Sequence (FCS) Alternatives, Address-and-Control-Field-Compression (ACFC), Asynchronous-Control-Character-Map (ACCM) (§7) | MUST NOT | 7 | **positive:** `unit/verify` [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L11). **positive:** `unit/verify` [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L101). **negative:** `unit/verify` [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L12). **negative:** `unit/verify` [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L102) |
| `RFC2516-7-3` | When LCP terminates, the Host and Access concentrator MUST stop using that PPPoE session. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC2516HostStopsUsingTheSessionWhenLCPTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L263). **positive:** `unit/verify` [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L227). **negative:** `unit/verify` [`TestRFC2516HostKeepsTheSessionUntilLCPTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L298). **negative:** `unit/verify` [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L320) |
| `RFC2516-7-4` | If the Host wishes to start another PPP session, it MUST return to the PPPoE Discovery stage. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc2516_pppoe_client_test.go#L59). **negative:** `unit/verify` [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc2516_pppoe_client_test.go#L60) |
| `RFC2516-x-10` | All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L19). **negative:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L20) |
| `RFC2516-x-11` | A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze fills no PPPoE relay agent; plan/spec-pppoe-relay-agent.md |
| `RFC2516-x-12` | This TAG (typically with a zero-length data section) indicates that for one reason or another, the requested Service-Name request could not be honored. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains why the request was denied. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L356). **negative:** no negative test. **{single-polarity}:** discovery.go::BuildPADSError writes the tag with no data, so the conditional never binds and no violating input exists |
| `RFC2516-x-13` | It MAY be included in PADS packets. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains the nature of the error. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L357). **negative:** no negative test. **{single-polarity}:** discovery.go::BuildPADSError writes the tag with no data, so the conditional never binds and no violating input exists |
| `RFC2516-x-14` | It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| `RFC2516-x-15` | If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| `RFC2516-5.2-5` | The Access Concentrator MAY include this TAG in a PADO packet. (Appendix A, §11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-3` | The Host MAY include a Host-Uniq TAG in a PADI or PADR. (Appendix A, §11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-1` | This packet may be sent anytime after a session is established to indicate that a PPPoE session has been terminated. It may be sent by either the Host or the Access Concentrator. (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2516-3-1`](#rfc2516-3-1) Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | {gap}, no test | both roles allocate the interface (server.go::handlePADR, pppoeclient/dialer.go::Dial) but no tagged test observes the ppp unit, which only a QEMU run can; plan/pre-release/spec-pppoe-virtual-interface-proof.md |
| [`RFC2516-6-1`](#rfc2516-6-1) The PPPoE CODE MUST be set to 0x00. (§6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux pppoe (PX_PROTO_OE, drivers/net/ppp/pppoe.c), linux:CONFIG_PPPOE; internal/component/l2tp/pppoe/kernel_linux.go::pppoeCreate connects the AF_PPPOX socket to the session ID, the peer MAC and the device, and the module writes the CODE byte of every session frame itself, so no value Ze writes decides this field |
| [`RFC2516-x-11`](#rfc2516-x-11) A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11) | {gap}, no test | Ze fills no PPPoE relay agent; plan/spec-pppoe-relay-agent.md |
| [`RFC2516-x-14`](#rfc2516-x-14) It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11) | {gap}, no test | Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| [`RFC2516-x-15`](#rfc2516-x-15) If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11) | {gap}, no test | Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2516-x-1`](#rfc2516-x-1)

The VER field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildFrameVerType asserts BuildPADI writes VER=1; TestParseBadVersion shows a VER=2 frame is refused

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L306) | unit/verify | not verified |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L454) | unit/verify | not verified |

### [`RFC2516-x-2`](#rfc2516-x-2)

The TYPE field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildFrameVerType asserts BuildPADI writes TYPE=1; TestParseBadType shows a TYPE=2 frame is refused

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L474) | unit/verify | not verified |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L455) | unit/verify | not verified |

### [`RFC2516-x-4`](#rfc2516-x-4)

This TAG indicates that there are no further TAGs in the list. The TAG_LENGTH of this TAG MUST always be zero. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516NoSentFrameCarriesEndOfList, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516NoSentFrameCarriesEndOfList#2 moved. first sentence: TestParseEndOfListTag. TAG_LENGTH MUST: Ze builds no End-Of-List; TestRFC2516NoSentFrameCarriesEndOfList walks the raw TLVs of the PADO and PADS replies and shows a received End-Of-List of TAG_LENGTH 4 is never copied out. Not observed: BuildPADR and BuildPADSError, which copy tags by type the same way (AddTagCopy of FindTag) and so cannot carry one either. Stale single-polarity marker removed

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516NoSentFrameCarriesEndOfList`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L349) | unit/verify | revert, verified |
| positive | [`TestRFC2516NoSentFrameCarriesEndOfList`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L348) | unit/verify | revert, verified |
| positive | [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L365) | unit/verify | revert, verified |

### [`RFC2516-5.2-1`](#rfc2516-5.2-1)

If the Access Concentrator receives this TAG, it MUST include the TAG unmodified in the associated PADO or PADS response. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516AdmittedPADSCarriesTheNewSessionsID moved. PADO echo byte-compared by TestBuildPADO; PADS echo now byte-compared both in TestBuildPADS and through handlePADR for two hosts with distinct Host-Uniq values (TestRFC2516AdmittedPADSCarriesTheNewSessionsID); TestBuildNoHostUniqEcho proves no Host-Uniq is invented. Observed-red records on BuildPADS for both new/changed units

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L578) | unit/verify | revert, verified |
| positive | [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L178) | unit/verify | revert, verified |

### [`RFC2516-5.3-1`](#rfc2516-5.3-1)

If a Host receives this TAG, it MUST return the TAG unmodified in the following PADR. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildPADREchoesTags compares the echoed AC-Cookie bytes; TestBuildPADRNoOptionalTags shows none is invented

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L549) | unit/verify | not verified |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L508) | unit/verify | not verified |

### [`RFC2516-x-5`](#rfc2516-x-5)

If either the Host or Access Concentrator receives this TAG they MUST include it unmodified in any discovery packet they send as a response. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516RefusingPADSEchoesRelaySessionID, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516RefusingPADSEchoesRelaySessionID#2 moved. the refusing PADS (BuildPADSError via handlePADR) and the admitting PADS now echo the Relay-Session-Id byte-for-byte, and a refusing PADS for a PADR without one invents none; PADO, PADR and PADS builder echo and no-echo units unchanged. Records red on BuildPADSError

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516RefusingPADSEchoesRelaySessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L272) | unit/verify | revert, verified |
| negative | [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L658) | unit/verify | revert, verified |
| positive | [`TestRFC2516RefusingPADSEchoesRelaySessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L271) | unit/verify | revert, verified |
| positive | [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L615) | unit/verify | revert, verified |

### [`RFC2516-5.1-1`](#rfc2516-5.1-1)

The PADI packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADIDiscovery counts exactly one Service-Name tag

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L485) | unit/verify | not verified |

### [`RFC2516-5.1-2`](#rfc2516-5.1-2)

The Host sends the PADI packet with the DESTINATION_ADDR set to the broadcast address. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADIDiscovery asserts the PADI DstMAC is the broadcast address

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L486) | unit/verify | not verified |

### [`RFC2516-5.2-2`](#rfc2516-5.2-2)

The PADO packet MUST contain one AC-Name TAG containing the Access Concentrator's name, a Service-Name TAG identical to the one in the PADI, and any number of other Service-Name TAGs indicating other services that the Access Concentrator offers. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516PADOCarriesACNameAndServiceNames moved. TestRFC2516PADOCarriesACNameAndServiceNames drives handlePADI and asserts exactly one AC-Name equal to the configured name, the first Service-Name identical to the PADI's, then the other offered name in order; TestPADOAlwaysCarriesServiceName pins the Service-Name list and the zero-length edge. Records observed red on BuildPADO

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L726) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADOCarriesACNameAndServiceNames`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L240) | unit/verify | revert, verified |
| positive | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L725) | unit/verify | revert, verified |

### [`RFC2516-5.2-4`](#rfc2516-5.2-4)

If the Access Concentrator can not serve the PADI it MUST NOT respond with a PADO. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516UnservedServiceNameIsRefused, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516UnservedServiceNameIsRefused#2 moved. TestRFC2516UnservedServiceNameIsRefused drives handlePADI: a PADI naming an unserved Service-Name draws no frame, the same PADI naming a served one draws exactly one PADO, so the silence is specific to the unserved service. Both polarities recorded red on handlePADI

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L200) | unit/verify | revert, verified |
| positive | [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L201) | unit/verify | revert, verified |

### [`RFC2516-5.3-2`](#rfc2516-5.3-2)

The PADR packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADREchoesTags counts exactly one Service-Name tag

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L509) | unit/verify | not verified |

### [`RFC2516-5.4-1`](#rfc2516-5.4-1)

The PADS packet contains exactly one TAG of TAG_TYPE Service-Name, indicating the service under which Access Concentrator has accepted the PPPoE session, and any number of other TAG types. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. re-read after TestBuildPADS changed (only its Host-Uniq check was tightened to a byte compare): TestBuildPADS and TestPADSAlwaysCarriesServiceName both count exactly one Service-Name tag in the PADS, including for a PADR carrying none

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L782) | unit/verify | revert, verified |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L179) | unit/verify | not verified |

### [`RFC2516-5.4-2`](#rfc2516-5.4-2)

If the Access Concentrator does not like the Service-Name in the PADR, then it MUST reply with a PADS containing a TAG of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this case the SESSION_ID MUST be set to 0x0000. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516AdmittedPADSCarriesTheNewSessionsID, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516UnservedServiceNameIsRefused, internal/component/l2tp/pppoe/rfc2516_server_test.go::TestPADRWithoutServiceNameGetsServiceNameError moved. positive: a PADR with a PRESENT unserved Service-Name through handlePADR draws one PADS with Service-Name-Error and SESSION_ID 0x0000, no session, no socket (TestRFC2516UnservedServiceNameIsRefused); the absent-tag branch stays covered by TestPADRWithoutServiceNameGetsServiceNameError. negative: a served name draws a non-zero SID and no Service-Name-Error. All three recorded red on handlePADR

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC2516UnservedServiceNameIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_server_test.go#L165) | unit/verify | revert, verified |

### [`RFC2516-x-6`](#rfc2516-x-6)

For Discovery packets, the value is either a unicast or broadcast address as defined in the Discovery section. For PPP session traffic, this field MUST contain the peer's unicast address as determined from the Discovery stage. (§4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516AdmittedPADSCarriesTheNewSessionsID, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516SessionNeverBoundToANonUnicastPeer moved. session-stage addressing is set by connecting the pppox socket: the positive asserts each admitted session's socket is bound to its PADR's unicast source MAC; the negative shows a broadcast- and a multicast-sourced PADR are refused by ParseDiscovery before HandleDiscovery and bind nothing, while a unicast source binds itself. The negative mirrors Subsystem.discoveryReader's parse-then-dispatch inline (read at subsystem.go, it matches), so a change to the reader itself would not turn it red. Records red on createTransport and ParseDiscovery

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516SessionNeverBoundToANonUnicastPeer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L75) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |

### [`RFC2516-x-7`](#rfc2516-x-7)

The SOURCE_ADDR field MUST contains the Ethernet MAC address of the source device. (§4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516BuildersWriteTheSendersMAC moved. TestRFC2516BuildersWriteTheSendersMAC asserts PADI, PADR, PADO, PADS, the refusing PADS and PADT write the sender MAC they are given in SOURCE_ADDR; every production call site passes the device's own MAC (s.hwAddr in server.go, hwaddr/srcMAC in pppoeclient/dialer.go), read, not asserted. Receive side refuses broadcast and multicast sources. Record red on NewBuilder

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L332) | unit/verify | revert, verified |
| negative | [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L342) | unit/verify | revert, verified |
| positive | [`TestRFC2516BuildersWriteTheSendersMAC`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L303) | unit/verify | revert, verified |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L51) | unit/verify | revert, verified |

### [`RFC2516-x-8`](#rfc2516-x-8)

If a discovery packet is received with a TAG of unknown TAG_TYPE, the TAG MUST be ignored unless otherwise specified in this document. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestUnknownTagIgnored parses a frame with an unknown tag without error and keeps known tags findable

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L678) | unit/verify | not verified |

### [`RFC2516-x-9`](#rfc2516-x-9)

The Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a larger size than 1492. (§7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go::TestRFC2516AccessConcentratorNegotiatesMRUAtThePPPoECeiling, internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go::TestRFC2516AccessConcentratorRefusesAnMRUAboveThePPPoECeiling moved. Independent rejudgment 2026-11-15 against RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a larger size than 1492." Read all seven tagged units and their production paths. Host-default accepts omitted MTU as 1492, preserves 1492 and 1480, and reads an actual first client Configure-Request naming 1492; Host-negative refuses 1493/1500/9000 with the specific range error and verifies Dial rejects 1500 before discovery/interface resolution. The client positive negotiates both directions at 1492 and opens; distinct negative requests a peer MRU 1500, requires a Nak containing only 1492 and then opens after correction, and separately sends a correlated 1500 Nak suggestion which must not raise the next client MRU above1492. The AC pair drives handlePADR into the production PPP driver and observes its own MRU at or below1492, Ack of a canonical1492 request and Nak of a distinct1500 request to1492, preserving Identifiers. TestLCPConfigRequestMRU independently checks exact encoder output1492. Read pppoeSessionMTU and Dial admission, clientLCPPolicy/negotiateLCP and server MaxMRU/transmitMRU caps. Positive and negative buffers differ only in the targeted MRU value and valid correlation; neither relies on a packet-shape refusal. New explicit LCP/Max-Failure fixtures leave the scoped MRU assertions unchanged. Main-thread current native records supply observed-red evidence; no new behavior is claimed from panic-style producer-disable observations.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516AccessConcentratorRefusesAnMRUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go#L179) | unit/verify | revert, verified |
| negative | [`TestRFC2516HostRefusesASessionMTUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_session_mtu_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC2516ClientRefusesAnMRUAboveThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_mru_ceiling_test.go#L153) | unit/verify | revert, verified |
| positive | [`TestRFC2516AccessConcentratorNegotiatesMRUAtThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_ac_mru_ceiling_linux_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC2516HostSessionMTUDefaultsToThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_session_mtu_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC2516ClientNegotiatesMRUAtThePPPoECeiling`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_mru_ceiling_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117) | unit/verify | revert, verified |

### [`RFC2516-7-1`](#rfc2516-7-1)

The SESSION_ID MUST NOT change for that PPPoE session and MUST be the value assigned in the Discovery stage. (§6)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516AdmittedPADSCarriesTheNewSessionsID, internal/component/l2tp/pppoe/rfc2516_admission_test.go::TestRFC2516PADRReplayKeepsTheSessionID moved. Ze's share of Section 6 is binding the session socket: TestRFC2516AdmittedPADSCarriesTheNewSessionsID asserts the socket is bound once to the SID the PADS assigned (createTransport seam, production path unchanged), and TestRFC2516PADRReplayKeepsTheSessionID shows a retransmitted PADR keeps the SID, binds no second socket, adds no session. The kernel pppox socket keeps the SID fixed after connect. The PADT-MAC unit no longer carries the tag. Records red on createTransport and matchLiveCookie

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516PADRReplayKeepsTheSessionID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L74) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |

### [`RFC2516-3-1`](#rfc2516-3-1)

Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2516-3-1, so there is no linked test.

### [`RFC2516-4-1`](#rfc2516-4-1)

A value of 0xffff is reserved for future use and MUST NOT be used (§4)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516AllocSIDReturnsUsableValue, internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516SessionID0xffffIsNeverAllocated moved. TestRFC2516SessionID0xffffIsNeverAllocated exhausts the table and shows 0xffff is never handed out, even after freeSID

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516SessionID0xffffIsNeverAllocated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L64) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC2516AllocSIDReturnsUsableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L52) | unit/verify | revert, verified |

### [`RFC2516-5.1-4`](#rfc2516-5.1-4)

The CODE field is set to 0x09 and the SESSION_ID MUST be set to 0x0000. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADISessionIDIsZero, internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADIWithNonZeroSessionIDIsDropped moved. TestRFC2516PADISessionIDIsZero asserts CODE 0x09 and SESSION_ID 0; the negative shows handlePADI drops a non-zero SESSION_ID PADI

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516PADIWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADISessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L85) | unit/verify | revert, verified |

### [`RFC2516-5.1-5`](#rfc2516-5.1-5)

An entire PADI packet (including the PPPoE header) MUST NOT exceed 1484 octets so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestPADINeverExceeds1484Octets accepts exactly 1484 octets and refuses 1485

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L17) | unit/verify | revert, verified |

### [`RFC2516-5.2-6`](#rfc2516-5.2-6)

The CODE field is set to 0x07 and the SESSION_ID MUST be set to 0x0000. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADOSessionIDIsZero moved. TestRFC2516PADOSessionIDIsZero asserts CODE 0x07 and SESSION_ID 0; the client refuses a PADO with a non-zero SESSION_ID

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516PADOWithNonZeroSessionIDIsNotAnOffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_discovery_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADOSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L121) | unit/verify | revert, verified |

### [`RFC2516-5.3-5`](#rfc2516-5.3-5)

The CODE field is set to 0x19 and the SESSION_ID MUST be set to 0x0000. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADRSessionIDIsZero, internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADRWithNonZeroSessionIDIsDropped moved. TestRFC2516PADRSessionIDIsZero asserts CODE 0x19 and SESSION_ID 0; handlePADR drops a non-zero SESSION_ID PADR

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516PADRWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADRSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L138) | unit/verify | revert, verified |

### [`RFC2516-5.4-3`](#rfc2516-5.4-3)

The CODE field is set to 0x65 and the SESSION_ID MUST be set to the unique value generated for this PPPoE session. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516PADSCarriesTheSessionsID moved. the fresh-admission PADS is now observed: two hosts each get CODE 0x65, a non-zero SESSION_ID equal to the SID the table holds for that host, and the two SIDs differ (TestRFC2516AdmittedPADSCarriesTheNewSessionsID); the replay path stays covered; the client refuses a zero-SID PADS (pppoeclient negative). Records observed red on handlePADR and tryReadPADS

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516PADSWithZeroSessionIDIsARefusal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_discovery_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRFC2516AdmittedPADSCarriesTheNewSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_admission_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADSCarriesTheSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L184) | unit/verify | revert, verified |

### [`RFC2516-5.5-3`](#rfc2516-5.5-3)

The DESTINATION_ADDR field is a unicast Ethernet address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to indicate which session is to be terminated. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ACTUAL LIVE SOURCE VERDICT, no native stamp or runtime claim. Read full RFC2516 section5.5 and exact summary requirement: destination is unicast Ethernet, CODE0xa7 and SESSION_ID identifies the terminated session. Census five tagged units across three carriers: AC TestRFC2516SessionDownSendsPADTForThatSession and TestRFC2516UnknownSessionIDTerminatesNothing, external pppoe TestBuildPADT, host TestRFC2516HostPADTNamesItsAccessConcentratorAndSession and TestRFC2516HostPADTNeverAddressesAGroupOrAForeignSession. Read full units and actual NewBuilder/BuildPADT/Finish, ParseDiscovery, Host tryReadPADO/PADS/sendPADT, discovery seams, InterfaceServer matching teardown and genuine Driver consumer. BuildPADT positive asserts exact CODE/SID/destination; AC positive preserves exact single PADT code/SID/unicast MAC after actual original worker termination, native close before PADT and disjoint worker survival. Correctly addressed absent-SID PADT reaches unknown-session rule and must preserve actual admitted worker/native/Echo. Host positive discovers real offered AC MAC and assigned0x2345, then directly inspects raw destination unicast bit, literal0xa7 and exact original SID. Negative changes only valid PADO source to broadcast/multicast, and offers complete foreignAC PADS0x3456 before correctAC0x2345; neither may supply the Host termination destination/SID. Actual ParseDiscovery group-source rejection and tryReadPADS source correlation are reached with otherwise valid packets, excluding neighbouring-rule ambiguity. These tests would fail on wrong code/address/session selection, not merely absence of panic. Final AC/host changed declarations match independently judged caa945ad/2291368d proposals; source pins are in7-3 note. The missing host pipe helper was corrected before final comparison. MAIN owes named canonical GREEN and meaningful semantic RED proof refreshes, including wrong PADT SID and absent-SID selects-live cuts from15edda33; no whole-producer panic substitutes for these assertions. Full source judgment covers all three fields and both production roles; supported audit rejudge stamping remains MAIN work after final formatting/source pin confirmation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L318) | unit/verify | mutant, verified |
| negative | [`TestRFC2516HostPADTNeverAddressesAGroupOrAForeignSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_discovery_test.go#L225) | unit/verify | revert, verified |
| positive | [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L225) | unit/verify | mutant, verified |
| positive | [`TestRFC2516HostPADTNamesItsAccessConcentratorAndSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L142) | unit/verify | revert, verified |

### [`RFC2516-5.5-4`](#rfc2516-5.5-4)

Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L54) | unit/verify | revert, verified |
| negative | [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestRFC2516ACSentPADTClosesTransportBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC2516ClientSentPADTStopsPPPBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go#L131) | unit/verify | revert, verified |

### [`RFC2516-6-1`](#rfc2516-6-1)

The PPPoE CODE MUST be set to 0x00. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2516-6-1, so there is no linked test.

### [`RFC2516-7-2`](#rfc2516-7-2)

An implementation MUST NOT request any of the following options, and MUST reject a request for such an option: Field Check Sequence (FCS) Alternatives, Address-and-Control-Field-Compression (ACFC), Asynchronous-Control-Character-Map (ACCM) (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516ACRejectsForbiddenLCPOptions and TestRFC2516ClientRejectsForbiddenLCPOptions assert Configure-Reject of ACCM, ACFC and FCS Alternatives verbatim, that neither role requests them, and Ack for MRU/PFC

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L12) | unit/verify | revert, verified |
| negative | [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L11) | unit/verify | revert, verified |
| positive | [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L101) | unit/verify | revert, verified |

### [`RFC2516-7-3`](#rfc2516-7-3)

When LCP terminates, the Host and Access concentrator MUST stop using that PPPoE session. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ACTUAL LIVE SOURCE VERDICT: all four tagged units, both new AC helpers and their type, host networkPhase helper and hostChannel type now match exact frozen caa945ad and2291368d proposals in normalized source comparison. The previously missing networkPhase pipe implementation is corrected and independently reread. Final AC raw source pin ee2cd48656adc3e8e64e342c6f58450af6010e11a866719ee9b25cd6c876a96a and hostc4ad462a2bc64bb6dd81906a8295da268230e4e50104a1a03e502a069c3ad404 are provenance only, not native audit fingerprints. Complete four-tag census confirmed. Source judgment is ready, with actual validation and supported audit stamping owned by MAIN. Read the complete RFC2516 section7 obligation that Host and Access Concentrator stop using that PPPoE session when LCP terminates, its following Discovery obligation, section5.5 and exact summary row. Census is four units in two carriers: pppoe TestRFC2516SessionDownSendsPADTForThatSession and TestRFC2516UnknownSessionIDTerminatesNothing; pppoeclient TestRFC2516HostStopsUsingTheSessionWhenLCPTerminates and TestRFC2516HostKeepsTheSessionUntilLCPTerminates. A authored none and separately APPROVE(a) each after full HEAD/current/proposed source and actual producer/helper reads. AC positive preserves exact PADT count/code/SID/unicast destination/removal, but now actual original worker TermReq77 produces genuine LCPDown, acknowledged through the actual consumer, exact Ack77, genuine Down and consumer cleanup. Its original native pipe writer must be closed before PADT while the reader remains open; the worker must retire/release; another admitted actual worker must stay native-writable and answer Echo, including after old Down replay. AC negative retains absent-SID PADT and absent-session Down and all old no-send/survival assertions; correctly addressed PADT reaches absent-SID selection and real worker/native/Echo survival excludes unconditional teardown. Host positive retains exact Ack5, one logical close, Done and all later guarded-write refusal checks; actual production callback must close original native writer once and direct write must return os.ErrClosed with reader alive. The logical channel deliberately still accepts writes after Close so original guard checks remain discriminating. Host negative retains Echo7/strayAck8/Echo9/TermReq10 sequence, exact replies and lifecycle assertions, adding native survival before actual termination and exact once-close afterward. Actual producers read: Driver/Attempt fixture and native transport ownership, Subsystem LCPDown/Down matching consumers, InterfaceServer finishSessionDown/closeAdmitted/CloseTransport, custom-client keepaliveLoop/superviseNetworkPhase/sessionLink.Close. These are native-owner boundary tests, not a Linux PPP forwarding claim. No renewed PPP-session admission is exercised, so the adjacent return-to-Discovery obligation remains its separate requirement scope. No runtime, observed countercut, compile result or native audit stamp is claimed. MAIN owes focused four-unit canonical GREEN and named behavior-only producer breaks, then supported rejudge stamp.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L320) | unit/verify | mutant, verified |
| negative | [`TestRFC2516HostKeepsTheSessionUntilLCPTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L298) | unit/verify | mutant, verified |
| positive | [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L227) | unit/verify | mutant, verified |
| positive | [`TestRFC2516HostStopsUsingTheSessionWhenLCPTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc2516_host_termination_test.go#L263) | unit/verify | mutant, verified |

### [`RFC2516-7-4`](#rfc2516-7-4)

If the Host wishes to start another PPP session, it MUST return to the PPPoE Discovery stage. (§7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/iface/rfc2516_pppoe_client_test.go::TestRFC2516HostReturnsToDiscoveryForTheNextSession, internal/component/iface/rfc2516_pppoe_client_test.go::TestRFC2516HostReturnsToDiscoveryForTheNextSession#2 moved. Rejudge 2026-11-08: TestRFC2516HostReturnsToDiscoveryForTheNextSession changed only by passing Timing: testPPPoETiming (ReconnectDelayMin 1s) now that keepalive and reconnect timing come from config, and the comment naming the delay. Assertions unchanged: after the first session's Done the client calls Dial again (Discovery) within 5s, the first session's Cleanup (PADT) ran first, and the client holds SESSION_ID 0 in state discovery.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc2516_pppoe_client_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc2516_pppoe_client_test.go#L59) | unit/verify | revert, verified |

### [`RFC2516-x-10`](#rfc2516-x-10)

All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestPADINeverExceeds1484Octets asserts exactly 16 octets are left at the largest PADI and that one octet more is refused

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_padi_length_test.go#L19) | unit/verify | revert, verified |

### [`RFC2516-x-11`](#rfc2516-x-11)

A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2516-x-11, so there is no linked test.

### [`RFC2516-x-12`](#rfc2516-x-12)

This TAG (typically with a zero-length data section) indicates that for one reason or another, the requested Service-Name request could not be honored. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains why the request was denied. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516ErrorTagsCarryNoData moved. single-polarity; TestRFC2516ErrorTagsCarryNoData drives handlePADR and asserts the Service-Name-Error TAG_LENGTH is 0

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L356) | unit/verify | revert, verified |

### [`RFC2516-x-13`](#rfc2516-x-13)

It MAY be included in PADS packets. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains the nature of the error. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/pppoe/rfc2516_session_id_test.go::TestRFC2516ErrorTagsCarryNoData moved. single-polarity; TestRFC2516ErrorTagsCarryNoData drives a resource refusal and asserts the AC-System-Error TAG_LENGTH is 0

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/rfc2516_session_id_test.go#L357) | unit/verify | revert, verified |

### [`RFC2516-x-14`](#rfc2516-x-14)

It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2516-x-14, so there is no linked test.

### [`RFC2516-x-15`](#rfc2516-x-15)

If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2516-x-15, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc2516.txt |
| Source fingerprint | 260dccfe0a8a3e8c |
| Record | rfc/extraction/rfc2516.json |
| Mapped sentences | 35 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 5 | reviewed | not stated |
| `5` | not stated | 1 | reviewed | not stated |
| `5.1` | not stated | 3 | reviewed | not stated |
| `5.2` | not stated | 3 | reviewed | not stated |
| `5.3` | not stated | 2 | reviewed | not stated |
| `5.4` | not stated | 3 | reviewed | not stated |
| `5.5` | not stated | 2 | reviewed | not stated |
| `6` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 5 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 10 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The RFC 2119 conventions boilerplate of section 2. It tells the reader how to read the keywords and states no obligation of its own. | The keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL, when they appear in this document, are to be interpreted as described in [2]. |
| `5.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the Service-Name-Error reply rule the previous sentence states. RFC2516-5.4-2 already carries it: the Access Concentrator 'MUST reply with a PADS carrying a Service-Name-Error tag, and MUST set SESSION_ID to 0x0000 in that reply'. | In this case the SESSION_ID MUST be set to 0x0000. |
| `7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same 1492 ceiling the previous sentence sets on the MRU option, restated as the PPP MTU with its arithmetic: 1500 octets of Ethernet payload less the 6-octet PPPoE header and the 2-octet PPP Protocol ID. | Since Ethernet has a maximum payload size of 1500 octets, the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MTU MUST NOT be greater than 1492. |

## Superseded

No document obsoletes RFC 2516, so its obligations are stated where they were written.
