# RFC 7948 - Internet Exchange BGP Route Server Operations

No row in the public ledger. Every requirement this repository extracted from RFC 7948, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 0 | of 11 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (non-normative), so every count below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 0 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

No card above is a part of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | No row in the public ledger |
| Enrolment | Not enrolled (non-normative) |
| Requirements | 11 |
| Gated MUST-level | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7948.md` |
| Requirement shard | `rfc/requirements/rfc7948.md` |
| RFC text | `rfc/full/rfc7948.txt` |

## Enrolment

Not enrolled (non-normative, the document imposes no MUST-level obligation on an implementation, so there is nothing to gate): Internet Exchange BGP Route Server Operations, IETF category Informational. A capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over rfc/full/rfc7948.txt hits zero keywords outside the Section 1.1 key-words paragraph. The capitalised keywords it does use are RECOMMENDED (five times, Sections 4.3, 4.4 and 4.5) and SHOULD (six times, Sections 4.6.1, 4.6.3, 4.8, 4.9 and 5), so the text states operational guidance at advisory level only. The lower-case "must" sentences the prose inventory finds describe consequences (the Decision Process "must be applied" to every Loc-RIB, routes "must be transmitted individually") or the operator's dependence on client data, and none binds an implementation. The summary captures 11 advisory requirements and gates none. The walk is recorded in rfc/extraction/rfc7948.json at register prose, all 8 sites excluded as not-a-requirement.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7948.

## Coverage

RFC 7948 declares no MUST-level requirement, so the gate counts nothing here.

## Figures

- **MUST bar**: no rows
- **Overview bar**: SHOULD not decided 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 11 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc7948.json](../../../data/rfc/rfc7948.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7948-4.3-1` | It is therefore RECOMMENDED when deploying route servers that, due to the potential for collateral damage caused by BGP route leakage, route server operators deploy prefix leakage mitigation measures in order to prevent unintentional prefix announcements or else limit the scale of any such leak. (§4.3) | RECOMMENDED | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.3-2` | If a route server operator implements per-client inbound prefix filtering, then it is RECOMMENDED that the operator also builds in mechanisms to automatically compare the Adj-RIB-In received from each client with the inbound prefix lists configured for those clients. (§4.3) | RECOMMENDED | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.4-1` | As the purpose of an IXP route server implementation is to provide a reliable reachability brokerage service, it is RECOMMENDED that exchange operators who implement route server systems provision multiple route servers on each shared Layer 2 domain. (§4.4) | RECOMMENDED | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.4-2` | it is RECOMMENDED that where an operator provisions more than a single server on the same shared Layer 2 domain, each route server implementation be configured equivalently and in such a manner that the path reachability information from each system is identical. (§4.4) | RECOMMENDED | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.5-1` | It is therefore RECOMMENDED that route server clients disable the AS_PATH consistency check towards the route server. (§4.5) | RECOMMENDED | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.6.1-1` | Bearing this in mind, the route server operator SHOULD take care to ensure that the predefined BGP Communities values mechanism used on their route server is compatible with 4-octet AS numbers [RFC6793]. (§4.6.1) | SHOULD | 4.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.6.3-1` | A database of this form SHOULD allow a route server client operator to update their routing policy and provide a mechanism for allowing the client to specify whether they wish to exchange all their prefixes with any other route server client. (§4.6.3) | SHOULD | 4.6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.8-1` | Because route server clients cannot easily implement next-hop policy checks against route server BGP sessions, route server operators SHOULD check that the BGP NEXT_HOP attribute for BGP routes received from a route server client matches the interface address of the client. (§4.8) | SHOULD | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.8-2` | If the route server receives a BGP route where these addresses are different and where the announcing route server client is in a different AS to the route server client that uses the next- hop address, the BGP route SHOULD be dropped. (§4.8) | SHOULD | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-4.9-1` | BGP route servers SHOULD be configured and operated in compliance with [RFC7454] with the exception of Section 11, "BGP Community Scrubbing", which may not necessarily apply on a route server, depending on the route server operator policy. (§4.9) | SHOULD | 4.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7948-5-1` | Route server installations SHOULD be secured against BGP NEXT_HOP hijacking, as described in Section 4.8. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7948 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 7948 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | prose |
| Source | rfc/full/rfc7948.txt |
| Source fingerprint | c3435c9387bb11ec |
| Record | rfc/extraction/rfc7948.json |
| Mapped sentences | 0 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, abstract, status, copyright and contents | 1 | skipped (front-matter) | Title page, abstract, status, copyright and contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 3 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.1.1` | not stated | 1 | walked | not stated |
| `4.2.1.2` | not stated | 0 | walked | not stated |
| `4.2.1.3` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.6.1` | not stated | 0 | walked | not stated |
| `4.6.2` | not stated | 0 | walked | not stated |
| `4.6.3` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 0 | walked | not stated |
| `4.8` | not stated | 0 | walked | not stated |
| `4.9` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | References | 0 | skipped (references) | References |
| `6.1` | Normative references | 0 | skipped (references) | Normative references |
| `6.2` | Informative references | 0 | skipped (references) | Informative references |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Copyright boilerplate: the Trust Legal Provisions licence text for code components, not a protocol obligation | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: 'scales quadratically' states a cost of bilateral peering, and the lower-case modal binds nobody | As many exchanges have large numbers of participating networks, the amount of administrative and operation overhead required to implement an open interconnection scales quadratically. |
| `4.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: states that the RFC 4271 Decision Process runs per Loc-RIB to explain O(P * N) scaling; the obligation is RFC 4271's | As the BGP Decision Process [RFC4271] must be applied to all Loc-RIBs deployed on the route server, both CPU and memory requirements on the host computer scale approximately according to O(P * N), where P is the total number of unique paths received by the route server, and N is the number of route server clients that require a unique Loc-RIB. |
| `4.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: 'may derive benefit' is a lower-case observation about deployment cost, no keyword use | As this is a super-linear scaling relationship, large route servers may derive benefit from deploying per-client Loc-RIBs only where they are required. |
| `4.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: 'so must be transmitted individually' states a worst-case assumption in a scaling formula | If we assume that in the worst case, each prefix is associated with a different set of BGP path attributes, so must be transmitted individually, the network bandwidth scaling function can be rewritten as O((P_avg * N) * N) or O(N^2). |
| `4.2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: the sentence names three problem areas the scaling analysis indicates | Scaling analysis indicates problems in three key areas: route processor CPU overhead associated with BGP Decision Process calculations, the memory requirements for handling many different BGP path entries, and the network traffic bandwidth required to distribute these BGP routes from the route server to each route server client. |
| `4.2.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: 'must depend on each client sharing their interconnection policies' states an operator dependency, not an obligation | As this approach requires prior knowledge of interconnection relationships, the route server operator must depend on each client sharing their interconnection policies either in an internal provisioning database controlled by the operator or in an external data store such as an Internet Routing Registry Database. |
| `4.2.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: says recursive NEXT_HOP resolution 'is generally not required and can be replaced' by an IXP-LAN check; a lower-case observation with no keyword | As route servers are usually deployed at IXPs where all connected routers are on the same Layer 2 broadcast domain, recursive resolution of the NEXT_HOP attribute is generally not required and can be replaced by a simple check to ensure that the NEXT_HOP value for each received BGP route is a network address on the IXP LAN's IP address range. |

## Superseded

No document obsoletes RFC 7948, so its obligations are stated where they were written.
