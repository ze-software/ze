# RFC 7534 - AS112 Nameserver Operations

AS112 operations. Every requirement listed for RFC 7534, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 2 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 6 | 6 of 7 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 3 | of 2 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 16 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | AS112 operations |
| Automatic checks | Included |
| Requirements | 16 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tests linked to requirements | 7 |
| Recorded audit verdicts | 3 |
| Fault test records | 6 |
| Summary | `rfc/short/rfc7534.md` |
| Requirement table | `rfc/requirements/rfc7534.md` |
| RFC text | `rfc/full/rfc7534.txt` |

## Automatic checks

Included in automatic checks: AS112 Nameserver Operations: two MUSTs over ze's authoritative AS112 DNS server (internal/plugins/as112, answerQuestions/zones.go). RFC7534-3.5-1 (answer authoritatively for each delegated zone) both polarities: a query at the apex of a Direct-Delegation reverse zone is answered from that zone (TestZoneAnswer_ReverseZoneNoData) while a name outside every served zone draws REFUSED with AA clear, so the node makes no authority claim over it (TestZoneAnswer_OutOfZoneRefused). RFC7534-3.5-2 (no records beyond SOA and NS) both polarities: a PTR query at the apex yields NODATA (TestZoneAnswer_ReverseZoneNoData) while the zone does contain its SOA + the two blackhole NS records (TestSOA_RFCMandatedParameters). RFC7534-3.5-3 (RFC 1918 records not hosted) is a SHOULD NOT, since the RFC states it in a lowercase zone-file comment and has no RFC 2119 key-words section, and is {single-polarity: positive}: the zones hold their records at the apex and nothing below it, so an RFC 1918 reverse PTR yields NXDOMAIN with no PTR in the reply and no such record is ever hosted (TestZoneAnswer_ResponseCodeByNamePosition). Ledger row unchanged (no gap).

## Published support status

**Area:** AS112 operations

**Test verification:** 2 of 2 MUST proven

**Evidence:** unit 2

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC7534-3.5-1`](#rfc7534-3.5-1), [`RFC7534-3.5-2`](#rfc7534-3.5-2)

## Figures

- **MUST bar**: Ze verified 2
- **Overview bar**: Ze MUST verified 2, Tests awaiting verification 1, Not assessed 13

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze verified | 2 |
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
| SHOULD / MAY rows | 14 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 1 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 13 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7534.json](../../../data/rfc/rfc7534.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7534-3.5-1` | Each nameserver functions as a single node in an AS112 anycast cloud [RFC4786] and is configured to answer authoritatively for a particular set of nominated zones. (§1) | MUST | 1 - Introduction | **positive:** `unit/verify` [`TestRFC7534EveryDirectDelegationZoneAuthoritativeAndBare`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc7534_direct_delegation_test.go#L28). **positive:** `unit/verify` [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L85). **negative:** `unit/verify` [`TestZoneAnswer_OutOfZoneRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L169) |
| `RFC7534-3.5-2` | There should be no other resource records included in this zone. (§3.5, db.dd-empty) | MUST | 3.5 - DNS Software | **positive:** `unit/verify` [`TestRFC7534EveryDirectDelegationZoneAuthoritativeAndBare`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc7534_direct_delegation_test.go#L35). **positive:** `unit/verify` [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L88). **negative:** `unit/verify` [`TestSOA_RFCMandatedParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L293) |
| `RFC7534-3.3-1` | The chosen platform should include either support for cloned loopback interfaces or the capability to bind multiple addresses to a single loopback interface. (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-2` | A host that is configured to act as an AS112 anycast node should be dedicated to that purpose and should not be used to simultaneously provide other services. (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-3` | The order in which interfaces are configured and software components started should be arranged such that routing software startup follows DNS software startup, and DNS software startup follows loopback interface configuration. (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-4` | Wrapper scripts or other arrangements should be employed to ensure that the anycast service prefix for AS112 is not advertised while either the anycast addresses are not configured or the DNS software is not running. (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-1` | The configuration above includes two restrictions on what the AS112 should advertise to its BGP neighbours: a prefix filter that permits only the service prefixes, and an AS_PATH filter that matches only locally originated routes. (§3.4) | SHOULD | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-2` | The configuration above includes two restrictions on what the AS112 should advertise to its BGP neighbours: a prefix filter that permits only the service prefixes, and an AS_PATH filter that matches only locally originated routes. (§3.4) | SHOULD | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.5-4` | Note that the nameserver is configured to act as an authoritative-only server (i.e., recursion is disabled). (§3.5) | SHOULD | 3.5 - DNS Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.5-5` | Note that the responses to the queries "HOSTNAME.AS112.NET IN TXT" and "HOSTNAME.AS112.ARPA IN TXT" should fit within a 512-octet DNS/ UDP datagram: i.e., it should be available over UDP transport without requiring EDNS0 support by the client. (§3.5) | SHOULD | 3.5 - DNS Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.5-3` | Records that relate to RFC 1918-numbered resources within the ; site hosting this AS112 node should not be hosted on this ; nameserver. (§3.5) | SHOULD NOT | 3.5 - DNS Software | **positive:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L247). **negative:** no negative test. **{single-polarity}:** ze's AS112 nameserver serves only empty SOA/NS Direct-Delegation zones (internal/plugins/as112/zones.go), which hold their records at the apex and nothing below it, so every RFC 1918 reverse name draws a name error and no RFC 1918 record is ever hosted -- there is no "RFC 1918 record is hosted" case to assert as a negative. The positive (an RFC 1918 reverse PTR yields NXDOMAIN, with no PTR anywhere in the reply) is proven in TestZoneAnswer_ResponseCodeByNamePosition |
| `RFC7534-4.1-1` | AS112 nodes should be monitored to ensure that they are functioning correctly, just as with any other production service. (§4.1) | SHOULD | 4.1 - Monitoring | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-4.2-1` | An AS112 node that needs to go off-line (e.g., for planned maintenance or as part of the diagnosis of some problem) should stop advertising the AS112 service prefixes to its BGP peers. (§4.2) | SHOULD | 4.2 - Downtime | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-4.3-1` | Use of the AS112 node should be measured in order to track long-term trends, identify anomalous conditions, and ensure that the configuration of the AS112 node is sufficient to handle the query load. (§4.3) | SHOULD | 4.3 - Statistics and Measurement | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.2-1` | It is good operational practice to notify the community of users that may fall within the reach of a new AS112 node before it is installed. At an Internet Exchange, local mailing lists usually exist to facilitate such announcements. For nodes that are intended to be globally reachable, coordination with other AS112 operators is highly recommended. (§3.2) | SHOULD | 3.2 - Topological Location | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-3` | For clarity, an IPv4-only AS112 node need not configure any of the IPv6 elements that follow; similarly, an IPv6-only AS112 node need not configure any of the IPv4 elements. (§3.4) | MAY | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7534 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7534-3.5-1`](#rfc7534-3.5-1)

Each nameserver functions as a single node in an AS112 anycast cloud [RFC4786] and is configured to answer authoritatively for a particular set of nominated zones. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Positive now covers every served Direct-Delegation zone (>=20 probed) through the real handler: SOA at the apex is AA, NOERROR, the zone's own SOA. Negative TestZoneAnswer_OutOfZoneRefused: a name outside every served zone draws REFUSED with AA clear. Records: answerQuestions revert, positive (author) and negative (judge) observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestZoneAnswer_OutOfZoneRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestRFC7534EveryDirectDelegationZoneAuthoritativeAndBare`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc7534_direct_delegation_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L85) | unit/verify | revert, verified |

### [`RFC7534-3.5-2`](#rfc7534-3.5-2)

There should be no other resource records included in this zone. (§3.5, db.dd-empty)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Positive: at every Direct-Delegation apex, A/AAAA/TXT/PTR/MX/CNAME/ANY return nothing but SOA/NS in any section, closing the PTR-only finding. Negative TestSOA_RFCMandatedParameters bounds the rule (the zone does carry SOA and NS) rather than violating it; the prohibition itself is proven by the positive across all zones and types. Records: answerQuestions (+, author) and buildSOA (-, judge) reverts observed red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSOA_RFCMandatedParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestRFC7534EveryDirectDelegationZoneAuthoritativeAndBare`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc7534_direct_delegation_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L88) | unit/verify | revert, verified |

### [`RFC7534-3.5-3`](#rfc7534-3.5-3)

Records that relate to RFC 1918-numbered resources within the ; site hosting this AS112 node should not be hosted on this ; nameserver. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (marker on the row). Sentence: records for the site's RFC 1918-numbered resources are not hosted on the nameserver. Forbidden behaviour: a PTR (or any record) for an RFC 1918 reverse name below a served zone is answered. TestZoneAnswer_ResponseCodeByNamePosition row 1.0.10.in-addr.arpa. PTR asserts Rcode NXDOMAIN (reply.Rcode != tc.rcode) and the loop over Answer and Extra fails on any PTR record, both red if such a record were hosted. answerQuestions has no per-zone branch below the apex (every non-apex name takes the missing path), so the one 10/8 probe covers the uniform path for all RFC 1918 zones.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L247) | unit/verify | not verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc7534 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc7534.txt |
| Source fingerprint | e49b3f8cd69c5bc3 |
| Record | rfc/extraction/rfc7534.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | reviewed | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes two sites here, both excluded below: the Abstract's last paragraph and the IETF Trust Legal Provisions boilerplate. The Abstract restates section 1 and states that the document obsoletes RFC 6304. Nothing before section 1 binds a nameserver or its operator. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative throughout: private-use addresses are not globally unique, devices originate reverse lookups for them, those queries are answered by nobody usefully, and the AS112 project is a distributed sink for them. Its one guidance sentence, 'it is good practice for site administrators to ensure that such queries are answered locally [RFC6303]', binds the site administrator whose resolver leaks the query, not the AS112 node; rfc/short/rfc7534.md carries it under Security Considerations and RFC 6303 owns it. The last paragraph states the project's operating shape, which the AS112 plugin implements: a nameserver configured to answer authoritatively for a nominated zone set, one node in an anycast cloud. |
| `2` | AS112 DNS Service, the parent heading | 0 | reviewed | AS112 DNS Service, the parent heading. No text of its own. |
| `2.1` | Approach, the parent heading of 2.1.1 and 2.1.2 | 0 | reviewed | Approach, the parent heading of 2.1.1 and 2.1.2. No text of its own. |
| `2.1.1` | Direct Delegation | 0 | reviewed | Direct Delegation. Indicative: zones whose traffic should be sunk are delegated directly to AS112 nameservers, and each node is manually configured to answer for them. The delegation half binds the parent zone's operator and IANA, not the node. The node half is the same obligation as RFC7534-3.5-1, declared unsourced on 3.5. |
| `2.1.2` | DNAME Redirection | 0 | reviewed | DNAME Redirection. Reports that RFC 7535 defines a different approach, in which queries are redirected to an empty zone also hosted on AS112 servers using DNAME [RFC6672], and that this document introduces the capability. The obligations of that approach belong to RFC 7535, which has its own summary and its own sign-off; nothing here directs a node beyond serving EMPTY.AS112.ARPA, which 2.2 lists and RFC7534-3.5-1 covers. |
| `2.2` | Zones | 0 | reviewed | Zones. The zone list an AS112 nameserver answers authoritatively for: 10.IN-ADDR.ARPA, the sixteen 172.16/12 zones, 168.192.IN-ADDR.ARPA, 254.169.IN-ADDR.ARPA, EMPTY.AS112.ARPA, and the two identification zones HOSTNAME.AS112.NET and HOSTNAME.AS112.ARPA. It is a list of names, not a directive; the obligation to answer authoritatively for each of them is RFC7534-3.5-1, declared unsourced on 3.5. The section closes by pointing at 3.5 for the recommended contents. Ze's table is buildServedZones in internal/plugins/as112/zones.go, twenty-two zones with the same membership. |
| `2.3` | Nameservers | 0 | reviewed | Nameservers. States which nameserver names and anycast addresses the zones are delegated to (BLACKHOLE-1 and BLACKHOLE-2.IANA.ORG, PRISONER.IANA.ORG as the SOA MNAME, BLACKHOLE.AS112.ARPA for DNAME redirection), and which prefixes cover them. The delegation binds IANA and the parent zone operators. The addresses a node configures are the subject of 3.3 and 3.4 and of RFC7534-3.4-3; the names a node puts in its own SOA and NS records are the subject of 3.5. Nothing is directed at a speaker here that is not directed again there. Ze's constants are anycastV4DirectDelegationAddr and its three siblings in internal/plugins/as112/register.go. |
| `3` | Installation of a New Node, the parent heading | 0 | reviewed | Installation of a New Node, the parent heading. No text of its own. |
| `3.1` | Useful Background Knowledge | 0 | reviewed | Useful Background Knowledge. Says installation is straightforward and that experience in BGP, DNS authoritative server operations and anycast distribution may prove useful. Advice to a person, directing no software behavior. |
| `3.2` | Topological Location | 0 | reviewed | Topological Location. Says a node may be located anywhere, that an Internet Exchange Point is often advantageous, and that a node may advertise its service prefix for local or for global use. Its one guidance sentence, 'It is good operational practice to notify the community of users that may fall within the reach of a new AS112 node before it is installed', with 'coordination with other AS112 operators is highly recommended' for globally reachable nodes, binds the operator installing the node and is repeated in section 5. rfc/short/rfc7534.md declares it as RFC7534-3.2-1 at [SHOULD] and cites both sections; it is declared unsourced here because the modal is the lowercase 'recommended' inside 'is highly recommended', which the site scan does not attribute to this section. |
| `3.3` | Operating System and Host Considerations | 0 | reviewed | Operating System and Host Considerations. Four lowercase-'should' sentences, all four declared unsourced here because the site scan attributes no site to this section: the chosen platform 'should include either support for cloned loopback interfaces or the capability to bind multiple addresses to a single loopback interface' (RFC7534-3.3-1); a host acting as an AS112 node 'should be dedicated to that purpose and should not be used to simultaneously provide other services' (RFC7534-3.3-2); startup order 'should be arranged such that routing software startup follows DNS software startup, and DNS software startup follows loopback interface configuration' (RFC7534-3.3-3); and 'Wrapper scripts or other arrangements should be employed to ensure that the anycast service prefix for AS112 is not advertised while either the anycast addresses are not configured or the DNS software is not running' (RFC7534-3.3-4). The first three bind the operator choosing and building the host. The fourth lands on ze, which is both the DNS server and the BGP speaker: as112Producer.onServingChanged and withdraw (internal/plugins/as112/redistribute.go) hold the covering prefixes back until the listeners are serving and withdraw them when they stop, so the wrapper the RFC asks an operator to write is inside the product. |
| `3.4` | Routing Software | 1 | reviewed | Routing Software. States that each AS112 node is a BGP speaker announcing 192.175.48.0/24 and 2620:4f:8000::/48 with origin AS 112, that the examples are based on the Quagga Routing Suite, and that an IPv4-only node need not configure the IPv6 elements and an IPv6-only node need not configure the IPv4 ones (RFC7534-3.4-3, [MAY]). Most of the section is an annotated bgpd.conf, and the guidance it carries is stated in the paragraph after it: 'two restrictions on what the AS112 should advertise to its BGP neighbours: a prefix filter that permits only the service prefixes, and an AS_PATH filter that matches only locally originated routes', which rfc/short/rfc7534.md declares as RFC7534-3.4-1 and RFC7534-3.4-2. All three ids are declared unsourced here: the one site the scan attributes to this section is the zebra.conf sentence, which is about Quagga and is excluded below. Ze's covering prefixes and origin AS are coveringPrefixesFor and as112DefaultASN (internal/plugins/as112/redistribute.go, config.go). |
| `3.5` | DNS Software | 0 | reviewed | DNS Software. The section that carries every MUST-level id this summary declares, and not one of them is stated with a modal the site scan can see, so all five ids read from here are declared unsourced. 'AS112 nodes operate as fully functional and standards-compliant DNS authoritative servers [RFC1034]' is RFC7534-3.5-1, with the named.conf note 'the nameserver is configured to act as an authoritative-only server (i.e., recursion is disabled)' as RFC7534-3.5-4. The zone content rules are comments inside db.dd-empty and db.dr-empty: 'There should be no other resource records included in this zone' is RFC7534-3.5-2, and 'Records that relate to RFC 1918-numbered resources within the site hosting this AS112 node should not be hosted on this nameserver' is RFC7534-3.5-3. 'the responses to the queries "HOSTNAME.AS112.NET IN TXT" and "HOSTNAME.AS112.ARPA IN TXT" should fit within a 512-octet DNS/UDP datagram: i.e., it should be available over UDP transport without requiring EDNS0 support by the client' is RFC7534-3.5-5. The SOA field values, the NS targets and the $TTL of 1W are sample zone data rather than sentences, and rfc/short/rfc7534.md carries them in its DNS Software Requirements prose; ze's constants are soaRefresh, soaRetry, soaExpire, soaMinTTL and zoneTTL in internal/plugins/as112/zones.go, and the answer synthesis is zoneAnswer in the same file. Two further sentences state no obligation ze is left owing. 'The optional LOC record [RFC1876] included in each zone apex provides information about the geospatial location of the node' is offered as optional and no id declares it; ze synthesizes SOA, NS and TXT at the apex and no LOC. 'Where software implementations support it, operational data should also be carried using NSID [RFC5001]' is conditioned on the DNS software supporting NSID, and ze's DNS harness (internal/core/dnsserver) reads the EDNS0 OPT record for client-subnet (RFC 7871) and implements no NSID option, so the condition does not hold and the sentence binds nothing here. |
| `3.6` | Testing a Newly Installed Node | 0 | reviewed | Testing a Newly Installed Node. A worked 'dig' invocation against hostname.as112.net TXT, how to read a response that names a different node, and the note that a proper test set queries from many places over both UDP and TCP against all three anycast addresses. Instructions to the person commissioning the node, directing no behavior of the node itself. Ze's equivalent is the operator-facing as112 health command, handleAS112Health in internal/plugins/as112/health.go. |
| `4` | Operations, the parent heading | 0 | reviewed | Operations, the parent heading. No text of its own. |
| `4.1` | Monitoring | 0 | reviewed | Monitoring. One lowercase-'should' sentence, declared unsourced here: 'AS112 nodes should be monitored to ensure that they are functioning correctly, just as with any other production service.' It binds the node's operator, and the rest of the section says why: a node that answers wrongly causes failures and timeouts in dependent systems that are hard to diagnose. |
| `4.2` | Downtime | 0 | reviewed | Downtime. One lowercase-'should' sentence, declared unsourced here: 'An AS112 node that needs to go off-line (e.g., for planned maintenance or as part of the diagnosis of some problem) should stop advertising the AS112 service prefixes to its BGP peers', by shutting the routing software down or by withdrawing the route. The second paragraph gives the reason: an announced prefix with a dead nameserver blackholes the query traffic. The obligation binds the operator taking the node down, and ze gives them the mechanism: as112Producer.withdraw (internal/plugins/as112/redistribute.go) withdraws the covering prefixes when the listeners stop serving. |
| `4.3` | Statistics and Measurement | 0 | reviewed | Statistics and Measurement. One lowercase-'should' sentence, declared unsourced here: 'Use of the AS112 node should be measured in order to track long-term trends, identify anomalous conditions, and ensure that the configuration of the AS112 node is sufficient to handle the query load.' It binds the operator. The rest names free monitoring tools (bindgraph, dnstop, DSC) and suggests joining DNS-OARC collection events, which is a recommendation about a community activity and directs no software. Ze publishes the counters an operator measures with: buildMetrics in internal/plugins/as112/metrics.go. |
| `5` | Communications | 0 | reviewed | Communications. Repeats 3.2's notify-the-community guidance, names the as112-ops@lists.dns-oarc.net mailing list and www.as112.net, says information about a node should also be published in the DNS through the HOSTNAME.AS112.* zones (which is 3.5's TXT content, RFC7534-3.5-5), and says operators should be aware of RFC 6305 and direct site administrators appropriately. Every obligation binds the node's operator and is already declared: RFC7534-3.2-1 cites this section beside 3.2, and it is declared unsourced on 3.2 rather than twice. |
| `6` | On the Future of AS112 Nodes | 0 | reviewed | On the Future of AS112 Nodes. Indicative and predictive: recursive-nameserver operators are recommended by RFC 6303 to answer these zones locally, no trend toward that compliance is observable, vendors are expected to ship such defaults, the query load will not fall to zero, and the community might one day recommend retiring Direct Delegation service -- which the section says explicitly this document does not recommend. The one recommendation named belongs to RFC 6303 and binds a recursive resolver, not an AS112 node. |
| `7` | IANA Considerations, the parent heading | 0 | skipped (iana) | IANA Considerations, the parent heading. |
| `7.1` | IANA Considerations, General | 0 | skipped (iana) | IANA Considerations, General. Records who assigned the AS number and the four prefixes (ARIN for 112, 192.175.48.0/24 and 2620:4f:8000::/48; IANA for 192.31.196.0/24 and 2001:4:112::/48) and that the anycast infrastructure itself is not an IANA function. Binds IANA and the registries. |
| `7.2` | IANA Actions, the parent heading | 0 | skipped (iana) | IANA Actions, the parent heading. |
| `7.2.1` | not stated | 0 | skipped (iana) | The AAAA records IANA has added for PRISONER, BLACKHOLE-1 and BLACKHOLE-2.IANA.ORG. An IANA action on the IANA.ORG zone. |
| `7.2.2` | not stated | 0 | skipped (iana) | The registration of AS 112 in the Special-Purpose AS Numbers registry [RFC7249]. An IANA action. |
| `7.2.3` | not stated | 0 | skipped (iana) | The registration of 192.175.48.0/24 in the IANA IPv4 Special-Purpose Address Registry [RFC6890]. An IANA action. |
| `7.2.4` | not stated | 0 | skipped (iana) | The registration of 2620:4f:8000::/48 in the IANA IPv6 Special-Purpose Address Registry [RFC6890]. An IANA action. |
| `8` | Security Considerations | 0 | reviewed | Security Considerations. Five paragraphs, none of which directs the node's DNS or BGP behavior. Hosts should never normally send these queries and should answer them locally, which binds the querying site. Responses are unexpected and can trigger intrusion detection, so AS112 operators should expect to be contacted, with RFC 6305 as the advice to give. A loosely coordinated deployment makes a compromised node hard to detect, so operators should protect their nodes as they would any production nameserver, with RFC 2870 named as instructive: that binds the operator. The last paragraph states as fact that AS112 zones are not DNSSEC-signed, because the key material would have to be effectively public; it is a statement of what the service does not do, not an obligation. rfc/short/rfc7534.md carries all five under Security Considerations and declares no id for them. |
| `9` | References, the parent heading | 0 | skipped (references) | References, the parent heading. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 1034, RFC 1918, RFC 2870, RFC 4033, RFC 4271, RFC 4786 and RFC 7535. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: RFC 1876, RFC 5001, RFC 5855, RFC 6303, RFC 6304, RFC 6305, RFC 6672, RFC 6890 and RFC 7249. |
| `A` | Appendix A, A Brief History of AS112 | 0 | skipped (appendix-non-normative) | Appendix A, A Brief History of AS112. Narrative: RFC 1918 use spread from 1996, offloading IN-ADDR.ARPA from the root servers was proposed by Bill Manning and John Brown, ARIN provided the prefix and AS 112, the first nodes were deployed in 2002, and IN-ADDR.ARPA was redelegated in 2011. It directs nobody. |
| `B` | not stated | 0 | skipped (appendix-non-normative) | Appendix B, Changes since RFC 6304: IPv6 transport, DNAME-based delegation of additional zones, the consequent configuration guidance, clarified information-leak text, and the direction to IANA to register the prefixes. A change list over the obsoleted document. The derivation attributes the trailing Acknowledgements and Authors' Addresses to this section, and neither binds anybody either. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Abstract's closing paragraph, non-normative by RFC convention. Its lowercase 'required' is inside 'the steps required to install a new AS112 node', which names what the document is about rather than obliging anybody to do anything. The installation guidance itself is section 3, and every id read from it is declared unsourced on the section that states it. | This document describes the steps required to install a new AS112 node and offers advice relating to such a node's operation. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no DNS or BGP behavior. The document's sample bgpd.conf, named.conf and zone files are Quagga and BIND9 configuration, not code components ze reuses: internal/plugins/as112 synthesizes the same zone contents in Go. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `3.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another system, and the extractor cannot see which: 'zebra.conf' is a file of the Quagga Routing Suite, which the same section names as the basis of its examples while saying 'other software packages exist that also provide suitable BGP support for AS112 nodes'. The sentence states that Quagga needs a zebra daemon to reach the kernel FIB, so its lowercase 'required' is a fact about that package's architecture and not an obligation on an AS112 node. Ze is a single daemon with its own FIB path and no zebra; the two restrictions this section does place on an AS112 node's BGP advertisement are RFC7534-3.4-1 and RFC7534-3.4-2, declared unsourced on 3.4. | The "zebra.conf" file is required to provide integration between protocol daemons (bgpd, in this case) and the kernel. |

## Superseded

No document obsoletes RFC 7534, so its obligations are stated where they were written.
